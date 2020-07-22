module github.com/metal3-io/cluster-api-provider-metal3

go 1.13

require (
	github.com/emicklei/go-restful v2.13.0+incompatible // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.9 // indirect
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/golang/mock v1.4.3
	github.com/google/gofuzz v1.1.0
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20200720071502-fdf45f0a9f6e
	github.com/metal3-io/ip-address-manager v0.0.4
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.0-alpha.2
	k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery v0.19.0-alpha.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.18.6 // indirect
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200720150651-0bdb4ca86cbc
	sigs.k8s.io/cluster-api v0.3.7
	sigs.k8s.io/controller-runtime v0.6.1
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/client-go => k8s.io/client-go v0.19.0-alpha.2
