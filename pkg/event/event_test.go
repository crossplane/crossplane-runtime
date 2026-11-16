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

package event

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

func TestAPIRecorderPreservesLiteralMessage(t *testing.T) {
	for _, message := range []string{"100% ready", "literal %s and %d"} {
		t.Run(message, func(t *testing.T) {
			recorder := record.NewFakeRecorder(1)
			NewAPIRecorder(recorder).Event(&corev1.Pod{}, Normal("Ready", message))
			select {
			case got := <-recorder.Events:
				if diff := cmp.Diff("Normal Ready "+message, got); diff != "" {
					t.Errorf("recorded event (-want +got):\n%s", diff)
				}
			default:
				t.Fatal("no event recorded")
			}
		})
	}
}

func TestSliceMap(t *testing.T) {
	type args struct {
		from []string
		to   map[string]string
	}
	cases := map[string]struct {
		reason string
		args   args
		want   map[string]string
	}{
		"OnePair": {
			reason: "One key value pair should be added.",
			args: args{
				from: []string{"key", "val"},
				to:   map[string]string{},
			},
			want: map[string]string{"key": "val"},
		},
		"TwoPairs": {
			reason: "Two key value pairs should be added.",
			args: args{
				from: []string{
					"key", "val",
					"another", "value",
				},
				to: map[string]string{},
			},
			want: map[string]string{
				"key":     "val",
				"another": "value",
			},
		},
		"NoValue": {
			reason: "Two key value pairs should be added.",
			args: args{
				from: []string{"key"},
				to:   map[string]string{},
			},
			want: map[string]string{},
		},
		"ExtraneousKey": {
			reason: "One key value pair should be added.",
			args: args{
				from: []string{
					"key", "val",
					"extraneous",
				},
				to: map[string]string{},
			},
			want: map[string]string{"key": "val"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sliceMap(tc.args.from, tc.args.to)

			if diff := cmp.Diff(tc.want, tc.args.to); diff != "" {
				t.Errorf("%s\nsliceMap(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
