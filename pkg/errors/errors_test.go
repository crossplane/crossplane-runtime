/*
Copyright 2021 The Crossplane Authors.

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

package errors

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestWrap(t *testing.T) {
	errBoom := New("boom")

	type args struct {
		err     error
		message string
	}
	cases := map[string]struct {
		args args
		want error
	}{
		"NilError": {
			args: args{
				err:     nil,
				message: "very useful context",
			},
			want: nil,
		},
		"NonNilError": {
			args: args{
				err:     errBoom,
				message: "very useful context",
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Wrap(tc.args.err, tc.args.message)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Wrap(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	errBoom := New("boom")

	type args struct {
		err     error
		message string
		args    []any
	}
	cases := map[string]struct {
		args args
		want error
	}{
		"NilError": {
			args: args{
				err:     nil,
				message: "very useful context",
			},
			want: nil,
		},
		"NonNilError": {
			args: args{
				err:     errBoom,
				message: "very useful context about %s",
				args:    []any{"ducks"},
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Wrapf(tc.args.err, tc.args.message, tc.args.args...)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Wrapf(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCause(t *testing.T) {
	errBoom := New("boom")

	cases := map[string]struct {
		err  error
		want error
	}{
		"NilError": {
			err:  nil,
			want: nil,
		},
		"BareError": {
			err:  errBoom,
			want: errBoom,
		},
		"WrappedError": {
			err:  Wrap(Wrap(errBoom, "interstitial context"), "very important context"),
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Cause(tc.err)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Cause(...): -want, +got:\n%s", diff)
			}
		})
	}
}
