# Setting up a development environment

## Pre-requisites

CAPM3 requires two external tools for running the tests
during development.

### Install kustomize

```bash
./hack/tools/install_kustomize.sh
```

### Install kubebuilder

```bash
./hack/tools/install_kubebuilder.sh
```

## Development using Kind or Minikube

See the [Kind docs](https://kind.sigs.k8s.io/docs/user/quick-start) for
instructions on launching a Kind cluster and the
[Minikube docs](https://kubernetes.io/docs/setup/minikube/) for
instructions on launching a Minikube cluster.

### Add CRDs and CRs from baremetal-operator

The provider also uses the `BareMetalHost` custom resource that’s defined by
the `baremetal-operator`. The following command deploys the CRD and creates
dummy BareMetalHosts.

```sh
    make deploy-bmo-cr
```

When a `Metal3Machine` gets created, the provider looks for an available
`BareMetalHost` to claim and then sets it to be provisioned to fulfill the
request expressed by the `Metal3Machine`. Before creating a
`Metal3Machine`, we can create a dummy `BareMetalHost` object. There’s no
requirement to actually run the
`baremetal-operator` to test the reconciliation logic of the provider.

Refer to the [baremetal-operator developer
documentation](https://github.com/metal3-io/baremetal-operator/blob/master/docs/dev-setup.md)
for instructions and tools for creating BareMetalHost objects.

### Deploy CAPI and CAPM3

The following command will deploy the controllers from CAPI, CABPK and CAPM3 and
the requested CRDs.

```sh
    make deploy
```

### Run the Controller locally

You will first need to scale down the controller deployment :

```sh
    kubectl scale -n capm3-system deployment.v1.apps/capm3-controller-manager \
      --replicas 0
```

You can manually run the controller from outside of the cluster for development
and testing purposes. There’s a `Makefile` target which makes this easy.

```bash
make run
```

You can follow the output on the console to see information about what the
controller is doing. You can also proceed to create/update/delete
`Metal3Machines` and `BareMetalHosts` to test the controller logic.
