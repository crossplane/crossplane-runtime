/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package connection

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errConnectStore    = "cannot connect to secret store"
	errWriteStore      = "cannot write to secret store"
	errReadStore       = "cannot read from secret store"
	errDeleteFromStore = "cannot delete from secret store"
	errGetStoreConfig  = "cannot get store config"
	errSecretConflict  = "cannot establish control of existing connection secret"

	errFmtNotOwnedBy      = "existing secret is not owned by UID %q"
	errFmtRefusingUnowned = "refusing to modify unowned secret of type %q"
)

// StoreBuilderFn is a function that builds and returns a Store with a given
// store config.
type StoreBuilderFn func(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (Store, error)

// A DetailsManagerOption configures a DetailsManager.
type DetailsManagerOption func(*DetailsManager)

// WithStoreBuilder configures the StoreBuilder to use.
func WithStoreBuilder(sb StoreBuilderFn) DetailsManagerOption {
	return func(m *DetailsManager) {
		m.storeBuilder = sb
	}
}

// DetailsManager is a connection details manager that satisfies the required
// interfaces to work with connection details by managing interaction with
// different store implementations.
type DetailsManager struct {
	client       client.Client
	newConfig    func() StoreConfig
	storeBuilder StoreBuilderFn
}

// NewDetailsManager returns a new connection DetailsManager.
func NewDetailsManager(c client.Client, of schema.GroupVersionKind, o ...DetailsManagerOption) *DetailsManager {
	nc := func() StoreConfig {
		return resource.MustCreateObject(of, c.Scheme()).(StoreConfig)
	}

	// Panic early if we've been asked to reconcile a resource kind that has not
	// been registered with our controller manager's scheme.
	_ = nc()

	m := &DetailsManager{
		client:       c,
		newConfig:    nc,
		storeBuilder: RuntimeStoreBuilder,
	}

	for _, mo := range o {
		mo(m)
	}

	return m
}

// PublishConnection publishes the supplied ConnectionDetails to a secret on
// the configured connection Store.
func (m *DetailsManager) PublishConnection(ctx context.Context, so resource.ConnectionSecretOwner, conn managed.ConnectionDetails) (bool, error) {
	// This resource does not want to expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return false, nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return false, errors.Wrap(err, errConnectStore)
	}

	if p.Metadata == nil {
		p.Metadata = &v1.ConnectionSecretMetadata{}
	}
	p.Metadata.SetOwnerUID(so)

	changed, err := ss.WriteKeyValues(ctx, &store.Secret{
		ScopedName: store.ScopedName{
			Name:  p.Name,
			Scope: so.GetNamespace(),
		},
		Metadata: p.Metadata,
		Data:     store.KeyValues(conn),
	}, SecretMustBeOwnedBy(so))

	return changed, errors.Wrap(err, errWriteStore)
}

// UnpublishConnection deletes connection details secret to the configured
// connection Store.
func (m *DetailsManager) UnpublishConnection(ctx context.Context, so resource.ConnectionSecretOwner, conn managed.ConnectionDetails) error {
	// This resource didn't expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return errors.Wrap(err, errConnectStore)
	}

	err = ss.DeleteKeyValues(ctx, &store.Secret{
		ScopedName: store.ScopedName{
			Name:  p.Name,
			Scope: so.GetNamespace(),
		},
		Metadata: p.Metadata,
		Data:     store.KeyValues(conn),
	})
	return errors.Wrap(err, errDeleteFromStore)
}

// FetchConnection fetches connection details of a given ConnectionSecretOwner.
func (m *DetailsManager) FetchConnection(ctx context.Context, so resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	// This resource does not want to expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil, nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return nil, errors.Wrap(err, errConnectStore)
	}

	s := &store.Secret{}
	return managed.ConnectionDetails(s.Data), errors.Wrap(ss.ReadKeyValues(ctx, store.ScopedName{Name: p.Name, Scope: so.GetNamespace()}, s), errReadStore)
}

// PropagateConnection propagate connection details from one resource to another.
func (m *DetailsManager) PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (propagated bool, err error) {
	// Either from does not expose a connection secret, or to does not want one.
	if from.GetPublishConnectionDetailsTo() == nil || to.GetPublishConnectionDetailsTo() == nil {
		return false, nil
	}

	ssFrom, err := m.connectStore(ctx, from.GetPublishConnectionDetailsTo())
	if err != nil {
		return false, errors.Wrap(err, errConnectStore)
	}

	sFrom := &store.Secret{}
	if err = ssFrom.ReadKeyValues(ctx, store.ScopedName{
		Name:  from.GetPublishConnectionDetailsTo().Name,
		Scope: from.GetNamespace(),
	}, sFrom); err != nil {
		return false, errors.Wrap(err, errReadStore)
	}

	// Make sure 'from' is the controller of the connection secret it references
	// before we propagate it. This ensures a resource cannot use Crossplane to
	// circumvent RBAC by propagating a secret it does not own.
	if m := sFrom.Metadata; m == nil || m.GetOwnerUID() != string(from.GetUID()) {
		return false, errors.New(errSecretConflict)
	}

	ssTo, err := m.connectStore(ctx, to.GetPublishConnectionDetailsTo())
	if err != nil {
		return false, errors.Wrap(err, errConnectStore)
	}

	toMeta := to.GetPublishConnectionDetailsTo().Metadata
	if toMeta == nil {
		toMeta = &v1.ConnectionSecretMetadata{}
	}
	toMeta.SetOwnerUID(to)
	changed, err := ssTo.WriteKeyValues(ctx, &store.Secret{
		ScopedName: store.ScopedName{
			Name:  to.GetPublishConnectionDetailsTo().Name,
			Scope: to.GetNamespace(),
		},
		Metadata: toMeta,
		Data:     sFrom.Data,
	}, SecretMustBeOwnedBy(to))

	return changed, errors.Wrap(err, errWriteStore)
}

func (m *DetailsManager) connectStore(ctx context.Context, p *v1.PublishConnectionDetailsTo) (Store, error) {
	sc := m.newConfig()
	if err := m.client.Get(ctx, types.NamespacedName{Name: p.SecretStoreConfigRef.Name}, sc); err != nil {
		return nil, errors.Wrap(err, errGetStoreConfig)
	}

	return m.storeBuilder(ctx, m.client, sc.GetStoreConfig())
}

// SecretMustBeOwnedBy requires that the current object is a
// connection secret that is owned by an object with the supplied UID.
func SecretMustBeOwnedBy(so metav1.Object) store.WriteOption {
	return func(_ context.Context, current, desired *store.Secret) error {
		o := ""
		if current.Metadata != nil {
			o = current.Metadata.GetOwnerUID()
		}

		switch {
		case o == "" && current.Metadata != nil && current.Metadata.Type != nil && desired.Metadata != nil && current.Metadata.Type != desired.Metadata.Type:
			return errors.Errorf(errFmtRefusingUnowned, *current.Metadata.Type)
		case o == "":
			return nil
		case o != string(so.GetUID()):
			return errors.Errorf(errFmtNotOwnedBy, string(so.GetUID()))
		}

		return nil
	}
}
