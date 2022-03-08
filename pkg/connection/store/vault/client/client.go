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

package client

import (
	"encoding/json"
	"path/filepath"

	"github.com/hashicorp/vault/api"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errGet           = "cannot get secret"
	errDelete        = "cannot delete secret"
	errRead          = "cannot read secret"
	errWriteData     = "cannot write secret Data"
	errWriteMetadata = "cannot write secret metadata Data"

	// ErrNotFound is the error returned when secret does not exist.
	ErrNotFound = "secret not found"
)

// KVSecret is a KVAdditiveClient Engine secret
type KVSecret struct {
	CustomMeta map[string]interface{}
	Data       map[string]interface{}
	version    json.Number
}

// LogicalClient is a client to perform logical backend operations on Vault.
type LogicalClient interface {
	Read(path string) (*api.Secret, error)
	Write(path string, data map[string]interface{}) (*api.Secret, error)
	Delete(path string) (*api.Secret, error)
}

// KVAdditiveOption configures a KVAdditiveClient.
type KVAdditiveOption func(*KVAdditiveClient)

// WithVersion specifies which version of KVAdditiveClient Secrets engine to be used.
func WithVersion(v *v1.VaultKVVersion) KVAdditiveOption {
	return func(kv *KVAdditiveClient) {
		if v != nil {
			kv.version = *v
		}
	}
}

// KVAdditiveClient is a Vault KV Secrets Engine client that adds new data
// to existing ones while applying secrets.
type KVAdditiveClient struct {
	client    LogicalClient
	mountPath string
	version   v1.VaultKVVersion
}

// NewAdditiveClient returns a KVAdditiveClient.
func NewAdditiveClient(logical LogicalClient, mountPath string, opts ...KVAdditiveOption) *KVAdditiveClient {
	kv := &KVAdditiveClient{
		client:    logical,
		mountPath: mountPath,
		version:   v1.VaultKVVersionV2,
	}

	for _, o := range opts {
		o(kv)
	}

	return kv
}

// Get returns a KVSecret at a given path.
func (k *KVAdditiveClient) Get(path string, secret *KVSecret) error {
	dataPath := filepath.Join(k.mountPath, path)
	if k.version == v1.VaultKVVersionV2 {
		dataPath = filepath.Join(k.mountPath, "data", path)
	}
	s, err := k.client.Read(dataPath)
	if err != nil {
		return errors.Wrap(err, errRead)
	}
	if s == nil {
		return errors.New(ErrNotFound)
	}
	return k.parseAsKVSecret(s, secret)
}

// Apply applies given KVSecret at path by patching its Data and setting
// provided custom metadata.
func (k *KVAdditiveClient) Apply(path string, secret *KVSecret) error {
	existing := &KVSecret{}
	if err := k.Get(path, existing); resource.Ignore(IsNotFound, err) != nil {
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

	mp, changed := metadataPayload(existing.CustomMeta, secret.CustomMeta)
	// Update metadata only if there is some Data in secret.
	if len(existing.Data) > 0 && changed {
		if _, err := k.client.Write(filepath.Join(k.mountPath, "metadata", path), mp); err != nil {
			return errors.Wrap(err, errWriteMetadata)
		}
	}

	return nil
}

// Delete deletes KVSecret at the given path.
func (k *KVAdditiveClient) Delete(path string) error {
	if k.version == v1.VaultKVVersionV1 {
		_, err := k.client.Delete(filepath.Join(k.mountPath, path))
		return errors.Wrap(err, errDelete)
	}

	// Note(turkenh): With KVAdditiveClient v2, we need to delete metadata and all versions:
	// https://www.vaultproject.io/api-docs/secret/kv/kv-v2#delete-metadata-and-all-versions
	_, err := k.client.Delete(filepath.Join(k.mountPath, "metadata", path))
	return errors.Wrap(err, errDelete)
}

func dataPayloadV2(existing, new *KVSecret) (map[string]interface{}, bool) {
	data := make(map[string]interface{}, len(existing.Data)+len(new.Data))
	for k, v := range existing.Data {
		data[k] = v
	}
	changed := false
	for k, v := range new.Data {
		if ev, ok := existing.Data[k]; !ok || ev != v {
			changed = true
			data[k] = v
		}
	}
	ver := json.Number("0")
	if existing.version != "" {
		ver = existing.version
	}
	return map[string]interface{}{
		"options": map[string]interface{}{
			"cas": ver,
		},
		"data": data,
	}, changed
}

func metadataPayload(existing, new map[string]interface{}) (map[string]interface{}, bool) {
	payload := map[string]interface{}{
		"custom_metadata": new,
	}
	if len(existing) != len(new) {
		return payload, true
	}
	for k, v := range new {
		if ev, ok := existing[k]; !ok || ev != v {
			return payload, true
		}
	}
	return payload, false
}

func dataPayloadV1(existing, new *KVSecret) (map[string]interface{}, bool) {
	data := make(map[string]interface{}, len(existing.Data)+len(new.Data))
	for k, v := range existing.Data {
		data[k] = v
	}
	changed := false
	for k, v := range new.Data {
		if ev, ok := existing.Data[k]; !ok || ev != v {
			changed = true
			data[k] = v
		}
	}
	return data, changed
}

func (k *KVAdditiveClient) parseAsKVSecret(s *api.Secret, kv *KVSecret) error {
	if k.version == v1.VaultKVVersionV1 {
		kv.Data = s.Data
		return nil
	}

	// kv version is v2

	// Note(turkenh): kv v2 secrets contains another "Data" and a "metadata"
	// block inside the top level generic "Data" field.
	// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
	if sData, ok := s.Data["data"].(map[string]interface{}); ok && sData != nil {
		kv.Data = sData
	}
	if sMeta, ok := s.Data["metadata"].(map[string]interface{}); ok && sMeta != nil {
		kv.version, _ = sMeta["version"].(json.Number)
		if cMeta, ok := sMeta["custom_metadata"].(map[string]interface{}); ok && cMeta != nil {
			kv.CustomMeta = cMeta
		}
	}
	return nil
}

// IsNotFound returns whether given error is a "Not Found" error or not.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == ErrNotFound
}
