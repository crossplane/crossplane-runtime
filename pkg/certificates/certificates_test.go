package certificates

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const (
	caCertFileName  = "ca.crt"
	tlsCertFileName = "tls.crt"
	tlsKeyFileName  = "tls.key"
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
				err: os.ErrNotExist,
				out: nil,
			},
		},
		"LoadCAError": {
			reason: "Should return a proper error if CA certificate does not exist.",
			args: args{
				certsFolderPath: "test-data/no-ca",
			},
			want: want{
				err: os.ErrNotExist,
				out: nil,
			},
		},
		"InvalidCAError": {
			reason: "Should return a proper error if CA certificate is not valid.",
			args: args{
				certsFolderPath: "test-data/invalid-certs/",
			},
			want: want{
				// TODO(negz): Can we be more specific? Should we use a sentinel
				// error?
				err: cmpopts.AnyError,
				out: nil,
			},
		},
		"NoError": {
			reason: "Should not return an error after loading certificates.",
			args: args{
				certsFolderPath: "test-data/certs/",
			},
			want: want{
				err: nil,
				out: &tls.Config{},
			},
		},
		"NoErrorWithClientValidation": {
			reason: "Should not return an error after loading certificates.",
			args: args{
				certsFolderPath:         "test-data/certs/",
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

			cfg, err := LoadMTLSConfig(filepath.Join(certsFolderPath, caCertFileName), filepath.Join(certsFolderPath, tlsCertFileName), filepath.Join(certsFolderPath, tlsKeyFileName), requireClient)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
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
