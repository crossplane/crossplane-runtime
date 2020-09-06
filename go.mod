module github.com/crossplane/crossplane-runtime

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.4.0
	github.com/hashicorp/go-getter v1.4.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.2.2
	golang.org/x/tools v0.0.0-20191018212557-ed542cd5b28a // indirect
	k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.2.4
	sigs.k8s.io/yaml v1.2.0
)
