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

package managed

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// A PublisherChain chains multiple ManagedPublishers.
type PublisherChain []ConnectionPublisher

// PublishConnection calls each ConnectionPublisher.PublishConnection serially. It returns the first error it
// encounters, if any.
func (pc PublisherChain) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) error {
	for _, p := range pc {
		if err := p.PublishConnection(ctx, o, c); err != nil {
			return err
		}
	}
	return nil
}

// UnpublishConnection calls each ConnectionPublisher.UnpublishConnection serially. It returns the first error it
// encounters, if any.
func (pc PublisherChain) UnpublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) error {
	for _, p := range pc {
		if err := p.UnpublishConnection(ctx, o, c); err != nil {
			return err
		}
	}
	return nil
}
