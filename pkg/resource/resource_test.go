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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
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

type MockLocalOwner struct {
	runtime.Object
	metav1.ObjectMeta

	Ref *v1alpha1.LocalSecretReference
}

func (m *MockLocalOwner) GetWriteConnectionSecretToReference() *v1alpha1.LocalSecretReference {
	return m.Ref
}

func (m *MockLocalOwner) SetWriteConnectionSecretToReference(r *v1alpha1.LocalSecretReference) {
	m.Ref = r
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
				o: &MockLocalOwner{
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

type MockOwner struct {
	runtime.Object
	metav1.ObjectMeta

	Ref *v1alpha1.SecretReference
}

func (m *MockOwner) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	return m.Ref
}

func (m *MockOwner) SetWriteConnectionSecretToReference(r *v1alpha1.SecretReference) {
	m.Ref = r
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
				o: &MockOwner{
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
					fake.GVK(&fake.Claim{}),
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
				kind: fake.GVK(&fake.Claim{}),
				oc:   fake.SchemeWith(&fake.Claim{}),
			},
			want: &fake.Claim{},
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

func TestResolveClassClaimValues(t *testing.T) {
	type args struct {
		classValue string
		claimValue string
	}

	type want struct {
		err   error
		value string
	}

	cases := map[string]struct {
		args
		want
	}{
		"ClassValueUnset": {
			args: args{claimValue: "cool"},
			want: want{value: "cool"},
		},
		"ClaimValueUnset": {
			args: args{classValue: "cool"},
			want: want{value: "cool"},
		},
		"IdenticalValues": {
			args: args{classValue: "cool", claimValue: "cool"},
			want: want{value: "cool"},
		},
		"ConflictingValues": {
			args: args{classValue: "lame", claimValue: "cool"},
			want: want{err: errors.New("claim value [cool] does not match class value [lame]")},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ResolveClassClaimValues(tc.args.classValue, tc.args.claimValue)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ResolveClassClaimValues(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("ResolveClassClaimValues(...): -want, +got:\n%s", diff)
			}

		})
	}
}

func TestSetBindable(t *testing.T) {
	cases := map[string]struct {
		b    Bindable
		want v1alpha1.BindingPhase
	}{
		"BindableIsUnbindable": {
			b:    &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseUnbindable}},
			want: v1alpha1.BindingPhaseUnbound,
		},
		"BindableIsUnbound": {
			b:    &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseUnbound}},
			want: v1alpha1.BindingPhaseUnbound,
		},
		"BindableIsBound": {
			b:    &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
			want: v1alpha1.BindingPhaseBound,
		},
		"BindableIsReleased": {
			b:    &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased}},
			want: v1alpha1.BindingPhaseReleased,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			SetBindable(tc.b)
			if diff := cmp.Diff(tc.want, tc.b.GetBindingPhase()); diff != "" {
				t.Errorf("SetBindable(...): -got, +want:\n%s", diff)
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

var _ Applicator = ApplyFn(Apply)

func TestApply(t *testing.T) {
	errBoom := errors.New("boom")
	named := &object{}
	named.SetName("barry")

	controlled := &object{}
	ref := metav1.NewControllerRef(named, schema.GroupVersionKind{})
	meta.AddControllerReference(controlled, *ref)

	type args struct {
		ctx context.Context
		c   client.Client
		o   runtime.Object
		ao  []ApplyOption
	}

	type want struct {
		o   runtime.Object
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotAMetadataObject": {
			reason: "An error should be returned if we can't access the object's metadata",
			args: args{
				c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				o: &nopeject{},
			},
			want: want{
				o:   &nopeject{},
				err: errors.New("cannot access object metadata"),
			},
		},
		"GetError": {
			reason: "An error should be returned if we can't get the object",
			args: args{
				c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot get object"),
			},
		},
		"CreateError": {
			reason: "No error should be returned if we successfully create a new object",
			args: args{
				c: &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(errBoom),
				},
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot create object"),
			},
		},
		"ControllerMismatch": {
			reason: "An error should be returned if controllers must match, but don't",
			args: args{
				c: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
					*(o.(*object)) = *controlled
					return nil
				})},
				o:  &object{},
				ao: []ApplyOption{ControllersMustMatch()},
			},
			want: want{
				o:   controlled,
				err: errors.New("existing object has a different (or no) controller"),
			},
		},
		"PatchError": {
			reason: "An error should be returned if we can't patch the object",
			args: args{
				c: &test.MockClient{
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(errBoom),
				},
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot patch object"),
			},
		},
		"Created": {
			reason: "No error should be returned if we successfully create a new object",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(o runtime.Object) error {
						*(o.(*object)) = *named
						return nil
					}),
				},
				o: &object{},
			},
			want: want{
				o: named,
			},
		},
		"Patched": {
			reason: "No error should be returned if we successfully patch an existing object",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil, func(o runtime.Object) error {
						*(o.(*object)) = *named
						return nil
					}),
				},
				o: &object{},
			},
			want: want{
				o: named,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := Apply(tc.args.ctx, tc.args.c, tc.args.o, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nApply(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestGetExternalTags(t *testing.T) {
	provName := "prov"
	className := "classy"
	cases := map[string]struct {
		o    Managed
		want map[string]string
	}{
		"Successful": {
			o: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
				ProviderReferencer: fake.ProviderReferencer{Ref: &corev1.ObjectReference{Name: provName}},
				ClassReferencer:    fake.ClassReferencer{Ref: &corev1.ObjectReference{Name: className}},
			},
			want: map[string]string{
				ExternalResourceTagKeyKind:     strings.ToLower((&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupKind().String()),
				ExternalResourceTagKeyName:     name,
				ExternalResourceTagKeyProvider: provName,
				ExternalResourceTagKeyClass:    className,
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
