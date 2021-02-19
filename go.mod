module github.com/crossplane/crossplane-runtime

go 1.13

require (
	github.com/go-logr/logr v0.3.0
	github.com/google/go-cmp v0.5.2
	github.com/hashicorp/go-getter v1.4.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.2.2
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	k8s.io/api v0.20.1
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v0.20.1
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/controller-tools v0.2.4
	sigs.k8s.io/yaml v1.2.0
)
