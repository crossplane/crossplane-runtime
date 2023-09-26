// Copyright 2023 Upbound Inc.
// All rights reserved

package errors

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestSilentlyRequeueOnConflict(t *testing.T) {
	type args struct {
		result reconcile.Result
		err    error
	}
	type want struct {
		result reconcile.Result
		err    error
	}
	tests := []struct {
		reason string
		args   args
		want   want
	}{
		{
			reason: "nil error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: time.Second},
			},
		},
		{
			reason: "other error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
				err:    New("some other error"),
			},
			want: want{
				result: reconcile.Result{RequeueAfter: time.Second},
				err:    New("some other error"),
			},
		},
		{
			reason: "conflict error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
				err:    kerrors.NewConflict(schema.GroupResource{Group: "nature", Resource: "stones"}, "foo", New("nested error")),
			},
			want: want{
				result: reconcile.Result{Requeue: true},
			},
		},
		{
			reason: "nested conflict error",
			args: args{
				result: reconcile.Result{RequeueAfter: time.Second},
				err: Wrap(
					kerrors.NewConflict(schema.GroupResource{Group: "nature", Resource: "stones"}, "foo", New("nested error")),
					"outer error"),
			},
			want: want{
				result: reconcile.Result{Requeue: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got, err := SilentlyRequeueOnConflict(tt.args.result, tt.args.err)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIgnoreConflict(...): -want error, +got error:\n%s", tt.reason, diff)
			}
			if diff := cmp.Diff(tt.want.result, got); diff != "" {
				t.Errorf("\n%s\nIgnoreConflict(...): -want result, +got result:\n%s", tt.reason, diff)
			}
		})
	}
}
