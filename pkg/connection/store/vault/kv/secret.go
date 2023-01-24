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

// Package kv represents Vault key-value pairs.
package kv

import (
	"encoding/json"

	"github.com/hashicorp/vault/api"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errGet              = "cannot get secret"
	errDelete           = "cannot delete secret"
	errRead             = "cannot read secret"
	errWriteData        = "cannot write secret Data"
	errUpdateNotAllowed = "update not allowed"

	// ErrNotFound is the error returned when secret does not exist.
	ErrNotFound = "secret not found"
)

// LogicalClient is a client to perform logical backend operations on Vault.
type LogicalClient interface {
	Read(path string) (*api.Secret, error)
	Write(path string, data map[string]any) (*api.Secret, error)
	Delete(path string) (*api.Secret, error)
}

// Secret is a Vault KV secret.
type Secret struct {
	CustomMeta map[string]string
	Data       map[string]string
	version    json.Number
}

// NewSecret returns a new Secret.
func NewSecret(data map[string]string, meta map[string]string) *Secret {
	return &Secret{
		Data:       data,
		CustomMeta: meta,
	}
}

// AddData adds supplied key value as data.
func (kv *Secret) AddData(key string, val string) {
	if kv.Data == nil {
		kv.Data = map[string]string{}
	}
	kv.Data[key] = val
}

// AddMetadata adds supplied key value as metadata.
func (kv *Secret) AddMetadata(key string, val string) {
	if kv.CustomMeta == nil {
		kv.CustomMeta = map[string]string{}
	}
	kv.CustomMeta[key] = val
}

// An ApplyOption is called before patching the current secret to match the
// desired secret. ApplyOptions are not called if no current object exists.
type ApplyOption func(current, desired *Secret) error

// AllowUpdateIf will only update the current object if the supplied fn returns
// true. An error that satisfies IsNotAllowed will be returned if the supplied
// function returns false. Creation of a desired object that does not currently
// exist is always allowed.
func AllowUpdateIf(fn func(current, desired *Secret) bool) ApplyOption {
	return func(current, desired *Secret) error {
		if fn(current, desired) {
			return nil
		}
		return resource.NewNotAllowed(errUpdateNotAllowed)
	}
}

// IsNotFound returns whether given error is a "Not Found" error or not.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == ErrNotFound
}
