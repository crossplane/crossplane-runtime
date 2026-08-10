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

package xpkg

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

var _ PackageCache = &FsPackageCache{}

type errorFile struct {
	afero.File
	writeErr error
	closeErr error
}

func (f *errorFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}

	return f.File.Write(p)
}

func (f *errorFile) Close() error {
	err := f.File.Close()
	if f.closeErr != nil {
		return f.closeErr
	}

	return err
}

type errorFs struct {
	afero.Fs
	writeErr error
	closeErr error
}

func (f *errorFs) Create(name string) (afero.File, error) {
	file, err := f.Fs.Create(name)
	if err != nil {
		return nil, err
	}

	return &errorFile{File: file, writeErr: f.writeErr, closeErr: f.closeErr}, nil
}

func TestHas(t *testing.T) {
	fs := afero.NewMemMapFs()
	cf, _ := fs.Create("/cache/exists.gz")
	_ = fs.Mkdir("/cache/some-dir.gz", os.ModeDir)

	defer cf.Close()

	type args struct {
		cache PackageCache
		id    string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"Success": {
			reason: "Should not return an error if package exists at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "exists",
			},
			want: true,
		},
		"ErrNotExist": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-exist",
			},
			want: false,
		},
		"ErrIsDir": {
			reason: "Should return error if path is a directory.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "some-dir.gz",
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := tc.args.cache.Has(tc.args.id)

			if diff := cmp.Diff(tc.want, h); diff != "" {
				t.Errorf("\n%s\nHas(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGet(t *testing.T) {
	fs := afero.NewMemMapFs()
	cf, _ := fs.Create("/cache/exists.gz")
	// NOTE(hasheddan): valid gzip header.
	cf.Write([]byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 0})
	cf, _ = fs.Create("/cache/not-gzip.gz")

	cf.WriteString("some content")
	defer cf.Close()

	type args struct {
		cache PackageCache
		id    string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package exists at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "exists",
			},
		},
		"ErrNotGzip": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-gzip",
			},
			want: gzip.ErrHeader,
		},
		"ErrNotExist": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-exist",
			},
			want: &os.PathError{Op: "open", Path: "/cache/not-exist.gz", Err: afero.ErrFileNotFound},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.args.cache.Get(tc.args.id)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStore(t *testing.T) {
	fs := afero.NewMemMapFs()

	type args struct {
		cache PackageCache
		id    string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "exists-1234567",
			},
		},
		"ErrFailedCreate": {
			reason: "Should return an error if file creation fails.",
			args: args{
				cache: NewFsPackageCache("/cache", afero.NewReadOnlyFs(fs)),
				id:    "exists-1234567",
			},
			want: syscall.EPERM,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cache.Store(tc.args.id, io.NopCloser(new(bytes.Buffer)))
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStoreRoundTrip(t *testing.T) {
	cache := NewFsPackageCache("/cache", afero.NewMemMapFs())
	want := "package content"

	if err := cache.Store("package", io.NopCloser(strings.NewReader(want))); err != nil {
		t.Fatalf("Store(...): unexpected error: %v", err)
	}

	r, err := cache.Get("package")
	if err != nil {
		t.Fatalf("Get(...): unexpected error: %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Read(...): unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("Store(...): -want content, +got content:\n%s", diff)
	}
}

func TestStoreRemovesFailedWrites(t *testing.T) {
	errWrite := errors.New("write failed")
	cases := map[string]struct {
		fs      afero.Fs
		content io.ReadCloser
	}{
		"ContentReadError": {
			fs:      afero.NewMemMapFs(),
			content: io.NopCloser(io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errWrite))),
		},
		"GzipCloseError": {
			fs:      &errorFs{Fs: afero.NewMemMapFs(), writeErr: errWrite},
			content: io.NopCloser(strings.NewReader("")),
		},
		"FileCloseError": {
			fs:      &errorFs{Fs: afero.NewMemMapFs(), closeErr: errWrite},
			content: io.NopCloser(strings.NewReader("content")),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cache := NewFsPackageCache("/cache", tc.fs)
			if err := cache.Store("package", tc.content); err == nil {
				t.Fatal("Store(...): expected an error")
			}
			if cache.Has("package") {
				t.Fatal("Store(...): failed write remained in cache")
			}
		})
	}
}

func TestStoreWaitsForReader(t *testing.T) {
	cache := NewFsPackageCache("/cache", afero.NewMemMapFs())
	if err := cache.Store("package", io.NopCloser(strings.NewReader("old"))); err != nil {
		t.Fatalf("Store(...): unexpected error: %v", err)
	}

	r, err := cache.Get("package")
	if err != nil {
		t.Fatalf("Get(...): unexpected error: %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- cache.Store("package", io.NopCloser(strings.NewReader("new")))
	}()
	<-started

	select {
	case err := <-done:
		t.Fatalf("Store(...) completed while cache reader was open: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Read(...): unexpected error: %v", err)
	}
	if diff := cmp.Diff("old", string(got)); diff != "" {
		t.Errorf("Get(...): -want content, +got content:\n%s", diff)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close(...): unexpected error: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Store(...): unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Store(...) did not complete after cache reader closed")
	}
}

func TestGetErrorReleasesLock(t *testing.T) {
	fs := afero.NewMemMapFs()
	f, err := fs.Create("/cache/package.gz")
	if err != nil {
		t.Fatalf("Create(...): unexpected error: %v", err)
	}
	if _, err := f.WriteString("not gzip"); err != nil {
		t.Fatalf("WriteString(...): unexpected error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(...): unexpected error: %v", err)
	}

	cache := NewFsPackageCache("/cache", fs)
	if _, err := cache.Get("package"); err == nil {
		t.Fatal("Get(...): expected an error")
	}

	done := make(chan error, 1)
	go func() {
		done <- cache.Delete("package")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Delete(...): unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Delete(...) blocked after Get(...) failed")
	}
}

func TestDelete(t *testing.T) {
	fs := afero.NewMemMapFs()
	_, _ = fs.Create("/cache/exists.xpkg")

	type args struct {
		cache PackageCache
		id    string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package is deleted at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "exists",
			},
		},
		"SuccessNotExist": {
			reason: "Should not return an error if package does not exist.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-exist",
			},
		},
		"ErrFailedDelete": {
			reason: "Should return an error if file deletion fails.",
			args: args{
				cache: NewFsPackageCache("/cache", afero.NewReadOnlyFs(fs)),
				id:    "exists-1234567",
			},
			want: syscall.EPERM,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cache.Delete(tc.args.id)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
