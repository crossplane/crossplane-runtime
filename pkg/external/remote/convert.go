/*
Copyright 2024 The Crossplane Authors.

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

// Package remote implements a 'remote' ExternalClient, reached via gRPC.
package remote

import (
	"github.com/go-json-experiment/json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// AsManaged gets the supplied managed resource from the supplied struct.
func AsManaged(s *structpb.Struct, mg resource.Managed) error {
	// We try to avoid a JSON round-trip if mg is backed by unstructured data.
	// Any type that is or embeds *unstructured.Unstructured has this method.
	if u, ok := mg.(interface{ SetUnstructuredContent(map[string]any) }); ok {
		u.SetUnstructuredContent(s.AsMap())
		return nil
	}

	b, err := protojson.Marshal(s)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal %T to JSON", s)
	}
	return errors.Wrapf(json.Unmarshal(b, mg, json.RejectUnknownMembers(true)), "cannot unmarshal JSON from %T into %T", s, mg)
}

// AsStruct gets the supplied struct from the supplied managed resource.
func AsStruct(mg resource.Managed) (*structpb.Struct, error) {
	// We try to avoid a JSON round-trip if mg is backed by unstructured data.
	// Any type that is or embeds *unstructured.Unstructured has this method.
	if u, ok := mg.(interface{ UnstructuredContent() map[string]any }); ok {
		s, err := structpb.NewStruct(u.UnstructuredContent())
		return s, errors.Wrapf(err, "cannot create new Struct from %T", u)
	}

	b, err := json.Marshal(mg)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal %T to JSON", mg)
	}
	s := &structpb.Struct{}
	return s, errors.Wrapf(protojson.Unmarshal(b, s), "cannot unmarshal JSON from %T into %T", mg, s)
}
