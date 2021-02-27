module github.com/metal3-io/cluster-api-provider-metal3/api

go 1.16

require (
	github.com/google/gofuzz v1.2.0
	github.com/metal3-io/ip-address-manager/api v0.0.0-20210609163946-48b0ce9a1ac0
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.4.0
	sigs.k8s.io/controller-runtime v0.9.1

)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.0
