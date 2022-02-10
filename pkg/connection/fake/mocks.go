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

package fake

import xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

// ConnectionDetailsPublisherTo is a mock that implements ConnectionDetailsPublisherTo interface.
type ConnectionDetailsPublisherTo struct {
	To *xpv1.PublishConnectionDetailsTo
}

// SetPublishConnectionDetailsTo sets the PublishConnectionDetailsTo.
func (m *ConnectionDetailsPublisherTo) SetPublishConnectionDetailsTo(t *xpv1.PublishConnectionDetailsTo) {
	m.To = t
}

// GetPublishConnectionDetailsTo gets the PublishConnectionDetailsTo.
func (m *ConnectionDetailsPublisherTo) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	return m.To
}
