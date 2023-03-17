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

// Package certificates loads TLS certificates from a given folder.
package certificates

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	caCertFileName  = "ca.crt"
	tlsCertFileName = "tls.crt"
	tlsKeyFileName  = "tls.key"
)

const (
	errLoadCert  = "cannot load certificate"
	errLoadCA    = "cannot load CA certificate"
	errInvalidCA = "invalid CA certificate"
)

// LoadMTLSConfig loads TLS certificates in the given folder using well-defined filenames for certificates in a Kubernetes environment.
func LoadMTLSConfig(certsFolderPath string, isServer bool) (*tls.Config, error) {
	tlsCertFilePath := filepath.Clean(filepath.Join(certsFolderPath, tlsCertFileName))
	tlsKeyFilePath := filepath.Clean(filepath.Join(certsFolderPath, tlsKeyFileName))
	certificate, err := tls.LoadX509KeyPair(tlsCertFilePath, tlsKeyFilePath)
	if err != nil {
		return nil, errors.Wrap(err, errLoadCert)
	}

	caCertFilePath := filepath.Clean(filepath.Join(certsFolderPath, caCertFileName))
	ca, err := os.ReadFile(caCertFilePath)
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
	}

	if isServer {
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	} else {
		tlsConfig.RootCAs = pool
	}

	return tlsConfig, nil
}
