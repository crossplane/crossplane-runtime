package certificates

import (
	"crypto/tls"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	errNoSuchFile = errors.New("open invalid/path/tls.crt: no such file or directory")
	errNoCAFile   = errors.New("open fake/no-ca/ca.crt: no such file or directory")
)

func TestLoad(t *testing.T) {
	type args struct {
		certsFolderPath         string
		requireClientValidation bool
	}
	type want struct {
		err error
		out *tls.Config
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"LoadCertError": {
			reason: "Should return a proper error if certificates do not exist.",
			args: args{
				certsFolderPath: "invalid/path",
			},
			want: want{
				err: errors.Wrap(errNoSuchFile, errLoadCert),
				out: nil,
			},
		},
		"LoadCAError": {
			reason: "Should return a proper error if CA certificate does not exist.",
			args: args{
				certsFolderPath: "fake/no-ca",
			},
			want: want{
				err: errors.Wrap(errNoCAFile, errLoadCA),
				out: nil,
			},
		},
		"InvalidCAError": {
			reason: "Should return a proper error if CA certificate is not valid.",
			args: args{
				certsFolderPath: "fake/invalid-certs/",
			},
			want: want{
				err: errors.New(errInvalidCA),
				out: nil,
			},
		},
		"NoError": {
			reason: "Should not return an error after loading certificates.",
			args: args{
				certsFolderPath: "fake/certs/",
			},
			want: want{
				err: nil,
				out: &tls.Config{},
			},
		},
		"NoErrorWithClientValidation": {
			reason: "Should not return an error after loading certificates.",
			args: args{
				certsFolderPath:         "fake/certs/",
				requireClientValidation: true,
			},
			want: want{
				err: nil,
				out: &tls.Config{
					ClientAuth: tls.RequireAndVerifyClientCert,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			certsFolderPath := tc.args.certsFolderPath
			requireClient := tc.args.requireClientValidation

			cfg, err := LoadMTLSConfig(certsFolderPath, requireClient)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if requireClient {
				if diff := cmp.Diff(tc.want.out.ClientAuth, cfg.ClientAuth); diff != "" {
					t.Errorf("\n%s\nLoad(...): -want, +got:\n%s", tc.reason, diff)
				}
			}
		})
	}
}
