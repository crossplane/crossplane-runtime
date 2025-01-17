/*
Copyright 2019 The Crossplane Authors.

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

package v1

import "maps"

// See https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

// A Status reflects the observed status of a resource.
type Status map[string]interface{}

// GetStatus returns the status.
func (s *Status) GetStatus() Status {
	return *s
}

// SetConditions sets the supplied status on the resource.
func (s *Status) SetStatus(c Status) {
	*s = maps.Clone(c)
}
