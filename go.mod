module github.com/crossplane/crossplane-runtime

go 1.13

require (
	github.com/Azure/go-autorest/autorest v0.9.2 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.0 // indirect
	github.com/crossplane/crossplane-tools v0.0.0-20200219001116-bb8b2ce46330
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/google/go-cmp v0.3.1
	github.com/gophercloud/gophercloud v0.6.0 // indirect
	github.com/hashicorp/go-getter v1.4.0
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0 // indirect
	k8s.io/api v0.18.0
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/client-go v0.18.0
	// TODO(negz): Return to a real release once the below PR is released.
	// https://github.com/kubernetes-sigs/controller-runtime/pull/863
	sigs.k8s.io/controller-runtime v0.5.1-0.20200422200944-a457e2791293
	sigs.k8s.io/controller-tools v0.2.4
	sigs.k8s.io/yaml v1.2.0
)
