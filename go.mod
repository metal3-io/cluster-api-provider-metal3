module github.com/metal3-io/cluster-api-provider-metal3

go 1.16

require (
	github.com/containerd/containerd v1.5.9 // indirect
	github.com/docker/docker v20.10.11+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.5.0
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/jinzhu/copier v0.3.2
	github.com/metal3-io/baremetal-operator/apis v0.0.0-20211105090508-c38de6aabf99
	github.com/metal3-io/cluster-api-provider-metal3/api v0.0.0-20211015151901-0e880ce119c4
	github.com/metal3-io/ip-address-manager/api v0.0.0-20220218143845-45c86dc10798
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.21.4
	k8s.io/apiextensions-apiserver v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/cluster-api v0.4.8
	sigs.k8s.io/cluster-api/test v0.4.8
	sigs.k8s.io/controller-runtime v0.9.7
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/metal3-io/cluster-api-provider-metal3/api => ./api

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.8
