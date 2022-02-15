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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
)

// Error strings.
const (
	errConnectStore    = "cannot connect to secret store"
	errWriteStore      = "cannot write to secret store"
	errDeleteFromStore = "cannot delete from secret store"
	errGetStoreConfig  = "cannot get store config"
)

// A StoreConfigKind contains the type metadata for a kind of StoreConfig
// resource.
type StoreConfigKind schema.GroupVersionKind

// StoreBuilderFn is a function that builds and returns a Store with a given
// store config.
type StoreBuilderFn func(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (Store, error)

// A ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithLogger specifies how the Manager should log messages.
func WithLogger(l logging.Logger) ManagerOption {
	return func(m *Manager) {
		m.log = l
	}
}

// WithStoreBuilder configures the StoreBuilder to use.
func WithStoreBuilder(sb StoreBuilderFn) ManagerOption {
	return func(m *Manager) {
		m.storeBuilder = sb
	}
}

// Manager is a connection details manager that satisfies the required
// interfaces to work with connection details by managing interaction with
// different store implementations.
type Manager struct {
	client         client.Client
	newStoreConfig func() StoreConfig
	storeBuilder   StoreBuilderFn

	log logging.Logger
}

// NewManager returns a new connection Manager.
func NewManager(c client.Client, of StoreConfigKind, o ...ManagerOption) *Manager {
	nsc := func() StoreConfig {
		return store.NewConfig(store.ConfigWithGroupVersionKind(schema.GroupVersionKind(of)))
	}

	m := &Manager{
		client:         c,
		newStoreConfig: nsc,
		storeBuilder:   RuntimeStoreBuilder,

		log: logging.NewNopLogger(),
	}

	for _, mo := range o {
		mo(m)
	}

	return m
}

// PublishConnection publishes the supplied ConnectionDetails to a secret on
// the configured connection Store.
func (m *Manager) PublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	return m.publishConnection(ctx, mg.(SecretOwner), store.KeyValues(c))
}

// UnpublishConnection deletes connection details secret from the configured
// connection Store.
func (m *Manager) UnpublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	return m.unpublishConnection(ctx, mg.(SecretOwner), store.KeyValues(c))
}

func (m *Manager) connectStore(ctx context.Context, p *v1.PublishConnectionDetailsTo) (Store, error) {
	sc := m.newStoreConfig()
	if err := unstructured.NewClient(m.client).
		Get(ctx, types.NamespacedName{Name: p.SecretStoreConfigRef.Name}, sc); err != nil {
		return nil, errors.Wrap(err, errGetStoreConfig)
	}

	return m.storeBuilder(ctx, m.client, sc.GetStoreConfig())
}

func (m *Manager) publishConnection(ctx context.Context, so SecretOwner, kv store.KeyValues) error {
	// This resource does not want to expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return errors.Wrap(err, errConnectStore)
	}

	return errors.Wrap(ss.WriteKeyValues(ctx, store.Secret{
		Name:     p.Name,
		Scope:    so.GetNamespace(),
		Metadata: p.Metadata.Raw,
	}, kv), errWriteStore)
}

func (m *Manager) unpublishConnection(ctx context.Context, so SecretOwner, kv store.KeyValues) error {
	// This resource didn't expose a connection secret.
	p := so.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return errors.Wrap(err, errConnectStore)
	}

	return errors.Wrap(ss.DeleteKeyValues(ctx, store.Secret{
		Name:     p.Name,
		Scope:    so.GetNamespace(),
		Metadata: p.Metadata.Raw,
	}, kv), errDeleteFromStore)
}
