package parser

import (
	"bytes"
	"context"
	"io"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

// FuzzParse is a fuzz test function used to test the robustness of the parser.
func FuzzParse(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte) {
		objScheme := runtime.NewScheme()
		metaScheme := runtime.NewScheme()
		p := New(metaScheme, objScheme)
		r := io.NopCloser(bytes.NewReader(data))
		_, _ = p.Parse(context.Background(), r)
	})
}
