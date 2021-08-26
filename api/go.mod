module github.com/metal3-io/cluster-api-provider-metal3/api

go 1.16

require (
	github.com/google/gofuzz v1.2.0
	github.com/metal3-io/ip-address-manager/api v0.0.0-20210609163946-48b0ce9a1ac0
	github.com/onsi/gomega v1.14.0
	github.com/pkg/errors v0.9.1
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/utils v0.0.0-20210722164352-7f3ee0f31471
	sigs.k8s.io/cluster-api v0.4.2
	sigs.k8s.io/controller-runtime v0.9.6

)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.2
