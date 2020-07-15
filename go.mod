module github.com/metal3-io/cluster-api-provider-metal3

go 1.13

require (
	github.com/emicklei/go-restful v2.12.0+incompatible // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.8 // indirect
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/golang/mock v1.4.3
	github.com/google/gofuzz v1.1.0
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20200715105701-3207e795cae5
	github.com/metal3-io/ip-address-manager v0.0.3
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/securego/gosec v0.0.0-20200401082031-e946c8c39989 // indirect
	golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.0-alpha.2
	k8s.io/apiextensions-apiserver v0.18.3
	k8s.io/apimachinery v0.19.0-alpha.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.18.3 // indirect
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200520001619-278ece378a50
	sigs.k8s.io/cluster-api v0.3.6
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/client-go => k8s.io/client-go v0.19.0-alpha.2
