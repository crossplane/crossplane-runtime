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
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// These functions are mostly copied from the Kubernetes controller-runtime
// project. They can be removed following the implementation of
// https://github.com/kubernetes-sigs/controller-runtime/issues/691
// https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/envtest/crd.go

func readCRDs(path string) ([]*apiextensionsv1beta1.CustomResourceDefinition, error) {
	// Get the CRD files
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// White list the file extensions that may contain CRDs
	crdExts := sets.NewString(".json", ".yaml", ".yml")

	var crds []*apiextensionsv1beta1.CustomResourceDefinition
	for _, file := range files {
		// Only parse whitelisted file types
		if !crdExts.Has(filepath.Ext(file.Name())) {
			continue
		}

		// Unmarshal CRDs from file into structs
		docs, err := readDocuments(filepath.Join(path, file.Name()))
		if err != nil {
			return nil, err
		}

		for _, doc := range docs {
			crd := &apiextensionsv1beta1.CustomResourceDefinition{}
			if err = yaml.Unmarshal(doc, crd); err != nil {
				return nil, err
			}

			// Check that it is actually a CRD
			if crd.Spec.Names.Kind == "" || crd.Spec.Group == "" {
				continue
			}
			crds = append(crds, crd)
		}
	}
	return crds, nil
}

func readDocuments(fp string) ([][]byte, error) {
	b, err := os.ReadFile(filepath.Clean(fp))
	if err != nil {
		return nil, err
	}

	docs := [][]byte{}
	reader := k8syaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(b)))
	for {
		// Read document
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		docs = append(docs, doc)
	}

	return docs, nil
}
