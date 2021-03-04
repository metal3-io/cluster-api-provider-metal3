module github.com/metal3-io/cluster-api-provider-metal3

go 1.15

require (
	cloud.google.com/go v0.74.0 // indirect
	github.com/emicklei/go-restful v2.14.2+incompatible // indirect
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.3.0 // indirect
	github.com/go-openapi/spec v0.19.9 // indirect
	github.com/go-openapi/swag v0.19.9 // indirect
	github.com/gobuffalo/envy v1.7.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.4.4
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.1.4 // indirect
	github.com/googleapis/gnostic v0.5.3 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20210111093319-93a6fd209b9a
	github.com/metal3-io/ip-address-manager v0.0.4
	github.com/nxadm/tail v1.4.6 // indirect
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/operator-framework/operator-sdk v0.17.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0 // indirect
	github.com/prometheus/procfs v0.3.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/net v0.0.0-20201224014010-6772e930b67b
	golang.org/x/sys v0.0.0-20210110051926-789bb1bd4061 // indirect
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.20.1
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.20.1 // indirect
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.4.0 // indirect
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd // indirect
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/cluster-api v0.3.12
	sigs.k8s.io/controller-runtime v0.7.0
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
