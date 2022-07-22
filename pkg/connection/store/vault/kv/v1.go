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
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// We use this prefix to store metadata of v1 secrets as there is no dedicated
// metadata. Considering a connection key cannot contain ":" (since it is not
// in the set of allowed chars for a k8s secret key), it is safe to assume
// there is no actual connection data starting with this prefix.
const metadataPrefix = "metadata:"

// V1Client is a Vault KV V1 Secrets Engine client.
// https://www.vaultproject.io/api-docs/secret/kv/kv-v1
type V1Client struct {
	client    LogicalClient
	mountPath string
}

// NewV1Client returns a new V1Client.
func NewV1Client(logical LogicalClient, mountPath string) *V1Client {
	kv := &V1Client{
		client:    logical,
		mountPath: mountPath,
	}

	return kv
}

// Get returns a Secret at a given path.
func (c *V1Client) Get(path string, secret *Secret) error {
	s, err := c.client.Read(filepath.Join(c.mountPath, path))
	if err != nil {
		return errors.Wrap(err, errRead)
	}
	if s == nil {
		return errors.New(ErrNotFound)
	}
	return c.parseAsSecret(s, secret)
}

// Apply applies given Secret at path by patching its Data and setting
// provided custom metadata.
func (c *V1Client) Apply(path string, secret *Secret, ao ...ApplyOption) error {
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

	dp, changed := payloadV1(existing, secret)
	if !changed {
		return nil
	}
	_, err = c.client.Write(filepath.Join(c.mountPath, path), dp)
	return errors.Wrap(err, errWriteData)

}

// Delete deletes Secret at the given path.
func (c *V1Client) Delete(path string) error {
	_, err := c.client.Delete(filepath.Join(c.mountPath, path))
	return errors.Wrap(err, errDelete)
}

func (c *V1Client) parseAsSecret(s *api.Secret, kv *Secret) error {
	for key, val := range s.Data {
		if sVal, ok := val.(string); ok {
			if strings.HasPrefix(key, metadataPrefix) {
				kv.AddMetadata(strings.TrimPrefix(key, metadataPrefix), sVal)
				continue
			}
			kv.AddData(key, sVal)
		}
	}
	return nil
}

func payloadV1(existing, new *Secret) (map[string]any, bool) {
	payload := make(map[string]any, len(existing.Data)+len(new.Data))
	for k, v := range existing.Data {
		// Only transfer existing data, metadata updates are not additive.
		if !strings.HasPrefix(k, metadataPrefix) {
			payload[k] = v
		}
	}
	changed := false
	for k, v := range new.Data {
		if ev, ok := existing.Data[k]; !ok || ev != v {
			changed = true
			payload[k] = v
		}
	}
	for k, v := range new.CustomMeta {
		// kv secret engine v1 does not have metadata. So, we store them as data
		// by prefixing with "metadata:"
		if val, ok := existing.CustomMeta[k]; !ok && val != v {
			changed = true
		}
		payload[metadataPrefix+k] = v
	}
	return payload, changed
}
