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
	"compress/gzip"
	"io"
	"os"
	"sync"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

const (
	errGetNopCache = "cannot get content from a NopCache"
)

const cacheContentExt = ".gz"

// A PackageCache caches package content.
type PackageCache interface {
	Has(id string) bool
	// Get returns cached package content. The caller must close the returned reader.
	Get(id string) (io.ReadCloser, error)
	Store(id string, content io.ReadCloser) error
	Delete(id string) error
}

// FsPackageCache stores and retrieves package content in a filesystem-backed
// cache in a thread-safe manner.
type FsPackageCache struct {
	dir string
	fs  afero.Fs
	mu  sync.RWMutex
}

type unlockingReadCloser struct {
	io.ReadCloser

	once   sync.Once
	err    error
	unlock func()
}

func (r *unlockingReadCloser) Close() error {
	r.once.Do(func() {
		r.err = r.ReadCloser.Close()
		r.unlock()
	})

	return r.err
}

// NewFsPackageCache creates a new FsPackageCache.
func NewFsPackageCache(dir string, fs afero.Fs) *FsPackageCache {
	return &FsPackageCache{
		dir: dir,
		fs:  fs,
	}
}

// Has indicates whether an item with the given id is in the cache.
func (c *FsPackageCache) Has(id string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if fi, err := c.fs.Stat(BuildPath(c.dir, id, cacheContentExt)); err == nil && !fi.IsDir() {
		return true
	}

	return false
}

// Get retrieves package contents from the cache. It holds a read lock until the
// returned reader is closed.
func (c *FsPackageCache) Get(id string) (io.ReadCloser, error) {
	c.mu.RLock()

	f, err := c.fs.Open(BuildPath(c.dir, id, cacheContentExt))
	if err != nil {
		c.mu.RUnlock()
		return nil, err
	}

	r, err := GzipReadCloser(f)
	if err != nil {
		_ = f.Close()
		c.mu.RUnlock()
		return nil, err
	}

	return &unlockingReadCloser{
		ReadCloser: r,
		unlock:     c.mu.RUnlock,
	}, nil
}

// Store saves the package contents to the cache.
func (c *FsPackageCache) Store(id string, content io.ReadCloser) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := BuildPath(c.dir, id, cacheContentExt)
	cf, err := c.fs.Create(path)
	if err != nil {
		return err
	}
	cleanup := func() {
		_ = cf.Close()
		_ = c.fs.Remove(path)
	}

	w, err := gzip.NewWriterLevel(cf, gzip.BestSpeed)
	if err != nil {
		cleanup()
		return err
	}

	_, err = io.Copy(w, content)
	if err != nil {
		_ = w.Close()
		cleanup()
		return err
	}
	// NOTE(hasheddan): gzip writer must be closed to ensure all data is flushed
	// to file.
	if err := w.Close(); err != nil {
		cleanup()
		return err
	}

	if err := cf.Close(); err != nil {
		cleanup()
		return err
	}

	return nil
}

// Delete removes package contents from the cache.
func (c *FsPackageCache) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.fs.Remove(BuildPath(c.dir, id, cacheContentExt))
	if os.IsNotExist(err) {
		return nil
	}

	return err
}

// NopCache is a cache implementation that does not store anything and always
// returns an error on get.
type NopCache struct{}

// NewNopCache creates a new NopCache.
func NewNopCache() *NopCache {
	return &NopCache{}
}

// Has indicates whether content is in the NopCache.
func (c *NopCache) Has(string) bool {
	return false
}

// Get retrieves content from the NopCache.
func (c *NopCache) Get(string) (io.ReadCloser, error) {
	return nil, errors.New(errGetNopCache)
}

// Store saves content to the NopCache.
func (c *NopCache) Store(string, io.ReadCloser) error {
	return nil
}

// Delete removes content from the NopCache.
func (c *NopCache) Delete(string) error {
	return nil
}
