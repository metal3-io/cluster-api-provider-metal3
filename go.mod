module github.com/metal3-io/cluster-api-provider-metal3

go 1.15

require (
	github.com/emicklei/go-restful v2.14.2+incompatible // indirect
	github.com/go-logr/logr v0.2.1
	github.com/go-openapi/spec v0.19.9 // indirect
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/gobuffalo/envy v1.7.1 // indirect
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gnostic v0.5.1 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20201008113413-e4fcc9b53e41
	github.com/metal3-io/ip-address-manager v0.0.4
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.2
	github.com/operator-framework/operator-sdk v0.17.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.13.0 // indirect
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	golang.org/x/net v0.0.0-20200904194848-62affa334b73
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43 // indirect
	golang.org/x/sys v0.0.0-20200909081042-eff7692f9009 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.0
	k8s.io/apiextensions-apiserver v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.19.0 // indirect
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.3.0 // indirect
	k8s.io/kube-openapi v0.0.0-20200831175022-64514a1d5d59 // indirect
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111
	sigs.k8s.io/cluster-api v0.3.9
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/client-go => k8s.io/client-go v0.19.0

replace github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0
