module github.com/metal3-io/cluster-api-provider-metal3/api

go 1.16

require (
	github.com/google/gofuzz v1.2.0
	github.com/metal3-io/ip-address-manager/api v0.0.0-20211018090204-6be1b3878f19
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/cluster-api v0.4.4
	sigs.k8s.io/controller-runtime v0.9.7

)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.4
