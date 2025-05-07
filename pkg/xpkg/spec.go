/*
Copyright 2025 The Crossplane Authors.

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

// Package xpkg contains constants for values defined by Crossplane's xpkg specification.
package xpkg

import (
	"os"
)

const (
	// MetaFile is the name of a Crossplane package metadata file.
	MetaFile string = "crossplane.yaml"

	// StreamFile is the name of the file in a Crossplane package image that
	// contains its YAML stream.
	StreamFile string = "package.yaml"

	// StreamFileMode determines the permissions on the stream file.
	StreamFileMode os.FileMode = 0o644

	// XpkgExtension is the extension for compiled Crossplane packages.
	XpkgExtension string = ".xpkg"

	// XpkgMatchPattern is the match pattern for identifying compiled Crossplane packages.
	XpkgMatchPattern string = "*" + XpkgExtension

	// XpkgExamplesFile is the name of the file in a Crossplane package image
	// that contains the examples YAML stream.
	XpkgExamplesFile string = ".up/examples.yaml"

	// AnnotationKey is the key value for xpkg annotations.
	AnnotationKey string = "io.crossplane.xpkg"

	// PackageAnnotation is the annotation value used for the package.yaml
	// layer.
	PackageAnnotation string = "base"

	// ExamplesAnnotation is the annotation value used for the examples.yaml
	// layer.
	ExamplesAnnotation string = "upbound"

	// MetaAPIGroup is the API group for package metadata.
	MetaAPIGroup string = "meta.pkg.crossplane.io"

	// MetaAnnotationKey is the key value for xpkg metadata annotations.
	MetaAnnotationKey string = "meta.crossplane.io"
)
