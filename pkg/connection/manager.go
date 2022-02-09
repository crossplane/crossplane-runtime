package connection

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/secret/kubernetes"
	"github.com/crossplane/crossplane-runtime/pkg/connection/secret/store"
	"github.com/crossplane/crossplane-runtime/pkg/connection/secret/vault"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errConnectStore    = "cannot connect to secret store"
	errWriteStore      = "cannot write to secret store"
	errDeleteFromStore = "cannot delete from secret store"
	errGetStoreConfig  = "cannot get store config"

	errFmtUnknownSecretStore = "unknown secret store type: %q"
)

type StoreConfigProvider interface {
	GetStoreConfig() v1.SecretStoreConfig
}

type StoreConfig interface {
	resource.Object

	resource.Conditioned
	StoreConfigProvider
}

// A StoreConfigKind contains the type metadata for a kind of StoreConfig
// resource.
type StoreConfigKind schema.GroupVersionKind

type StoreBuilderFn func(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (store.Store, error)

type Manager struct {
	client         client.Client
	typer          runtime.ObjectTyper
	newStoreConfig func() StoreConfig
	storeBuilders  map[v1.SecretStoreType]StoreBuilderFn

	log logging.Logger
}

type ManagerOption func(*Manager)

func WithLogger(l logging.Logger) ManagerOption {
	return func(m *Manager) {
		m.log = l
	}
}

func NewManager(c client.Client, ot runtime.ObjectTyper, of StoreConfigKind, o ...ManagerOption) *Manager {
	nsc := func() StoreConfig {
		return store.NewConfig(store.ConfigWithGroupVersionKind(schema.GroupVersionKind(of)))
	}

	m := &Manager{
		client:         c,
		typer:          ot,
		newStoreConfig: nsc,
		storeBuilders: map[v1.SecretStoreType]StoreBuilderFn{
			v1.SecretStoreKubernetes: kubernetes.NewSecretStore,
			v1.SecretStoreVault:      vault.NewSecretStore,
		},

		log: logging.NewNopLogger(),
	}

	for _, mo := range o {
		mo(m)
	}

	return m
}

func (m *Manager) connectStore(ctx context.Context, p *v1.PublishConnectionDetailsTo) (store.Store, error) {
	sc := m.newStoreConfig()
	if err := m.client.Get(ctx, types.NamespacedName{Name: p.SecretStoreConfigRef.Name}, sc); err != nil {
		return nil, errors.Wrap(resource.IgnoreNotFound(err), errGetStoreConfig)
	}

	cfg := sc.GetStoreConfig()
	sb, ok := m.storeBuilders[cfg.Type]
	if !ok {
		return nil, errors.Errorf(errFmtUnknownSecretStore, cfg.Type)
	}

	return sb(ctx, m.client, cfg)
}

func (m *Manager) PublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	// This resource does not want to expose a connection secret.
	p := mg.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return errors.Wrap(err, errConnectStore)
	}

	return errors.Wrap(ss.WriteKeyValues(ctx, store.SecretInstance{
		Name:     p.Name,
		Scope:    mg.GetNamespace(),
		Owner:    meta.AsController(meta.TypedReferenceTo(mg, resource.MustGetKind(mg, m.typer))),
		Metadata: p.Metadata,
	}, store.KeyValues(c)), errWriteStore)
}

func (m *Manager) UnpublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	// This resource didn't expose a connection secret.
	p := mg.GetPublishConnectionDetailsTo()
	if p == nil {
		return nil
	}

	ss, err := m.connectStore(ctx, p)
	if err != nil {
		return errors.Wrap(err, errConnectStore)
	}

	return errors.Wrap(ss.DeleteKeyValues(ctx, store.SecretInstance{
		Name:     p.Name,
		Scope:    mg.GetNamespace(),
		Owner:    meta.AsController(meta.TypedReferenceTo(mg, resource.MustGetKind(mg, m.typer))),
		Metadata: p.Metadata,
	}, store.KeyValues(c)), errDeleteFromStore)
}
