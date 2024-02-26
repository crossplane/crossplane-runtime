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

package managed

import (
	"github.com/crossplane/crossplane-runtime/pkg/external"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// ConnectionDetails created or updated during an operation on an external
// resource, for example usernames, passwords, endpoints, ports, etc.
//
// Deprecated: Use resource.ConnectionDetails.
type ConnectionDetails = resource.ConnectionDetails

// An ExternalConnecter produces a new Client for an external system given the
// supplied managed resource.
//
// Deprecated: Use external.Connector.
type ExternalConnecter = external.Connector

// An ExternalDisconnector disconnects from an external system.
//
// Deprecated: Use external.ExternalDisconnector.
type ExternalDisconnector = external.Disconnector

// A NopDisconnector converts an Connector into an ConnectDisconnector with a
// no-op Disconnect method.
//
// Deprecated: Use external.NopDisconnector.
type NopDisconnector = external.NopDisconnector

// NewNopDisconnecter converts an Connector into an ConnectDisconnector with a
// no-op Disconnect method.
//
// Deprecated: Use external.NewNopDisconnecter.
func NewNopDisconnecter(c ExternalConnecter) ExternalConnectDisconnecter {
	return external.NewNopDisconnector(c)
}

// An ExternalConnectDisconnecter produces a new Client given the supplied
// managed resource. It supports disconnecting from the client.
//
// Deprecated: Use external.ConnectDisconnector.
type ExternalConnectDisconnecter = external.ConnectDisconnector

// An ExternalConnectorFn is a function that satisfies the ExternalConnector
// interface.
//
// Deprecated: Use external.ConnectorFn
type ExternalConnectorFn = external.ConnectorFn

// An ExternalDisconnectorFn is a function that satisfies the Disonnector interface.
type ExternalDisconnectorFn = external.DisconnectorFn

// ExternalConnectDisconnecterFns are functions that satisfy the ConnectDisconnector
// interface.
type ExternalConnectDisconnecterFns = external.ConnectDisconnectorFns

// An ExternalClient manages the lifecycle of an external resource.
// None of the calls here should be blocking. All of the calls should be
// idempotent. For example, Create should not return an AlreadyExists error if
// it's called again with the same parameters.
//
// Deprecated: Use external.Client.
type ExternalClient = external.Client

// ExternalClientFns are a series of functions that satisfy the Client interface.
//
// Deprecated: Use external.ExternalClientFns
type ExternalClientFns = external.ClientFns

// A NopConnecter does nothing.
//
// Deprecated: Use external.NopConnector.
type NopConnecter = external.NopConnector

// A NopClient does nothing.
//
// Deprecated: Use external.NopClient.
type NopClient = external.NopClient

// An ExternalObservation is the result of an observation of an external
// resource.
//
// Deprecated: Use external.ExternalObservation.
type ExternalObservation = external.Observation

// An ExternalCreation is the result of the creation of an external resource.
//
// Deprecated: Use external.ExternalCreation.
type ExternalCreation = external.Creation

// An ExternalUpdate is the result of an update to an external resource.
//
// Deprecated: Use external.ExternalUpdate.
type ExternalUpdate = external.Update
