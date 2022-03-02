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
	"encoding/json"
	"path/filepath"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/hashicorp/vault/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errRead          = "cannot read secret"
	errWriteData     = "cannot write secret data"
	errWriteMetadata = "cannot write secret metadata data"
	errNotFound      = "secret not found"
)

// KVSecret is a KV Engine secret
type KVSecret struct {
	customMeta map[string]interface{}
	data       map[string]interface{}
	version    json.Number
}

// KVOption configures a KV.
type KVOption func(*KV)

// WithVersion specifies which version of KV Secrets engine to be used.
func WithVersion(v *v1.VaultKVVersion) KVOption {
	return func(kv *KV) {
		if v != nil {
			kv.version = *v
		}
	}
}

// KV is a Vault KV Secrets Engine client.
type KV struct {
	client    *api.Logical
	mountPath string
	version   v1.VaultKVVersion
}

// NewKV returns a KV.
func NewKV(logical *api.Logical, mountPath string, opts ...KVOption) *KV {
	kv := &KV{
		client:    logical,
		mountPath: mountPath,
		version:   v1.VaultKVVersionV2,
	}

	for _, o := range opts {
		o(kv)
	}

	return kv
}

// Get returns KVSecret at a given path.
func (k *KV) Get(path string, secret *KVSecret) error {
	dataPath := filepath.Join(k.mountPath, path)
	if k.version == v1.VaultKVVersionV2 {
		dataPath = filepath.Join(k.mountPath, "data", path)
	}
	s, err := k.client.Read(dataPath)
	if err != nil {
		return errors.Wrap(err, errRead)
	}
	if s == nil {
		return errors.New(errNotFound)
	}
	return k.parseAsKVSecret(s, secret)
}

// Apply applies given KVSecret at path by patching its data and setting
// provided custom metadata.
func (k *KV) Apply(path string, secret *KVSecret) error {
	existing := &KVSecret{}
	if err := k.Get(path, existing); resource.Ignore(isNotFound, err) != nil {
		return errors.Wrap(err, errGet)
	}

	if k.version == v1.VaultKVVersionV1 {
		dp, changed := dataPayloadV1(existing, secret)
		if !changed {
			// No metadata in v1 secrets.
			// Hence, already up to date.
			return nil
		}
		_, err := k.client.Write(filepath.Join(k.mountPath, path), dp)
		return errors.Wrap(err, errWriteData)
	}

	dp, changed := dataPayloadV2(existing, secret)
	if changed {
		if _, err := k.client.Write(filepath.Join(k.mountPath, "data", path), dp); err != nil {
			return errors.Wrap(err, errWriteData)
		}
	}

	mp, changed := metadataPayload(existing.customMeta, secret.customMeta)
	// Update metadata only if there is some data in secret.
	if len(existing.data) > 0 && changed {
		if _, err := k.client.Write(filepath.Join(k.mountPath, "metadata", path), mp); err != nil {
			return errors.Wrap(err, errWriteMetadata)
		}
	}

	return nil
}

// Delete deletes KVSecret at the given path.
func (k *KV) Delete(path string) error {
	if k.version == v1.VaultKVVersionV1 {
		_, err := k.client.Delete(filepath.Join(k.mountPath, path))
		return errors.Wrap(err, errDelete)
	}

	// Note(turkenh): With KV v2, we need to delete metadata and all versions:
	// https://www.vaultproject.io/api-docs/secret/kv/kv-v2#delete-metadata-and-all-versions
	_, err := k.client.Delete(filepath.Join(k.mountPath, "metadata", path))
	return errors.Wrap(err, errDelete)
}

func dataPayloadV2(existing, new *KVSecret) (map[string]interface{}, bool) {
	data := make(map[string]interface{}, len(existing.data)+len(new.data))
	for k, v := range existing.data {
		data[k] = v
	}
	changed := false
	for k, v := range new.data {
		if ev, ok := existing.data[k]; !ok || ev != v {
			changed = true
			data[k] = v
		}
	}
	return map[string]interface{}{
		"options": map[string]interface{}{
			"cas": existing.version,
		},
		"data": data,
	}, changed
}

func metadataPayload(existing, new map[string]interface{}) (map[string]interface{}, bool) {
	changed := false
	for k, v := range new {
		if ev, ok := existing[k]; !ok || ev != v {
			changed = true
		}
	}
	return map[string]interface{}{
		"custom_metadata": new,
	}, changed
}

func dataPayloadV1(existing, new *KVSecret) (map[string]interface{}, bool) {
	data := make(map[string]interface{}, len(existing.data)+len(new.data))
	for k, v := range existing.data {
		data[k] = v
	}
	changed := false
	for k, v := range new.data {
		if ev, ok := existing.data[k]; !ok || ev != v {
			changed = true
			data[k] = v
		}
	}
	return data, changed
}

func (k *KV) parseAsKVSecret(s *api.Secret, kv *KVSecret) error {
	if k.version == v1.VaultKVVersionV1 {
		kv.data = s.Data
	}

	// kv version is v2

	// Note(turkenh): kv v2 secrets contains another "data" and a "metadata"
	// block inside the top level generic "data" field.
	// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
	if sData, ok := s.Data["data"].(map[string]interface{}); ok && sData != nil {
		kv.data = sData
	}
	if sMeta, ok := s.Data["metadata"].(map[string]interface{}); ok && sMeta != nil {
		kv.version, _ = sMeta["version"].(json.Number)
		if cMeta, ok := sMeta["custom_metadata"].(map[string]interface{}); ok && cMeta != nil {
			kv.customMeta = cMeta
		}
	}
	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == errNotFound
}
