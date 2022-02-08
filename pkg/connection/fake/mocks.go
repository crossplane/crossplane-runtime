package fake

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

type SecretStoreFns struct {
	ConnectionPublisherFns
}

// ConnectionPublisherFns is the pluggable struct to produce objects with ConnectionPublisher interface.
type ConnectionPublisherFns struct {
	PublishConnectionFn   func(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error
	UnpublishConnectionFn func(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error
}

// PublishConnection details for the supplied Managed resource.
func (fn ConnectionPublisherFns) PublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	return fn.PublishConnectionFn(ctx, mg, c)
}

// UnpublishConnection details for the supplied Managed resource.
func (fn ConnectionPublisherFns) UnpublishConnection(ctx context.Context, mg resource.Managed, c managed.ConnectionDetails) error {
	return fn.UnpublishConnectionFn(ctx, mg, c)
}
