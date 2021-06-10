module github.com/metal3-io/cluster-api-provider-metal3/api

go 1.16

replace k8s.io/client-go => k8s.io/client-go v0.17.9

replace k8s.io/apimachinery => k8s.io/apimachinery v0.17.9

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.9

replace k8s.io/api => k8s.io/api v0.17.9

replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29

replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.5.14

require (
	github.com/google/gofuzz v1.2.0
	github.com/metal3-io/ip-address-manager/api v0.0.0-20210609163946-48b0ce9a1ac0
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v1.5.1
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.3.17
	sigs.k8s.io/controller-runtime v0.8.3
)
