package validation

import (
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// MockRequiredFields mocks required fields for a given resource
func MockRequiredFields(res *composite.Unstructured, s *apiextensions.JSONSchemaProps) error {
	o, err := fieldpath.PaveObject(res)
	if err != nil {
		return err
	}
	err = mockRequiredFieldsSchemaProps(s, o, "")
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), res)

}

// mockRequiredFieldsSchemaPropos mock required fields for a given schema property
func mockRequiredFieldsSchemaProps(prop *apiextensions.JSONSchemaProps, o *fieldpath.Paved, path string) error { //nolint:gocyclo // TODO: refactor
	if prop == nil {
		return nil
	}
	if prop.Type != "object" {
		return fmt.Errorf("mocking required fields is only supported for object types")
	}
	for _, s := range prop.Required {
		p := prop.Properties[s]
		pathSoFar := strings.TrimLeft(strings.Join([]string{path, s}, "."), ".")
		t := p.Type
		switch t {
		case "object":
			// TODO(phisco): avoid recursion
			if err := mockRequiredFieldsSchemaProps(&p, o, pathSoFar); err != nil {
				return err
			}
		case "array":
			if p.Items == nil || p.Items.Schema == nil {
				continue
			}
			pathSoFar = strings.TrimLeft(strings.Join([]string{pathSoFar, "[0]"}, ""), ".")
			if p.Items.Schema.Type == "object" {
				if err := mockRequiredFieldsSchemaProps(p.Items.Schema, o, pathSoFar); err != nil {
					return err
				}
				continue
			}
			t = p.Items.Schema.Type
			fallthrough
		default:
			if p.Default != nil {
				if err := o.SetValue(pathSoFar, p.Default); err == nil {
					continue
				}
			}
			if err := setTypeDefaultValue(o, pathSoFar, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// setTypeDefaultValue sets the default value for a given type at a given path
func setTypeDefaultValue(o *fieldpath.Paved, path string, t string) error {
	var v any
	switch t {
	case "boolean":
		v = false
	case "array":
		v = []any{}
	case "object":
		v = map[string]any{}
	case "string":
		v = ""
	case "integer":
		v = int64(0)
	case "number":
		// This is a hack, we should be able to set float64(0) directly,
		// but we are marshaling and then unmarshaling the value, and it is being
		// converted to an int64 if we use 0 as default
		v = float64(0.1)
	case "null":
	default:
		return fmt.Errorf("unknown type %s", t)
	}
	return o.SetValue(path, v)
}
