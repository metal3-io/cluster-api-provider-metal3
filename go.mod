module github.com/metal3-io/cluster-api-provider-metal3

go 1.13

require (
	cloud.google.com/go v0.55.0 // indirect
	github.com/ant31/crd-validation v0.0.0-20180702145049-30f8a35d0ac2 // indirect
	github.com/bugsnag/bugsnag-go v1.5.0 // indirect
	github.com/bugsnag/panicwrap v1.2.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a // indirect
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/emicklei/go-restful v2.12.0+incompatible // indirect
	github.com/garyburd/redigo v1.6.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.7 // indirect
	github.com/go-openapi/swag v0.19.8 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible // indirect
	github.com/golang/lint v0.0.0-20180702182130-06c8688daad7 // indirect
	github.com/golang/mock v1.4.3
	github.com/google/gofuzz v1.1.0
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/gorilla/handlers v1.4.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190203031600-7a902570cb17 // indirect
	github.com/improbable-eng/thanos v0.3.2 // indirect
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/mdempsky/maligned v0.0.0-20180708014732-6e39bd26a8c8 // indirect
	github.com/metal3-io/baremetal-operator v0.0.0-20200617072321-b35c5e9bceba
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.0.11 // indirect
	github.com/prometheus/tsdb v0.8.0 // indirect
	github.com/securego/gosec v0.0.0-20200203094520-d13bb6d2420c // indirect
	github.com/xenolf/lego v0.3.2-0.20160613233155-a9d8cec0e656 // indirect
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.6 // indirect
	golang.org/x/crypto v0.0.0-20200317142112-1b76d66859c6 // indirect
	golang.org/x/net v0.0.0-20200319234117-63522dbf7eec
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	gonum.org/v1/gonum v0.0.0-20190710053202-4340aa3071a0 // indirect
	gopkg.in/square/go-jose.v1 v1.1.2 // indirect
	k8s.io/api v0.17.4
	k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cluster-bootstrap v0.17.4 // indirect
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200204173128-addea2498afe // indirect
	k8s.io/kubernetes v1.16.2 // indirect
	k8s.io/utils v0.0.0-20200318093247-d1ab8797c558
	sigs.k8s.io/cluster-api v0.3.2
	sigs.k8s.io/controller-runtime v0.5.2
	sigs.k8s.io/testing_frameworks v0.1.2 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.17.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.3
	k8s.io/apiserver => k8s.io/apiserver v0.17.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.3
	k8s.io/client-go => k8s.io/client-go v0.17.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.3
	k8s.io/code-generator => k8s.io/code-generator v0.17.3
	k8s.io/component-base => k8s.io/component-base v0.17.3
	k8s.io/cri-api => k8s.io/cri-api v0.17.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.3
	k8s.io/kubectl => k8s.io/kubectl v0.17.3
	k8s.io/kubelet => k8s.io/kubelet v0.17.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.3
	k8s.io/metrics => k8s.io/metrics v0.17.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.17.3
) // Required by BMO

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by BMO

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required by BMO until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved

replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0 // Issue with go-client version
