module github.com/metal3-io/cluster-api-provider-metal3/api

go 1.16

require (
	github.com/google/gofuzz v1.2.0
	github.com/metal3-io/ip-address-manager/api v0.0.0-20211018090204-6be1b3878f19
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.2
	sigs.k8s.io/controller-runtime v0.10.3

)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.0.2
