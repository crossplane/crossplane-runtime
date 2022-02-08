package connection

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/connection/secret/kubernetes"

	"github.com/crossplane/crossplane-runtime/pkg/connection/secret"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Error strings.
const (
	errExtractKubernetesAuthCreds = "cannot extract kubernetes auth credentials"

	errFmtUnknownSecretStore = "unknown secret store type: %d"
)

type SecretStoreManager struct {
	client client.Client
	typer  runtime.ObjectTyper

	log logging.Logger
}

type SecretStoreManagerOption func(*SecretStoreManager)

func WithLogger(l logging.Logger) SecretStoreManagerOption {
	return func(m *SecretStoreManager) {
		m.log = l
	}
}

func NewSecretStoreManager(c client.Client, ot runtime.ObjectTyper, o ...SecretStoreManagerOption) *SecretStoreManager {
	m := &SecretStoreManager{
		client: c,
		typer:  ot,

		log: logging.NewNopLogger(),
	}

	for _, mo := range o {
		mo(m)
	}

	return m
}

func (m *SecretStoreManager) connectToStore(ctx context.Context, cfg v1.SecretStoreConfig) (secret.Store, error) {
	switch cfg.Type {
	case v1.SecretStoreKubernetes:
		return kubernetes.NewSecretStore(ctx, m.client, cfg.Kubernetes)
	case v1.SecretStoreVault:
		return nil, nil
	default:
		return nil, errors.Errorf(errFmtUnknownSecretStore, cfg.Type)
	}
}

func (m *SecretStoreManager) ValidateConfig(ctx context.Context) error {
	m.log.Info("validating config...")
	return nil
}

func (m *SecretStoreManager) PublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	panic("implement me")
}

func (m *SecretStoreManager) UnpublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	panic("implement me")
}
