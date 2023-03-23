package validation

import (
	"reflect"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

func Test_mockRequiredFieldsSchemaProps(t *testing.T) {
	type args struct {
		prop *apiextensions.JSONSchemaProps
		o    *fieldpath.Paved
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		output  *fieldpath.Paved
	}{
		{
			name: "should error if given prop is not an object",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type: "string",
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: true,
		}, {
			name: "should not error if given prop is nil",
			args: args{
				prop: nil,
				o:    fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			output:  fieldpath.Pave(map[string]any{}),
		}, {
			name: "should mock required string field in simple object",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type: "string",
						},
					},
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			output:  fieldpath.Pave(map[string]any{"foo": ""}),
		}, {
			name: "should mock required string field in nested object",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type:     "object",
							Required: []string{"bar"},
							Properties: map[string]apiextensions.JSONSchemaProps{
								"bar": {
									Type: "string",
								},
							},
						},
					},
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			output:  fieldpath.Pave(map[string]any{"foo": map[string]any{"bar": ""}}),
		}, {
			name: "should mock required integer field in nested object",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type:     "object",
							Required: []string{"bar"},
							Properties: map[string]apiextensions.JSONSchemaProps{
								"bar": {
									Type: "integer",
								},
							},
						},
					},
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			output:  fieldpath.Pave(map[string]any{"foo": map[string]any{"bar": int64(0)}}),
		}, {
			name: "should mock required number field in nested object",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type:     "object",
							Required: []string{"bar"},
							Properties: map[string]apiextensions.JSONSchemaProps{
								"bar": {
									Type: "number",
								},
							},
						},
					},
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			// TODO: this should be float64(0) but we are using json.Unmarshal to
			// unmarshal the schema which converts all numbers to int64.
			output: fieldpath.Pave(map[string]any{"foo": map[string]any{"bar": float64(0.1)}}),
		}, {
			name: "should mock required string field in nested object",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type:  "array",
							Items: &apiextensions.JSONSchemaPropsOrArray{Schema: &apiextensions.JSONSchemaProps{Type: "string"}},
						},
					},
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			output:  fieldpath.Pave(map[string]any{"foo": []any{""}}),
		}, {
			name: "should mock required string field in an array of objects",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type: "array",
							Items: &apiextensions.JSONSchemaPropsOrArray{Schema: &apiextensions.JSONSchemaProps{
								Type:       "object",
								Required:   []string{"bar"},
								Properties: map[string]apiextensions.JSONSchemaProps{"bar": {Type: "string"}},
							}},
						},
					},
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			output:  fieldpath.Pave(map[string]any{"foo": []any{map[string]any{"bar": ""}}}),
		}, {
			name: "should mock required string fields in an array of objects and a simple object",
			args: args{
				prop: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo", "baz"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"baz": {
							Type: "string",
						},
						"foo": {
							Type: "array",
							Items: &apiextensions.JSONSchemaPropsOrArray{Schema: &apiextensions.JSONSchemaProps{
								Type:       "object",
								Required:   []string{"bar"},
								Properties: map[string]apiextensions.JSONSchemaProps{"bar": {Type: "string"}},
							}},
						},
					},
				},
				o: fieldpath.Pave(map[string]any{}),
			},
			wantErr: false,
			output:  fieldpath.Pave(map[string]any{"foo": []any{map[string]any{"bar": ""}}, "baz": ""}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := mockRequiredFieldsSchemaProps(tt.args.prop, tt.args.o, ""); (err != nil) != tt.wantErr {
				t.Errorf("mockRequiredFieldsSchemaProps() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.output != nil && !reflect.DeepEqual(tt.args.o.UnstructuredContent(), tt.output.UnstructuredContent()) {
				t.Errorf("mockRequiredFieldsSchemaProps() output = %+v, want %+v", tt.args.o.UnstructuredContent(), tt.output.UnstructuredContent())
			}
		})
	}
}
