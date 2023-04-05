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

package kv

import (
	"encoding/json"
	"path/filepath"

	"github.com/hashicorp/vault/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errWriteMetadata = "cannot write secret metadata Data"
)

// V2Client is a Vault KV V2 Secrets Engine client.
// https://www.vaultproject.io/api/secret/kv/kv-v2
type V2Client struct {
	client    LogicalClient
	mountPath string
}

// NewV2Client returns a new V2Client.
func NewV2Client(logical LogicalClient, mountPath string) *V2Client {
	kv := &V2Client{
		client:    logical,
		mountPath: mountPath,
	}

	return kv
}

// Get returns a Secret at a given path.
func (c *V2Client) Get(path string, secret *Secret) error {
	s, err := c.client.Read(c.dataPath(path))
	if err != nil {
		return errors.Wrap(err, errRead)
	}
	if s == nil {
		return errors.New(ErrNotFound)
	}
	return c.parseAsKVSecret(s, secret)
}

// Apply applies given Secret at path by patching its Data and setting
// provided custom metadata.
func (c *V2Client) Apply(path string, secret *Secret, ao ...ApplyOption) error {
	existing := &Secret{}
	err := c.Get(path, existing)

	if resource.Ignore(IsNotFound, err) != nil {
		return errors.Wrap(err, errGet)
	}
	if !IsNotFound(err) {
		for _, o := range ao {
			if err = o(existing, secret); err != nil {
				return err
			}
		}
	}

	// We write metadata first to ensure we set ownership (with the label) of
	// the secret before writing any data. This is to prevent situations where
	// secret create with some data but owner not set.
	mp, changed := metadataPayload(existing.CustomMeta, secret.CustomMeta)
	if changed {
		if _, err := c.client.Write(c.metadataPath(path), mp); err != nil {
			return errors.Wrap(err, errWriteMetadata)
		}
	}

	dp, changed := dataPayload(existing, secret)
	if changed {
		if _, err := c.client.Write(c.dataPath(path), dp); err != nil {
			return errors.Wrap(err, errWriteData)
		}
	}

	return nil
}

// Delete deletes Secret at the given path.
func (c *V2Client) Delete(path string) error {
	// Note(turkenh): With V2Client, we need to delete metadata and all versions:
	// https://www.vaultproject.io/api-docs/secret/kv/kv-v2#delete-metadata-and-all-versions
	_, err := c.client.Delete(c.metadataPath(path))
	return errors.Wrap(err, errDelete)
}

func dataPayload(existing, new *Secret) (map[string]any, bool) {
	data := make(map[string]string, len(existing.Data)+len(new.Data))
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
	return map[string]any{
		"options": map[string]any{
			"cas": ver,
		},
		"data": data,
	}, changed
}

func metadataPayload(existing, new map[string]string) (map[string]any, bool) {
	payload := map[string]any{
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

func (c *V2Client) parseAsKVSecret(s *api.Secret, kv *Secret) error {
	// Note(turkenh): kv v2 secrets contains another "data" and "metadata"
	// blocks inside the top level generic "Data" field.
	// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
	paved := fieldpath.Pave(s.Data)
	if err := parseSecretData(paved, kv); err != nil {
		return err
	}
	return parseSecretMeta(paved, kv)
}

func parseSecretData(payload *fieldpath.Paved, kv *Secret) error {
	sData := map[string]any{}
	err := payload.GetValueInto("data", &sData)
	if fieldpath.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	kv.Data = make(map[string]string, len(sData))
	for key, val := range sData {
		if sVal, ok := val.(string); ok {
			kv.Data[key] = sVal
		}
	}
	return nil
}

func parseSecretMeta(payload *fieldpath.Paved, kv *Secret) error {
	sMeta := map[string]any{}
	err := payload.GetValueInto("metadata", &sMeta)
	if fieldpath.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	pavedMeta := fieldpath.Pave(sMeta)
	if err = pavedMeta.GetValueInto("version", &kv.version); resource.Ignore(fieldpath.IsNotFound, err) != nil {
		return err
	}

	customMeta := map[string]any{}
	err = pavedMeta.GetValueInto("custom_metadata", &customMeta)
	if fieldpath.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	kv.CustomMeta = make(map[string]string, len(customMeta))
	for key, val := range customMeta {
		if sVal, ok := val.(string); ok {
			kv.CustomMeta[key] = sVal
		}
	}
	return nil
}

func (c *V2Client) dataPath(secretPath string) string {
	return filepath.Join(c.mountPath, "data", secretPath)
}

func (c *V2Client) metadataPath(secretPath string) string {
	return filepath.Join(c.mountPath, "metadata", secretPath)
}
