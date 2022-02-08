package kubernetes

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Error strings.
const (
	errLoadKubeconfig   = "cannot load kubeconfig"
	errBuildRestConfig  = "cannot build rest config kubeconfig"
	errCreateClient     = "cannot create Kubernetes client"
	errNoCurrentContext = "currentContext not set in kubeconfig"

	errFmtNoClusterForContext = "cluster for currentContext %q not found"
)

// Note(turkenh): We should probably move this function to a more common package
// and expose to be used by K8s based providers, i.e. provider-helm and
// provider-kubernetes.
func clientForKubeconfig(kubeconfig []byte) (client.Client, error) {
	ac, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, errors.Wrap(err, errLoadKubeconfig)
	}
	config, err := restConfigFromAPIConfig(ac)
	if err != nil {
		return nil, errors.Wrap(err, errBuildRestConfig)
	}
	kc, err := client.New(config, client.Options{})
	return kc, errors.Wrap(err, errCreateClient)
}

func restConfigFromAPIConfig(c *api.Config) (*rest.Config, error) {
	if c.CurrentContext == "" {
		return nil, errors.New(errNoCurrentContext)
	}
	ctx := c.Contexts[c.CurrentContext]
	cluster := c.Clusters[ctx.Cluster]
	if cluster == nil {
		return nil, errors.Errorf(errFmtNoClusterForContext, c.CurrentContext)
	}
	user := c.AuthInfos[ctx.AuthInfo]
	if user == nil {
		// We don't require a user because it's possible user
		// authorization configuration will be loaded from a separate
		// set of identity credentials (e.g. Google Application Creds).
		user = &api.AuthInfo{}
	}
	return &rest.Config{
		Host:            cluster.Server,
		Username:        user.Username,
		Password:        user.Password,
		BearerToken:     user.Token,
		BearerTokenFile: user.TokenFile,
		Impersonate: rest.ImpersonationConfig{
			UserName: user.Impersonate,
			Groups:   user.ImpersonateGroups,
			Extra:    user.ImpersonateUserExtra,
		},
		AuthProvider: user.AuthProvider,
		ExecProvider: user.Exec,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   cluster.InsecureSkipTLSVerify,
			ServerName: cluster.TLSServerName,
			CertData:   user.ClientCertificateData,
			KeyData:    user.ClientKeyData,
			CAData:     cluster.CertificateAuthorityData,
		},
	}, nil
}
