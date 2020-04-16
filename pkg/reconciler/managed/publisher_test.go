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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ ConnectionPublisher = &APISecretPublisher{}
	_ ConnectionPublisher = PublisherChain{}
)

func TestPublisherChain(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
		c   ConnectionDetails
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		p    ConnectionPublisher
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
				ConnectionPublisherFns{
					PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) error {
						return nil
					},
					UnpublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) error {
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
				ConnectionPublisherFns{
					PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) error {
						return errBoom
					},
					UnpublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) error {
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
