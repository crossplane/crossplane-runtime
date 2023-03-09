/*
Copyright 2023 The Crossplane Authors.

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

// Package external implements a gRPC client for external secret store plugins.
package external

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	ess "github.com/crossplane/crossplane-runtime/apis/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errGet    = "cannot get secret"
	errApply  = "cannot apply secret"
	errDelete = "cannot delete secret"
)

// SecretStore is an External Secret Store.
type SecretStore struct {
	client     ess.ExternalSecretStoreServiceClient
	kubeClient client.Client
	config     *v1.Config
}

// NewSecretStore returns a new External SecretStore.
func NewSecretStore(ctx context.Context, kube client.Client, tlsConfig *tls.Config, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	if cfg.Plugin.Endpoint == nil {
		return nil, errors.New("endpoint is not provided")
	}

	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.Dial(*(cfg.Plugin.Endpoint), grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("cannot dial to the endpoint: %s", *cfg.Plugin.Endpoint))
	}

	cl := ess.NewExternalSecretStoreServiceClient(conn)

	return &SecretStore{
		kubeClient: kube,
		client:     cl,
		config:     cfg.Plugin.ConfigRef,
	}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Secret.
func (ss *SecretStore) ReadKeyValues(ctx context.Context, n store.ScopedName, s *store.Secret) error {
	sec := new(ess.Secret)
	sec.ScopedName = filepath.Join(n.Scope, n.Name)

	cfg := ss.getConfigReference()

	res, err := ss.client.GetSecret(ctx, &ess.GetSecretRequest{Secret: sec, Config: cfg})
	if err != nil {
		return errors.Wrap(err, errGet)
	}

	s.ScopedName = n
	s.Data = make(map[string][]byte, len(res.Secret.Data))
	for d := range res.Secret.Data {
		s.Data[d] = res.Secret.Data[d]
	}
	if res.Secret != nil && len(res.Secret.Metadata) != 0 {
		s.Metadata = new(v1.ConnectionSecretMetadata)
		s.Metadata.Labels = make(map[string]string, len(res.Secret.Metadata))
		for k, v := range res.Secret.Metadata {
			s.Metadata.Labels[k] = v
		}
	}

	return nil
}

// WriteKeyValues writes key value pairs to a given Secret.
func (ss *SecretStore) WriteKeyValues(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (changed bool, err error) {
	sec := new(ess.Secret)
	sec.ScopedName = filepath.Join(s.Scope, s.Name)
	sec.Data = make(map[string][]byte, len(s.Data))
	for k, v := range s.Data {
		sec.Data[k] = v
	}

	if s.Metadata != nil && len(s.Metadata.Labels) != 0 {
		sec.Metadata = make(map[string]string, len(s.Metadata.Labels))
		for k, v := range s.Metadata.Labels {
			sec.Metadata[k] = v
		}
	}

	cfg := ss.getConfigReference()

	res, err := ss.client.ApplySecret(ctx, &ess.ApplySecretRequest{Secret: sec, Config: cfg})
	if err != nil {
		return false, errors.Wrap(err, errApply)
	}

	return res.Changed, nil
}

// DeleteKeyValues delete key value pairs from a given Secret.
func (ss *SecretStore) DeleteKeyValues(ctx context.Context, s *store.Secret, do ...store.DeleteOption) error {
	sec := new(ess.Secret)
	sec.ScopedName = filepath.Join(s.Scope, s.Name)

	cfg := ss.getConfigReference()

	_, err := ss.client.DeleteKeys(ctx, &ess.DeleteKeysRequest{Secret: sec, Config: cfg})
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	return nil
}

func (ss *SecretStore) getConfigReference() *ess.ConfigReference {
	cfg := new(ess.ConfigReference)
	cfg.ApiVersion = ss.config.APIVersion
	cfg.Kind = ss.config.Kind
	cfg.Name = ss.config.Name

	return cfg
}
