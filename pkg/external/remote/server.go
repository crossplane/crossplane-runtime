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

package remote

import (
	"context"

	"github.com/crossplane/crossplane-runtime/apis/proto/external/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/external"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TODO(negz): Implement an ExternalServiceServer that routes to a series of
// ExternalConnectDisconnector implementations by GVK. On each method call:
//
// 1. Get by GVK
// 2. Connect
// 3. Make call
// 4. Disconnect
// 5. Return response

var _ v1alpha1.ExternalServiceServer = &Server{}

type Server struct {
	v1alpha1.UnimplementedExternalServiceServer

	clients map[schema.GroupVersionKind]external.ConnectDisconnector
}

func (s *Server) Observe(context.Context, *v1alpha1.ObserveRequest) (*v1alpha1.ObserveResponse, error) {
	return nil, nil
}

func (s *Server) Create(context.Context, *v1alpha1.CreateRequest) (*v1alpha1.CreateResponse, error) {
	return nil, nil
}

func (s *Server) Update(context.Context, *v1alpha1.UpdateRequest) (*v1alpha1.UpdateResponse, error) {
	return nil, nil
}

func (s *Server) Delete(context.Context, *v1alpha1.DeleteRequest) (*v1alpha1.DeleteResponse, error) {
	return nil, nil
}
