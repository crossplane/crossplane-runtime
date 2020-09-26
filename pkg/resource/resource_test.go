/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    htcp://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	namespace = "coolns"
	name      = "cool"
	uid       = types.UID("definitely-a-uuid")
)

var MockOwnerGVK = schema.GroupVersionKind{
	Group:   "cool",
	Version: "large",
	Kind:    "MockOwner",
}

func TestLocalConnectionSecretFor(t *testing.T) {
	secretName := "coolsecret"

	type args struct {
		o    LocalConnectionSecretOwner
		kind schema.GroupVersionKind
	}

	controller := true

	cases := map[string]struct {
		args args
		want *corev1.Secret
	}{
		"Success": {
			args: args{
				o: &fake.MockLocalConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						UID:       uid,
					},
					Ref: &v1alpha1.LocalSecretReference{Name: secretName},
				},
				kind: MockOwnerGVK,
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      secretName,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: MockOwnerGVK.GroupVersion().String(),
						Kind:       MockOwnerGVK.Kind,
						Name:       name,
						UID:        uid,
						Controller: &controller,
					}},
				},
				Type: SecretTypeConnection,
				Data: map[string][]byte{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := LocalConnectionSecretFor(tc.args.o, tc.args.kind)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("LocalConnectionSecretFor(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnectionSecretFor(t *testing.T) {
	secretName := "coolsecret"

	type args struct {
		o    ConnectionSecretOwner
		kind schema.GroupVersionKind
	}

	controller := true

	cases := map[string]struct {
		args args
		want *corev1.Secret
	}{
		"Success": {
			args: args{
				o: &fake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						UID:       uid,
					},
					Ref: &v1alpha1.SecretReference{Namespace: namespace, Name: secretName},
				},
				kind: MockOwnerGVK,
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      secretName,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: MockOwnerGVK.GroupVersion().String(),
						Kind:       MockOwnerGVK.Kind,
						Name:       name,
						UID:        uid,
						Controller: &controller,
					}},
				},
				Type: SecretTypeConnection,
				Data: map[string][]byte{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConnectionSecretFor(tc.args.o, tc.args.kind)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ConnectionSecretFor(): -want, +got:\n%s", diff)
			}
		})
	}
}

type MockTyper struct {
	GVKs        []schema.GroupVersionKind
	Unversioned bool
	Error       error
}

func (t MockTyper) ObjectKinds(_ runtime.Object) ([]schema.GroupVersionKind, bool, error) {
	return t.GVKs, t.Unversioned, t.Error
}

func (t MockTyper) Recognizes(_ schema.GroupVersionKind) bool { return true }

func TestGetKind(t *testing.T) {
	type args struct {
		obj runtime.Object
		ot  runtime.ObjectTyper
	}
	type want struct {
		kind schema.GroupVersionKind
		err  error
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		args args
		want want
	}{
		"KindFound": {
			args: args{
				ot: MockTyper{GVKs: []schema.GroupVersionKind{fake.GVK(&fake.Managed{})}},
			},
			want: want{
				kind: fake.GVK(&fake.Managed{}),
			},
		},
		"KindError": {
			args: args{
				ot: MockTyper{Error: errBoom},
			},
			want: want{
				err: errors.Wrap(errBoom, "cannot get kind of supplied object"),
			},
		},
		"KindIsUnversioned": {
			args: args{
				ot: MockTyper{Unversioned: true},
			},
			want: want{
				err: errors.New("supplied object is unversioned"),
			},
		},
		"NotEnoughKinds": {
			args: args{
				ot: MockTyper{},
			},
			want: want{
				err: errors.New("supplied object does not have exactly one kind"),
			},
		},
		"TooManyKinds": {
			args: args{
				ot: MockTyper{GVKs: []schema.GroupVersionKind{
					fake.GVK(&fake.Object{}),
					fake.GVK(&fake.Managed{}),
				}},
			},
			want: want{
				err: errors.New("supplied object does not have exactly one kind"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := GetKind(tc.args.obj, tc.args.ot)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("GetKind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.kind, got); diff != "" {
				t.Errorf("GetKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestMustCreateObject(t *testing.T) {
	type args struct {
		kind schema.GroupVersionKind
		oc   runtime.ObjectCreater
	}
	cases := map[string]struct {
		args args
		want runtime.Object
	}{
		"KindRegistered": {
			args: args{
				kind: fake.GVK(&fake.Managed{}),
				oc:   fake.SchemeWith(&fake.Managed{}),
			},
			want: &fake.Managed{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := MustCreateObject(tc.args.kind, tc.args.oc)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MustCreateObject(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIgnore(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		is  ErrorIs
		err error
	}
	cases := map[string]struct {
		args args
		want error
	}{
		"IgnoreError": {
			args: args{
				is:  func(err error) bool { return true },
				err: errBoom,
			},
			want: nil,
		},
		"PropagateError": {
			args: args{
				is:  func(err error) bool { return false },
				err: errBoom,
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Ignore(tc.args.is, tc.args.err)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Ignore(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestIsConditionTrue(t *testing.T) {
	cases := map[string]struct {
		c    v1alpha1.Condition
		want bool
	}{
		"IsTrue": {
			c:    v1alpha1.Condition{Status: corev1.ConditionTrue},
			want: true,
		},
		"IsFalse": {
			c:    v1alpha1.Condition{Status: corev1.ConditionFalse},
			want: false,
		},
		"IsUnknown": {
			c:    v1alpha1.Condition{Status: corev1.ConditionUnknown},
			want: false,
		},
		"IsUnset": {
			c:    v1alpha1.Condition{},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsConditionTrue(tc.c)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsConditionTrue(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type object struct {
	runtime.Object
	metav1.ObjectMeta
}

func (o *object) DeepCopyObject() runtime.Object {
	return &object{ObjectMeta: *o.ObjectMeta.DeepCopy()}
}

type nopeject struct {
	runtime.Object
}

func (o *nopeject) DeepCopyObject() runtime.Object {
	return &nopeject{}
}

func TestControllersMustMatch(t *testing.T) {
	uid := types.UID("very-unique-string")
	controller := true

	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"ControllersMatch": {
			reason: "The current and desired objects have matching controller references",
			args: args{
				current: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        uid,
					Controller: &controller,
				}}}},
				desired: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        uid,
					Controller: &controller,
				}}}},
			},
		},
		"ControllersDoNotMatch": {
			reason: "The current and desired objects do not have matching controller references",
			args: args{
				current: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        uid,
					Controller: &controller,
				}}}},
				desired: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        types.UID("some-other-uid"),
					Controller: &controller,
				}}}},
			},
			want: errors.New("existing object has a different (or no) controller"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ao := ControllersMustMatch()
			err := ao(tc.args.ctx, tc.args.current, tc.args.desired)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nControllersMustMatch(...)(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestIsNotControllable(t *testing.T) {
	cases := map[string]struct {
		reason string
		err    error
		want   bool
	}{
		"NilError": {
			reason: "A nil error does not indicate something is not controllable.",
			err:    nil,
			want:   false,
		},
		"UnknownError": {
			reason: "An that doesn't have a 'NotControllable() bool' method does not indicate something is not controllable.",
			err:    errors.New("boom"),
			want:   false,
		},
		"NotControllableError": {
			reason: "An that has a 'NotControllable() bool' method indicates something is not controllable.",
			err:    errNotControllable{errors.New("boom")},
			want:   true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsNotControllable(tc.err)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nIsNotControllable(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}

}

func TestMustBeControllableBy(t *testing.T) {
	uid := types.UID("very-unique-string")
	controller := true

	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}

	cases := map[string]struct {
		reason string
		u      types.UID
		args   args
		want   error
	}{
		"Adoptable": {
			reason: "A current object with no controller reference may be adopted and controlled",
			u:      uid,
			args: args{
				current: &object{},
			},
		},
		"ControlledBySuppliedUID": {
			reason: "A current object that is already controlled by the supplied UID is controllable",
			u:      uid,
			args: args{
				current: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        uid,
					Controller: &controller,
				}}}},
			},
		},
		"ControlledBySomeoneElse": {
			reason: "A current object that is already controlled by a different UID is not controllable",
			u:      uid,
			args: args{
				current: &object{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
					UID:        types.UID("some-other-uid"),
					Controller: &controller,
				}}}},
			},
			want: errNotControllable{errors.Errorf("existing object is not controlled by UID %q", uid)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ao := MustBeControllableBy(tc.u)
			err := ao(tc.args.ctx, tc.args.current, tc.args.desired)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMustBeControllableBy(...)(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestConnectionSecretMustBeControllableBy(t *testing.T) {
	uid := types.UID("very-unique-string")
	controller := true

	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}

	cases := map[string]struct {
		reason string
		u      types.UID
		args   args
		want   error
	}{
		"Adoptable": {
			reason: "A Secret of SecretTypeConnection with no controller reference may be adopted and controlled",
			u:      uid,
			args: args{
				current: &corev1.Secret{Type: SecretTypeConnection},
			},
		},
		"ControlledBySuppliedUID": {
			reason: "A Secret of any type that is already controlled by the supplied UID is controllable",
			u:      uid,
			args: args{
				current: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
						UID:        uid,
						Controller: &controller,
					}}},
					Type: corev1.SecretTypeOpaque,
				},
			},
		},
		"ControlledBySomeoneElse": {
			reason: "A Secret of any type that is already controlled by the another UID is not controllable",
			u:      uid,
			args: args{
				current: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
						UID:        types.UID("some-other-uid"),
						Controller: &controller,
					}}},
					Type: SecretTypeConnection,
				},
			},
			want: errNotControllable{errors.Errorf("existing secret is not controlled by UID %q", uid)},
		},
		"UncontrolledOpaqueSecret": {
			reason: "A Secret of corev1.SecretTypeOpqaue with no controller is not controllable",
			u:      uid,
			args: args{
				current: &corev1.Secret{Type: corev1.SecretTypeOpaque},
			},
			want: errNotControllable{errors.Errorf("refusing to modify uncontrolled secret of type %q", corev1.SecretTypeOpaque)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ao := ConnectionSecretMustBeControllableBy(tc.u)
			err := ao(tc.args.ctx, tc.args.current, tc.args.desired)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConnectionSecretMustBeControllableBy(...)(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestAllowUpdateIf(t *testing.T) {
	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}

	cases := map[string]struct {
		reason string
		fn     func(current, desired runtime.Object) bool
		args   args
		want   error
	}{
		"Allowed": {
			reason: "No error should be returned when the supplied function returns true",
			fn:     func(current, desired runtime.Object) bool { return true },
			args: args{
				current: &object{},
			},
		},
		"NotAllowed": {
			reason: "An error that satisfies IsNotAllowed should be returned when the supplied function returns false",
			fn:     func(current, desired runtime.Object) bool { return false },
			args: args{
				current: &object{},
			},
			want: errNotAllowed{errors.New("update not allowed")},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ao := AllowUpdateIf(tc.fn)
			err := ao(tc.args.ctx, tc.args.current, tc.args.desired)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAllowUpdateIf(...)(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestGetExternalTags(t *testing.T) {
	provName := "prov"
	cases := map[string]struct {
		o    Managed
		want map[string]string
	}{
		"Successful": {
			o: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
				ProviderReferencer: fake.ProviderReferencer{Ref: &v1alpha1.Reference{Name: provName}},
			},
			want: map[string]string{
				ExternalResourceTagKeyKind:     strings.ToLower((&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupKind().String()),
				ExternalResourceTagKeyName:     name,
				ExternalResourceTagKeyProvider: provName,
			},
		},
		"SuccessfulWithProviderConfig": {
			o: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
				ProviderConfigReferencer: fake.ProviderConfigReferencer{Ref: &v1alpha1.Reference{Name: provName}},
			},
			want: map[string]string{
				ExternalResourceTagKeyKind:     strings.ToLower((&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupKind().String()),
				ExternalResourceTagKeyName:     name,
				ExternalResourceTagKeyProvider: provName,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := GetExternalTags(tc.o)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GetExternalTags(...): -want, +got:\n%s", diff)
			}
		})
	}
}
