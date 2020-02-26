module github.com/metal3-io/cluster-api-provider-baremetal

go 1.13

require (
	cloud.google.com/go v0.53.0 // indirect
	github.com/emicklei/go-restful v2.11.2+incompatible // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/go-openapi/spec v0.19.6 // indirect
	github.com/go-openapi/swag v0.19.7 // indirect
	github.com/golang/mock v1.4.0
	github.com/google/gofuzz v1.1.0
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/gophercloud/gophercloud v0.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/mdempsky/maligned v0.0.0-20180708014732-6e39bd26a8c8 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20200220133300-43fab21f7d0a
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.4.1 // indirect
	github.com/prometheus/procfs v0.0.10 // indirect
	github.com/securego/gosec v0.0.0-20200203094520-d13bb6d2420c // indirect
	golang.org/x/crypto v0.0.0-20200219234226-1ad67e1f0ef4 // indirect
	golang.org/x/net v0.0.0-20200219183655-46282727080f
	golang.org/x/sys v0.0.0-20200219091948-cb0a6d8edb6c // indirect
	k8s.io/api v0.17.3
	k8s.io/apiextensions-apiserver v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/cluster-bootstrap v0.17.3 // indirect
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200204173128-addea2498afe // indirect
	k8s.io/utils v0.0.0-20200124190032-861946025e34
	sigs.k8s.io/cluster-api v0.3.0-rc.0.0.20200216171528-7eead355bcbc
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0
	k8s.io/client-go => k8s.io/client-go v0.17.3
)
