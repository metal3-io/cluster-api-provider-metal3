module github.com/metal3-io/cluster-api-provider-metal3

go 1.16

require (
	github.com/containerd/containerd v1.5.7 // indirect
	github.com/docker/docker v20.10.7+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.6.0
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/jinzhu/copier v0.3.2
	github.com/metal3-io/baremetal-operator/apis v0.0.0-20210928111743-2abb8f3d5aa5
	github.com/metal3-io/cluster-api-provider-metal3/api v0.0.0-20211015151901-0e880ce119c4
	github.com/metal3-io/ip-address-manager/api v0.0.0-20211018090204-6be1b3878f19
	github.com/onsi/ginkgo v1.16.6-0.20211027140633-0db9665232ca
	github.com/onsi/gomega v1.16.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubernetes v1.13.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.0
	sigs.k8s.io/cluster-api/test v1.0.0
	sigs.k8s.io/controller-runtime v0.10.3-0.20211011182302-43ea648ec318
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/metal3-io/cluster-api-provider-metal3/api => ./api

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.0.0
