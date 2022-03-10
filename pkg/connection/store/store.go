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

package store

import (
	"context"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// KeyValues is a map with sensitive values.
type KeyValues map[string][]byte

// ScopedName is scoped name of a secret.
type ScopedName struct {
	Name  string
	Scope string
}

// A Secret is an entity representing a set of sensitive Key Values.
type Secret struct {
	ScopedName
	Metadata *v1.ConnectionSecretMetadata
	Data     KeyValues
}

// An WriteOption is called before writing the desired secret over the
// current object.
type WriteOption func(ctx context.Context, current, desired *Secret) error
