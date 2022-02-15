/*
Copyright 2022 The Crossplane Authors.

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

package kubernetes

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
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
