module github.com/metal3-io/cluster-api-provider-metal3

go 1.16

require (
	cloud.google.com/go v0.78.0 // indirect
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.5.0
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.0.0-20210416073321-c927d1d8da76
	github.com/metal3-io/ip-address-manager v0.0.5
	github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.18.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83 // indirect
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/oauth2 v0.0.0-20210220000619-9bb904979d93 // indirect
	golang.org/x/sys v0.0.0-20210309074719-68d13333faf2 // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.20.4
	k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.20.4 // indirect
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.6.0 // indirect
	k8s.io/kube-openapi v0.0.0-20210305164622-f622666832c1 // indirect
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/cluster-api v0.3.14
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/client-go => k8s.io/client-go v0.17.9

replace k8s.io/apimachinery => k8s.io/apimachinery v0.17.9

replace k8s.io/api => k8s.io/api v0.17.9

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.9

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.9

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.5.14

replace github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0

replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29

replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0
