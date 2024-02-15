/*
Copyright 2024 The Crossplane Authors.

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

// Package external contains utilities for observing, creating, updating, and
// deleting external resources. An external resource is a resource 'external to'
// Crossplane, for example a resource in the AWS API. Every Crossplane managed
// resource corresponds to an external resource.
package external

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// An Connector produces a new Client for an external system given the supplied
// managed resource.
type Connector interface {
	// Connect to the provider specified by the supplied managed resource and
	// produce an Client.
	Connect(ctx context.Context, mg resource.Managed) (Client, error)
}

// An Disconnector disconnects from an external system.
type Disconnector interface {
	// Disconnect from the provider and close the Client.
	Disconnect(ctx context.Context) error
}

// A NopDisconnector converts an Connector into an ConnectDisconnector with a
// no-op Disconnect method.
type NopDisconnector struct {
	c Connector
}

// Connect calls the underlying Connector's Connect method.
func (c *NopDisconnector) Connect(ctx context.Context, mg resource.Managed) (Client, error) {
	return c.c.Connect(ctx, mg)
}

// Disconnect does nothing. It never returns an error.
func (c *NopDisconnector) Disconnect(_ context.Context) error {
	return nil
}

// NewNopDisconnector converts an Connector into an ConnectDisconnector with a
// no-op Disconnect method.
func NewNopDisconnector(c Connector) ConnectDisconnector {
	return &NopDisconnector{c}
}

// An ConnectDisconnector produces a new Client given the supplied managed
// resource. It supports disconnecting from the client.
type ConnectDisconnector interface {
	Connector
	Disconnector
}

// An ConnectorFn is a function that satisfies the Connector interface.
type ConnectorFn func(ctx context.Context, mg resource.Managed) (Client, error)

// Connect to the external system specified by the supplied managed resource and
// produce n Client.
func (ec ConnectorFn) Connect(ctx context.Context, mg resource.Managed) (Client, error) {
	return ec(ctx, mg)
}

// An DisconnectorFn is a function that satisfies the Disonnector interface.
type DisconnectorFn func(ctx context.Context) error

// Disconnect from provider and close the Client.
func (ed DisconnectorFn) Disconnect(ctx context.Context) error {
	return ed(ctx)
}

// ConnectDisconnectorFns are functions that satisfy the ConnectDisconnector
// interface.
type ConnectDisconnectorFns struct {
	ConnectFn    func(ctx context.Context, mg resource.Managed) (Client, error)
	DisconnectFn func(ctx context.Context) error
}

// Connect to the external system specified by the supplied managed resource and
// produce an Client.
func (fns ConnectDisconnectorFns) Connect(ctx context.Context, mg resource.Managed) (Client, error) {
	return fns.ConnectFn(ctx, mg)
}

// Disconnect from the external system and close the Client.
func (fns ConnectDisconnectorFns) Disconnect(ctx context.Context) error {
	return fns.DisconnectFn(ctx)
}

// An Client manages the lifecycle of an external resource.
// None of the calls here should be blocking. All of the calls should be
// idempotent. For example, Create should not return an AlreadyExists error if
// it's called again with the same parameters.
type Client interface {
	// Observe the external resource the supplied managed resource represents,
	// if any. Observe implementations must not modify the external resource,
	// but may update the supplied Managed resource to reflect the state of the
	// external resource. Status modifications are automatically persisted
	// unless ResourceLateInitialized is true - see ResourceLateInitialized for
	// more detail.
	Observe(ctx context.Context, mg resource.Managed) (Observation, error)

	// Create an external resource per the specifications of the supplied
	// managed resource. Called when Observe reports that the associated
	// external resource does not exist. Create implementations may update
	// managed resource annotations, and those updates will be persisted. All
	// other updates will be discarded.
	Create(ctx context.Context, mg resource.Managed) (Creation, error)

	// Update the external resource represented by the supplied managed
	// resource, if necessary. Called unless Observe reports that the associated
	// external resource is up to date.
	Update(ctx context.Context, mg resource.Managed) (Update, error)

	// Delete the external resource upon deletion of its associated managed
	// resource. Called when the managed resource has been deleted.
	Delete(ctx context.Context, mg resource.Managed) error
}

// ClientFns are a series of functions that satisfy the Client interface.
type ClientFns struct {
	ObserveFn func(ctx context.Context, mg resource.Managed) (Observation, error)
	CreateFn  func(ctx context.Context, mg resource.Managed) (Creation, error)
	UpdateFn  func(ctx context.Context, mg resource.Managed) (Update, error)
	DeleteFn  func(ctx context.Context, mg resource.Managed) error
}

// Observe the external resource the supplied managed resource represents.
func (e ClientFns) Observe(ctx context.Context, mg resource.Managed) (Observation, error) {
	return e.ObserveFn(ctx, mg)
}

// Create an external resource per the specifications of the supplied managed
// resource.
func (e ClientFns) Create(ctx context.Context, mg resource.Managed) (Creation, error) {
	return e.CreateFn(ctx, mg)
}

// Update the external resource represented by the supplied managed resource.
func (e ClientFns) Update(ctx context.Context, mg resource.Managed) (Update, error) {
	return e.UpdateFn(ctx, mg)
}

// Delete the external resource represented by the supplied managed resource.
func (e ClientFns) Delete(ctx context.Context, mg resource.Managed) error {
	return e.DeleteFn(ctx, mg)
}

// A NopConnector does nothing.
type NopConnector struct{}

// Connect returns a NopClient. It never returns an error.
func (c *NopConnector) Connect(_ context.Context, _ resource.Managed) (Client, error) {
	return &NopClient{}, nil
}

// A NopClient does nothing.
type NopClient struct{}

// Observe does nothing. It returns an empty Observation and no error.
func (c *NopClient) Observe(_ context.Context, _ resource.Managed) (Observation, error) {
	return Observation{}, nil
}

// Create does nothing. It returns an empty Creation and no error.
func (c *NopClient) Create(_ context.Context, _ resource.Managed) (Creation, error) {
	return Creation{}, nil
}

// Update does nothing. It returns an empty Update and no error.
func (c *NopClient) Update(_ context.Context, _ resource.Managed) (Update, error) {
	return Update{}, nil
}

// Delete does nothing. It never returns an error.
func (c *NopClient) Delete(_ context.Context, _ resource.Managed) error { return nil }

// An Observation is the result of an observation of an external
// resource.
type Observation struct {
	// ResourceExists must be true if a corresponding external resource exists
	// for the managed resource. Typically this is proven by the presence of an
	// external resource of the expected kind whose unique identifier matches
	// the managed resource's external name. Crossplane uses this information to
	// determine whether it needs to create or delete the external resource.
	ResourceExists bool

	// ResourceUpToDate should be true if the corresponding external resource
	// appears to be up-to-date - i.e. updating the external resource to match
	// the desired state of the managed resource would be a no-op. Keep in mind
	// that often only a subset of external resource fields can be updated.
	// Crossplane uses this information to determine whether it needs to update
	// the external resource.
	ResourceUpToDate bool

	// ResourceLateInitialized should be true if the managed resource's spec was
	// updated during its observation. A Crossplane provider may update a
	// managed resource's spec fields after it is created or updated, as long as
	// the updates are limited to setting previously unset fields, and adding
	// keys to maps. Crossplane uses this information to determine whether
	// changes to the spec were made during observation that must be persisted.
	// Note that changes to the spec will be persisted before changes to the
	// status, and that pending changes to the status may be lost when the spec
	// is persisted. Status changes will be persisted by the first subsequent
	// observation that _does not_ late initialize the managed resource, so it
	// is important that Observe implementations do not late initialize the
	// resource every time they are called.
	ResourceLateInitialized bool

	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails resource.ConnectionDetails

	// Diff is a Debug level message that is sent to the reconciler when
	// there is a change in the observed Managed Resource. It is useful for
	// finding where the observed diverges from the desired state.
	// The string should be a cmp.Diff that details the difference.
	Diff string
}

// An Creation is the result of the creation of an external resource.
type Creation struct {
	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails resource.ConnectionDetails
}

// An Update is the result of an update to an external resource.
type Update struct {
	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails resource.ConnectionDetails
}
