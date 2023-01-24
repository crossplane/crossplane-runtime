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

// Package fake is a fake Vault LogicalClient.
package fake

import (
	"github.com/hashicorp/vault/api"
)

// LogicalClient is a fake LogicalClient
type LogicalClient struct {
	ReadFn   func(path string) (*api.Secret, error)
	WriteFn  func(path string, data map[string]any) (*api.Secret, error)
	DeleteFn func(path string) (*api.Secret, error)
}

// Read reads secret at the given path.
func (l *LogicalClient) Read(path string) (*api.Secret, error) {
	return l.ReadFn(path)
}

// Write writes data to the given path.
func (l *LogicalClient) Write(path string, data map[string]any) (*api.Secret, error) {
	return l.WriteFn(path, data)
}

// Delete deletes secret at the given path.
func (l *LogicalClient) Delete(path string) (*api.Secret, error) {
	return l.DeleteFn(path)
}
