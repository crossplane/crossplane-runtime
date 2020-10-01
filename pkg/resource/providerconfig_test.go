/*
Copyright 2020 The Crossplane Authors.

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
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestTrack(t *testing.T) {
	errBoom := errors.New("boom")
	name := "provisional"

	type fields struct {
		c  Applicator
		of ProviderConfigUsage
	}

	type args struct {
		ctx context.Context
		mg  Managed
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   error
	}{
		"MissingRef": {
			reason: "An error that satisfies IsMissingReference should be returned if the managed resource has no provider config reference",
			fields: fields{
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.Managed{},
			},
			want: errMissingRef{errors.New(errMissingPCRef)},
		},
		"NopUpdate": {
			reason: "No error should be returned if the apply fails because it would be a no-op",
			fields: fields{
				c: ApplyFn(func(c context.Context, r runtime.Object, ao ...ApplyOption) error {
					for _, fn := range ao {
						// Exercise the MustBeControllableBy and AllowUpdateIf
						// ApplyOptions. The former should pass because the
						// current object has no controller ref. The latter
						// should return an error that satisfies IsNotAllowed
						// because the current object has the same PC ref as the
						// new one we would apply.
						current := &fake.ProviderConfigUsage{
							RequiredProviderConfigReferencer: fake.RequiredProviderConfigReferencer{
								Ref: v1alpha1.Reference{Name: name},
							},
						}
						if err := fn(context.TODO(), current, nil); err != nil {
							return err
						}
					}
					return errBoom
				}),
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.Managed{
					ProviderConfigReferencer: fake.ProviderConfigReferencer{
						Ref: &v1alpha1.Reference{Name: name},
					},
				},
			},
			want: nil,
		},
		"ApplyError": {
			reason: "Errors applying the ProviderConfigUsage should be returned",
			fields: fields{
				c: ApplyFn(func(c context.Context, r runtime.Object, ao ...ApplyOption) error {
					return errBoom
				}),
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.Managed{
					ProviderConfigReferencer: fake.ProviderConfigReferencer{
						Ref: &v1alpha1.Reference{Name: name},
					},
				},
			},
			want: errors.Wrap(errBoom, errApplyPCU),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ut := &UsageTracker{c: tc.fields.c, of: tc.fields.of}
			got := ut.Track(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nut.Track(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
		})
	}
}
