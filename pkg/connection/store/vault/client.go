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

package vault

import (
	"path/filepath"
	"reflect"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/hashicorp/vault/api"
)

const (
	errRead     = "cannot read vault secret"
	errWrite    = "cannot write vault secret"
	errNotFound = "secret not found"

	errFmtUnexpectedValue = "expecting a string as a value for key %q, but it is %q"
)

// KVVersion represent API version of the Vault KV engine
// https://www.vaultproject.io/docs/secrets/kv
type KVVersion string

const (
	// KVVersionV1 indicates that "Kubernetes Auth" will be used to
	// authenticate to Vault.
	// https://www.vaultproject.io/docs/auth/kubernetes
	KVVersionV1 KVVersion = "v1"

	// KVVersionV2 indicates that "Token Auth" will be used to
	// authenticate to Vault.
	// https://www.vaultproject.io/docs/auth/token
	KVVersionV2 KVVersion = "v2"
)

// KVSecret is a KV Engine secret
type KVSecret struct {
	metadata map[string]string
	data     map[string][]byte
}

type KVOption func(*KV)

func WithVersion(v KVVersion) KVOption {
	return func(kv *KV) {
		kv.version = v
	}
}

type KV struct {
	client    *api.Logical
	mountPath string
	version   KVVersion
}

func NewKV(logical *api.Logical, mountPath string, opts ...KVOption) *KV {
	kv := &KV{
		client:    logical,
		mountPath: mountPath,
		version:   KVVersionV2,
	}

	for _, o := range opts {
		o(kv)
	}

	return kv
}

func (k *KV) Get(path string, secret *KVSecret) error {
	s, err := k.client.Read(k.pathForData(path))
	if err != nil {
		return errors.Wrap(err, errRead)
	}
	if s == nil {
		return errors.New(errNotFound)
	}
	return k.parseAsKVSecret(s, secret)
}

func (k *KV) Apply(path string, secret *KVSecret) error {
	_, err := k.client.Write(k.pathForData(path), k.secretDataFor(secret))
	return errors.Wrap(err, errWrite)
}

func (k *KV) Delete(path string) error {
	if k.version == KVVersionV1 {
		_, err := k.client.Delete(filepath.Join(k.mountPath, path))
		return errors.Wrap(err, errDelete)
	}

	// Note(turkenh): With KV v2, we need to delete metadata and all versions:
	// https://www.vaultproject.io/api-docs/secret/kv/kv-v2#delete-metadata-and-all-versions
	_, err := k.client.Delete(filepath.Join(k.mountPath, "metadata", path))
	return errors.Wrap(err, errDelete)
}

func isNotFound(err error) bool {
	return err.Error() == errNotFound
}

func (k *KV) pathForData(path string) string {
	if k.version == KVVersionV1 {
		return filepath.Join(k.mountPath, path)
	}
	return filepath.Join(k.mountPath, "data", path)
}

func (k *KV) parseAsKVSecret(s *api.Secret, kv *KVSecret) error {
	var err error
	if k.version == KVVersionV1 {
		kv.data, err = valuesAsByteArray(s.Data)
		return err
	}

	// kv version is v2

	// Note(turkenh): kv v2 secrets contains another "data" and a "metadata"
	// block inside the top level generic "data" field.
	// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
	if sData, ok := s.Data["data"].(map[string]interface{}); ok && sData != nil {
		if kv.data, err = valuesAsByteArray(sData); err != nil {
			return err
		}
	}
	if sMeta, ok := s.Data["metadata"].(map[string]interface{}); ok && sMeta != nil {
		if kv.metadata, err = valuesAsString(sMeta); err != nil {
			return err
		}
	}
	return nil
}

func (k *KV) secretDataFor(kv *KVSecret) map[string]interface{} {
	if k.version == KVVersionV1 {
		// There is no metadata for a v1 kv secret
		return valuesAsInterface(kv.data)
	}

	// kv version is v2

	// Note(turkenh): kv v2 secrets contains another "data" and a "metadata"
	// block inside the top level generic "data" field.
	// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1

	out := make(map[string]interface{}, 2)
	out["data"] = byteArrayValuesAsString(kv.data)
	out["metadata"] = kv.metadata

	return out
}

func valuesAsByteArray(in map[string]interface{}) (map[string][]byte, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string][]byte, len(in))
	for key, val := range in {
		sVal, ok := val.(string)
		if !ok {
			return nil, errors.Errorf(errFmtUnexpectedValue, key, reflect.TypeOf(val))
		}
		out[key] = []byte(sVal)
	}
	return out, nil
}

func valuesAsString(in map[string]interface{}) (map[string]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(in))
	for key, val := range in {
		sVal, ok := val.(string)
		if !ok {
			return nil, errors.Errorf(errFmtUnexpectedValue, key, reflect.TypeOf(val))
		}
		out[key] = sVal
	}
	return out, nil
}

func byteArrayValuesAsString(in map[string][]byte) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))

	for key, val := range in {
		out[key] = string(val)
	}
	return out
}

func valuesAsInterface(in map[string][]byte) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, val := range in {
		out[key] = val
	}
	return out
}
