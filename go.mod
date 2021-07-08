module github.com/crossplane/crossplane-runtime

go 1.13

require (
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/hashicorp/go-getter v1.4.0
	github.com/imdario/mergo v0.3.12
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.2.2
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	k8s.io/api v0.21.2
	k8s.io/apiextensions-apiserver v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	sigs.k8s.io/controller-runtime v0.9.2
	sigs.k8s.io/controller-tools v0.2.4
	sigs.k8s.io/yaml v1.2.0
)
