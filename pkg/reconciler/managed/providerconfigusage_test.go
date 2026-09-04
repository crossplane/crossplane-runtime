/*
Copyright 2026 The Crossplane Authors.

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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

// anotherProviderConfigUsage is a second namespaced usage kind, so a scheme can
// register more than one of them.
type anotherProviderConfigUsage struct {
	fake.ProviderConfigUsage
}

func (p *anotherProviderConfigUsage) DeepCopyObject() runtime.Object {
	return &anotherProviderConfigUsage{}
}

func TestDefaultProviderConfigUsageCleaner(t *testing.T) {
	type args struct {
		mg     resource.Managed
		scheme *runtime.Scheme
	}

	cases := map[string]struct {
		reason string
		args   args
		want   string
	}{
		"LegacyManaged": {
			reason: "A cluster scoped managed resource gets a legacy tracker for the cluster scoped usage kind in its scheme.",
			args: args{
				mg:     &fake.LegacyManaged{},
				scheme: fake.SchemeWith(&fake.LegacyManaged{}, &fake.LegacyProviderConfigUsage{}, &fake.ProviderConfigUsage{}),
			},
			want: "*resource.LegacyProviderConfigUsageTracker",
		},
		"ModernManaged": {
			reason: "A namespaced managed resource gets a tracker for the namespaced usage kind in its scheme.",
			args: args{
				mg:     &fake.ModernManaged{},
				scheme: fake.SchemeWith(&fake.ModernManaged{}, &fake.LegacyProviderConfigUsage{}, &fake.ProviderConfigUsage{}),
			},
			want: "*resource.ProviderConfigUsageTracker",
		},
		"NoUsageKind": {
			reason: "A scheme that registers no usage kind yields a cleaner that does nothing.",
			args: args{
				mg:     &fake.ModernManaged{},
				scheme: fake.SchemeWith(&fake.ModernManaged{}),
			},
			want: "resource.ProviderConfigUsageCleanerFns",
		},
		"UsageKindOfOtherScope": {
			reason: "A usage kind of the other scope must not be used.",
			args: args{
				mg:     &fake.LegacyManaged{},
				scheme: fake.SchemeWith(&fake.LegacyManaged{}, &fake.ProviderConfigUsage{}),
			},
			want: "resource.ProviderConfigUsageCleanerFns",
		},
		"AmbiguousUsageKind": {
			reason: "A scheme that registers several usage kinds of the managed resource's scope yields a cleaner that does nothing.",
			args: args{
				mg:     &fake.ModernManaged{},
				scheme: fake.SchemeWith(&fake.ModernManaged{}, &fake.ProviderConfigUsage{}, &anotherProviderConfigUsage{}),
			},
			want: "resource.ProviderConfigUsageCleanerFns",
		},
		"UnscopedManaged": {
			reason: "A managed resource that is neither cluster scoped nor namespaced yields a cleaner that does nothing.",
			args: args{
				mg:     &fake.Managed{},
				scheme: fake.SchemeWith(&fake.Managed{}, &fake.LegacyProviderConfigUsage{}, &fake.ProviderConfigUsage{}),
			},
			want: "resource.ProviderConfigUsageCleanerFns",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &fake.Manager{Client: &test.MockClient{}, Scheme: tc.args.scheme}

			got := fmt.Sprintf("%T", defaultProviderConfigUsageCleaner(m, tc.args.mg))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\ndefaultProviderConfigUsageCleaner(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
