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
	"crypto/x509"
	"fmt"
	"os"

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
	errLoadCert  = "cannot load client certificates"
	errLoadCA    = "cannot load CA certificate"
	errInvalidCA = "invalid CA certificate"

	errGet    = "cannot get secret"
	errApply  = "cannot apply secret"
	errDelete = "cannot delete secret"
)

var (
	certsPathFmt = "/certs/%s"
	caCertFile   = "ca.crt"
	tlsCertFile  = "tls.crt"
	tlsKeyFile   = "tls.key"
)

// SecretStore is an External Secret Store.
type SecretStore struct {
	client     ess.ExternalSecretStoreServiceClient
	kubeClient client.Client
	ctx        context.Context
	config     *v1.Config
}

// loadKeyPair loads CA and client certificates.
func loadKeyPair() (credentials.TransportCredentials, error) {
	certificate, err := tls.LoadX509KeyPair(fmt.Sprintf(certsPathFmt, tlsCertFile), fmt.Sprintf(certsPathFmt, tlsKeyFile))
	if err != nil {
		return nil, errors.Wrap(err, errLoadCert)
	}

	ca, err := os.ReadFile(fmt.Sprintf(certsPathFmt, caCertFile))
	if err != nil {
		return nil, errors.Wrap(err, errLoadCA)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New(errInvalidCA)
	}

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      pool,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// NewSecretStore returns a new External SecretStore.
func NewSecretStore(ctx context.Context, kube client.Client, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	kp, err := loadKeyPair()
	if err != nil {
		return nil, errors.Wrap(err, errLoadCert)
	}

	conn, err := grpc.Dial(cfg.External.Endpoint, grpc.WithTransportCredentials(kp))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("cannot dial to the endpoint: %s", cfg.External.Endpoint))
	}

	cl := ess.NewExternalSecretStoreServiceClient(conn)
	return &SecretStore{
		ctx:        ctx,
		kubeClient: kube,
		client:     cl,
		config:     cfg.External.ConfigRef,
	}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Secret.
func (ss *SecretStore) ReadKeyValues(ctx context.Context, n store.ScopedName, s *store.Secret) error {
	sec := new(ess.Secret)
	sec.ScopedName = n.Name

	cfg := ss.getConfigReference()

	res, err := ss.client.GetSecret(ctx, &ess.GetSecretRequest{Secret: sec, Config: cfg})
	if err != nil {
		return errors.Wrap(err, errGet)
	}

	s.Data = make(map[string][]byte, len(res.Secret.Data))
	for d := range res.Secret.Data {
		res.Secret.Data[d] = s.Data[d]
	}

	if res.Secret.Metadata != nil {
		s.Metadata.Labels = make(map[string]string, len(res.Secret.Metadata))
		for m := range res.Secret.Metadata {
			res.Secret.Metadata[m] = s.Metadata.Labels[m]
		}
	}

	return nil
}

// WriteKeyValues writes key value pairs to a given Secret.
func (ss *SecretStore) WriteKeyValues(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (changed bool, err error) {
	sec := new(ess.Secret)
	sec.ScopedName = s.Name

	cfg := ss.getConfigReference()

	sec.Data = make(map[string][]byte, len(s.Data))
	for d := range s.Data {
		sec.Data[d] = s.Data[d]
	}

	if sec.Metadata != nil {
		sec.Metadata = make(map[string]string, len(s.Metadata.Labels))
		for m := range s.Metadata.Labels {
			sec.Metadata[m] = s.Metadata.Labels[m]
		}
	}

	res, err := ss.client.ApplySecret(ctx, &ess.ApplySecretRequest{Secret: sec, Config: cfg})
	if err != nil {
		return false, errors.Wrap(err, errApply)
	}

	return res.Changed, nil
}

// DeleteKeyValues delete key value pairs from a given Secret.
func (ss *SecretStore) DeleteKeyValues(ctx context.Context, s *store.Secret, do ...store.DeleteOption) error {
	sec := new(ess.Secret)
	sec.ScopedName = s.Name

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
