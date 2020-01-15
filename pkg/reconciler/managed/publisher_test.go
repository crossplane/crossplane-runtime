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

package resource

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var (
	_ ManagedConnectionPublisher = &APISecretPublisher{}
	_ ManagedConnectionPublisher = PublisherChain{}
)

func TestPublisherChain(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
		c   ConnectionDetails
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		p    ManagedConnectionPublisher
		args args
		want error
	}{
		"EmptyChain": {
			p: PublisherChain{},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
				c:   ConnectionDetails{},
			},
			want: nil,
		},
		"SuccessfulPublisher": {
			p: PublisherChain{
				ManagedConnectionPublisherFns{
					PublishConnectionFn: func(_ context.Context, mg resource.Managed, c ConnectionDetails) error {
						return nil
					},
					UnpublishConnectionFn: func(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
				c:   ConnectionDetails{},
			},
			want: nil,
		},
		"PublisherReturnsError": {
			p: PublisherChain{
				ManagedConnectionPublisherFns{
					PublishConnectionFn: func(_ context.Context, mg resource.Managed, c ConnectionDetails) error {
						return errBoom
					},
					UnpublishConnectionFn: func(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
				c:   ConnectionDetails{},
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.p.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Publish(...): -want, +got:\n%s", diff)
			}
		})
	}
}
