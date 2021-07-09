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

package fieldpath

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	k8s "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type mergoOptArr []func(*mergo.Config)

func (arr mergoOptArr) names() []string {
	names := make([]string, len(arr))
	for i, opt := range arr {
		names[i] = runtime.FuncForPC(reflect.ValueOf(opt).Pointer()).Name()
	}
	sort.Strings(names)
	return names
}

func TestMergoConfiguration(t *testing.T) {
	tests := map[string]struct {
		mo   *MergeOptions
		want mergoOptArr
	}{
		"DefaultOptionsNil": {
			want: mergoOptArr{
				mergo.WithOverride,
			},
		},
		"DefaultOptionsEmptyStruct": {
			mo: &MergeOptions{},
			want: mergoOptArr{
				mergo.WithOverride,
			},
		},
		"MapKeepOnly": {
			mo: &MergeOptions{
				KeepMapValues: true,
			},
			want: mergoOptArr{},
		},
		"AppendSliceOnly": {
			mo: &MergeOptions{
				AppendSlice: true,
			},
			want: mergoOptArr{
				mergo.WithAppendSlice,
				mergo.WithOverride,
			},
		},
		"MapKeepAppendSlice": {
			mo: &MergeOptions{
				AppendSlice:   true,
				KeepMapValues: true,
			},
			want: mergoOptArr{
				mergo.WithAppendSlice,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want.names(), mergoOptArr(tc.mo.MergoConfiguration()).names()); diff != "" {
				t.Errorf("\nmo.MergoConfiguration(): -want, +got:\n %s", diff)
			}

		})
	}
}

type objWithUnstructuredContent struct {
	m map[string]interface{}
}

func (objWithUnstructuredContent) GetObjectKind() schema.ObjectKind {
	return nil
}

func (objWithUnstructuredContent) DeepCopyObject() k8s.Object {
	return nil
}

func (o objWithUnstructuredContent) UnstructuredContent() map[string]interface{} {
	return o.m
}

func pavedComparer(p1, p2 Paved) bool {
	return reflect.DeepEqual(p1, p2)
}

func TestToPaved(t *testing.T) {
	type args struct {
		o k8s.Object
	}
	type want struct {
		paved  *Paved
		copied bool
		err    error
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProvidesUnstructuredContent": {
			reason: "If object already provides UnstructuredContent, it should be used to pave it, with no copying of contents",
			args: args{
				o: objWithUnstructuredContent{
					m: map[string]interface{}{
						"key": "val",
					},
				},
			},
			want: want{
				paved: &Paved{
					object: map[string]interface{}{
						"key": "val",
					},
				},
			},
		},
		"NoUnstructuredContent": {
			reason: "If object does not provide UnstructuredContent, unstructured converter will be used to pave it, with its contents being copied",
			args: args{
				o: &v1.ConfigMap{},
			},
			want: want{
				copied: true,
				paved: &Paved{
					object: map[string]interface{}{
						"metadata": map[string]interface{}{"creationTimestamp": nil},
					},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotPaved, gotCopied, err := ToPaved(tc.args.o)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\nToPaved(...) unexpected error: %s: -want error, +got error:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.want.paved, gotPaved, cmp.Comparer(pavedComparer)); diff != "" {
				t.Errorf("\nToPaved(...) unexpected paved: %s: -want paved, +got paved:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.copied, gotCopied); diff != "" {
				t.Errorf("\nToPaved(...) unexpected copied: %s: -want copied, +got copied:\n%s", tc.reason, diff)
			}
		})
	}
}

type object struct {
	P1 *p1
}
type p1 struct {
	P2 *p2
	S  *string
}
type p2 struct {
	S *string
	B *bool
	A []string
}

func (object) GetObjectKind() schema.ObjectKind {
	return nil
}

func (object) DeepCopyObject() k8s.Object {
	return nil
}

type strObject string

func (strObject) GetObjectKind() schema.ObjectKind {
	return nil
}

func (strObject) DeepCopyObject() k8s.Object {
	return nil
}

var (
	valStringDst   = "value-from-dst"
	valStringSrc   = "value-from-src"
	valBoolTrue    = true
	valBoolFalse   = false
	valArrDst      = []string{valStringDst}
	valArrSrc      = []string{valStringSrc}
	valArrAppended = []string{valStringDst, valStringSrc}
)

func dstObject() *object {
	return &object{
		P1: &p1{
			S: &valStringDst,
			P2: &p2{
				S: &valStringDst,
				B: &valBoolTrue,
				A: valArrDst,
			},
		},
	}
}

func srcObject() *object {
	return &object{
		P1: &p1{
			S: &valStringSrc,
			P2: &p2{
				S: &valStringSrc,
				B: &valBoolFalse,
				A: valArrSrc,
			},
		},
	}
}

func TestMergePath(t *testing.T) {
	type args struct {
		fieldPath    string
		dst          k8s.Object
		src          k8s.Object
		mergeOptions *MergeOptions
	}
	type want struct {
		err error
		dst k8s.Object
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ReplacePath": {
			reason: "Default behavior if no merge options are supplied is to replace dst with src",
			args: args{
				fieldPath:    "p1.p2",
				dst:          dstObject(),
				src:          srcObject(),
				mergeOptions: nil,
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringSrc,
							B: &valBoolFalse,
							A: valArrSrc,
						},
					},
				},
			},
		},
		"MergePathNoSliceAppend": {
			reason: "When KeepMapValues is set but AppendSlice is not, dst should preserve its values at the merge path",
			args: args{
				fieldPath: "p1.p2",
				dst:       dstObject(),
				src:       srcObject(),
				mergeOptions: &MergeOptions{
					KeepMapValues: true,
				},
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringDst,
							B: &valBoolTrue,
							A: valArrDst,
						},
					},
				},
			},
		},
		"MergePathWithSliceAppend": {
			reason: "When both KeepMapValues and AppendSlice are ser, dst should preserve map values but arrays being appended",
			args: args{
				fieldPath: "p1.p2",
				dst:       dstObject(),
				src:       srcObject(),
				mergeOptions: &MergeOptions{
					KeepMapValues: true,
					AppendSlice:   true,
				},
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringDst,
							B: &valBoolTrue,
							A: valArrAppended,
						},
					},
				},
			},
		},
		"PathNotFound": {
			reason: "If specified merge path does not exist, dst should be unmodified even if replace is requested (empty src value is merged onto dst value)",
			args: args{
				fieldPath: "p1.non.existent",
				dst:       dstObject(),
				src:       srcObject(),
			},
			want: want{
				dst: dstObject(),
			},
		},
		"SrcValueEmpty": {
			reason: "If value at the specified merge path is zero in src, dst should be unmodified, even if replace is requested",
			args: args{
				fieldPath: "p1.p2",
				dst:       dstObject(),
				src: &object{
					P1: &p1{
						S: &valStringSrc,
					},
				},
			},
			want: want{
				dst: dstObject(),
			},
		},
		"DstValueEmpty": {
			reason: "If value at the specified merge path is zero in dst but not in src, should be identical to a replace, even if merge is configured",
			args: args{
				fieldPath: "p1.p2",
				src:       srcObject(),
				dst: &object{
					P1: &p1{
						S: &valStringDst,
					},
				},
				mergeOptions: &MergeOptions{
					KeepMapValues: true,
					AppendSlice:   true,
				},
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringSrc,
							B: &valBoolFalse,
							A: valArrSrc,
						},
					},
				},
			},
		},
		"ErrSrcNotPaved": {
			reason: "If src cannot be paved, MergePath should be failing",
			args: args{
				dst: dstObject(),
				src: strObject("src"),
			},
			want: want{
				err: fmt.Errorf("ToUnstructured requires a non-nil pointer to an object, got fieldpath.strObject"),
			},
		},
		"ErrDstNotPaved": {
			reason: "If dst cannot be paved, MergePath should be failing",
			args: args{
				fieldPath: "p1.p2",
				dst:       strObject("dst"),
				src:       srcObject(),
			},
			want: want{
				err: fmt.Errorf("ToUnstructured requires a non-nil pointer to an object, got fieldpath.strObject"),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := MergePath(tc.args.fieldPath, tc.args.dst, tc.args.src, tc.args.mergeOptions)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\nMergePath(...) unexpected error: %s: -want error, +got error:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.want.dst, tc.args.dst); diff != "" {
				t.Errorf("\nMergePath(...) unexpected dst: %s: -want dst, +got dst:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMergeReplace(t *testing.T) {
	type args struct {
		fieldPath    string
		current      k8s.Object
		desired      k8s.Object
		mergeOptions *MergeOptions
	}
	type want struct {
		current k8s.Object
		desired k8s.Object
		err     error
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"HappyPath": {
			args: args{
				fieldPath: "data",
				current: &v1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-current",
					},
				},
				desired: &v1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-desired",
						"key2": "value-from-desired",
					},
				},
				mergeOptions: &MergeOptions{
					KeepMapValues: true,
				},
			},
			want: want{
				current: &v1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-current",
					},
				},
				desired: &v1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-current",
						"key2": "value-from-desired",
					},
				},
			},
		},
		"ErrFromMergePath": {
			args: args{
				fieldPath: "data",
				current:   strObject("current"),
				desired:   strObject("desired"),
			},
			want: want{
				err: fmt.Errorf("ToUnstructured requires a non-nil pointer to an object, got fieldpath.strObject"),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := MergeReplace(tc.args.fieldPath, tc.args.current, tc.args.desired, tc.args.mergeOptions)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\nMergeReplace(...) unexpected error: -want error, +got error:\n%s", diff)
			}
			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.want.current, tc.args.current); diff != "" {
				t.Errorf("\nMergeReplace(...) unexpected current: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.desired, tc.args.desired); diff != "" {
				t.Errorf("\nMergeReplace(...) unexpected desired: -want, +got:\n%s", diff)
			}
		})
	}
}
