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

package pprof

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Server represents a HTTP server to serve Memory Profiling.
type Server interface {
	// Start memory profiling server
	Start()

	// Stop memory profiling server
	Stop()
}

const (
	port = 9090
)

type profilingServer struct {
	log    logging.Logger
	server *http.Server
}

// NewServer creates a new HTTP server on port 9090 for Memory Profiling.
func NewServer(log logging.Logger) Server {
	mux := http.NewServeMux()

	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))

	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
		Handler:        mux,
	}

	return &profilingServer{log: log, server: server}
}

func (p *profilingServer) Start() {
	go func() {
		p.log.Debug("starting memory profiling server")
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.log.Debug("error starting memory profiling server", err)
		}
	}()
}

func (p *profilingServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p.log.Debug("shutting down memory profiling server")
	if err := p.server.Shutdown(ctx); err != nil {
		p.log.Debug("error shutting down memory profiling server", err)
	}
}
