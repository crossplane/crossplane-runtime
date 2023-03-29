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

package integration

import (
	"context"
	"os"
	"time"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	// Allow auth to cloud providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	syncPeriod      = "30s"
	errCleanup      = "failure in default cleanup"
	errCreateTmpDir = "unable to create temporary directory for CRDs"
	errGetCRDs      = "unable to download CRDs"
)

// BuilderFn is a function that performs operations prior to starting test controllers
type BuilderFn func(client.Client) error

// CleanerFn is a function that performs test cleanup
type CleanerFn func(*Manager) error

// Config is a set of configuration values for setup.
type Config struct {
	CRDPaths       []string
	Builder        BuilderFn
	Cleaners       []CleanerFn
	ManagerOptions manager.Options
}

// NewBuilder returns a new no-op Builder
func NewBuilder() BuilderFn {
	return func(client.Client) error {
		return nil
	}
}

// NewCRDCleaner returns a new Cleaner that deletes all installed CRDs from the
// API server.
func NewCRDCleaner() CleanerFn {
	return func(m *Manager) error {
		var crds []*apiextensionsv1beta1.CustomResourceDefinition
		for _, path := range m.env.CRDDirectoryPaths {
			crd, err := readCRDs(path)
			if err != nil {
				return errors.Wrap(err, errCleanup)
			}
			crds = append(crds, crd...)
		}

		deletionPolicy := metav1.DeletePropagationForeground
		for _, crd := range crds {
			if err := m.client.Delete(context.TODO(), crd, &client.DeleteOptions{PropagationPolicy: &deletionPolicy}); resource.IgnoreNotFound(err) != nil {
				return errors.Wrap(err, errCleanup)

			}
		}
		return nil
	}
}

// NewCRDDirCleaner cleans up the tmp directory where CRDs were
// downloaded.
func NewCRDDirCleaner() CleanerFn {
	return func(m *Manager) error {
		return os.RemoveAll(m.tmpDir)
	}
}

// An Option configures a Config.
type Option func(*Config)

// WithBuilder sets a custom builder function for a Config.
func WithBuilder(builder BuilderFn) Option {
	return func(c *Config) {
		c.Builder = builder
	}
}

// WithCleaners sets custom cleaner functios for a Config.
func WithCleaners(cleaners ...CleanerFn) Option {
	return func(c *Config) {
		c.Cleaners = cleaners
	}
}

// WithCRDPaths sets custom CRD locations for a Config.
func WithCRDPaths(crds ...string) Option {
	return func(c *Config) {
		c.CRDPaths = crds
	}
}

// WithManagerOptions sets custom options for the manager configured by
// Config.
func WithManagerOptions(m manager.Options) Option {
	return func(c *Config) {
		c.ManagerOptions = m
	}
}

func defaultConfig() *Config {
	t, err := time.ParseDuration(syncPeriod)
	if err != nil {
		panic(err)
	}

	return &Config{
		CRDPaths:       []string{},
		Builder:        NewBuilder(),
		Cleaners:       []CleanerFn{NewCRDDirCleaner(), NewCRDCleaner()},
		ManagerOptions: manager.Options{SyncPeriod: &t},
	}
}

// Manager wraps a controller-runtime manager with additional functionality.
type Manager struct {
	manager.Manager
	stop   chan struct{}
	env    *envtest.Environment
	client client.Client
	c      *Config
	tmpDir string
}

// New creates a new Manager.
func New(cfg *rest.Config, o ...Option) (*Manager, error) {
	var useExisting bool
	if cfg != nil {
		useExisting = true
	}

	c := defaultConfig()
	for _, op := range o {
		op(c)
	}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, errors.Wrap(err, errCreateTmpDir)
	}

	crdPaths := []string{}
	for _, path := range c.CRDPaths {
		dst, err := downloadPath(path, dir)
		if err != nil {
			return nil, errors.Wrap(err, errGetCRDs)
		}
		crdPaths = append(crdPaths, dst)
	}

	e := &envtest.Environment{
		CRDDirectoryPaths:  crdPaths,
		Config:             cfg,
		UseExistingCluster: &useExisting,
	}

	cfg, err = e.Start()
	if err != nil {
		return nil, err
	}

	client, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, err
	}

	if err := c.Builder(client); err != nil {
		return nil, err
	}

	mgr, err := manager.New(cfg, c.ManagerOptions)
	if err != nil {
		return nil, err
	}

	stop := make(chan struct{})
	return &Manager{mgr, stop, e, client, c, dir}, nil
}

// Run starts a controller-runtime manager with a signal channel.
func (m *Manager) Run() {
	go func() {
		if err := m.Start(context.Background()); err != nil {
			panic(err)
		}
	}()
}

// GetClient returns a Kubernetes rest client.
func (m *Manager) GetClient() client.Client {
	return m.client
}

// Cleanup runs the supplied cleanup or defaults to deleting all CRDs.
func (m *Manager) Cleanup() error {
	close(m.stop)
	for _, clean := range m.c.Cleaners {
		if err := clean(m); err != nil {
			return err
		}
	}

	return m.env.Stop()
}
