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

// Package fake is a fake Vault KVClient.
package fake

import (
	"github.com/crossplane/crossplane-runtime/pkg/connection/store/vault/kv"
)

// KVClient is a fake KVClient.
type KVClient struct {
	GetFn    func(path string, secret *kv.Secret) error
	ApplyFn  func(path string, secret *kv.Secret, ao ...kv.ApplyOption) error
	DeleteFn func(path string) error
}

// Get fetches a secret at a given path.
func (k *KVClient) Get(path string, secret *kv.Secret) error {
	return k.GetFn(path, secret)
}

// Apply creates or updates a secret at a given path.
func (k *KVClient) Apply(path string, secret *kv.Secret, ao ...kv.ApplyOption) error {
	return k.ApplyFn(path, secret, ao...)
}

// Delete deletes a secret at a given path.
func (k *KVClient) Delete(path string) error {
	return k.DeleteFn(path)
}
