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

// Package logging provides a logger that satisfies https://github.com/go-logr/logr.
// It is implemented as a light wrapper around sigs.k8s.io/controller-runtime/pkg/log
package logging

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	runtimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// Logging levels.
const (
	Debug = 1
)

var (
	logger = runtimelog.Log

	// Logger is the base logger used by Crossplane. It delegates to another
	// logr.Logger. You *must* call SetLogger to get any actual logging.
	Logger = logger.WithName("crossplane")

	// ZapLogger is a Logger implementation.
	ZapLogger = zapLogger
)

// ZapLogger is a Logger implementation from controller runtime.
// If development is true, a Zap development config will be used
// (stacktraces on warnings, no sampling), otherwise a Zap production
// config will be used (stacktraces on errors, sampling).
// If disableStacktrace is true, stacktraces enabled on fatals
// independent of config.
func zapLogger(development bool, disableStacktrace bool) logr.Logger {
	zl := runtimezap.RawLoggerTo(os.Stderr, development)

	if disableStacktrace {
		zl = zl.WithOptions(zap.AddStacktrace(zap.FatalLevel))
	}

	return zapr.NewLogger(zl)
}

// SetLogger sets a concrete logging implementation for all deferred Loggers.
func SetLogger(l logr.Logger) {
	logger.Fulfill(l)
}
