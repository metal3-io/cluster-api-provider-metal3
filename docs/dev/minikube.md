# Development using Minikube

See the [Minikube docs](https://kubernetes.io/docs/setup/minikube/) for
instructions on launching a Minikube cluster.

## Add CRDs from cluster-api

Once your cluster is running, the CRDs defined by the `cluster-api` project
need to be added to the cluster.

```bash
kubectl apply -f vendor/sigs.k8s.io/cluster-api/config/crds/
```

## Add CRDs from baremetal-operator

The actuator also uses the `BareMetalHost` custom resource that’s defined by
the `baremetal-operator`.

```bash
kubectl apply -f vendor/github.com/metalkube/baremetal-operator/deploy/crds/metalkube_v1alpha1_baremetalhost_crd.yaml
```

## Create a BareMetalHost

When a `Machine` gets created, the actuator looks for an available
`BareMetalHost` to claim and then sets it to be provisioned to fulfill the
request expressed by the `Machine`.  Before creating a `Machine`, we can create
a dummy `BareMetalHost` object.  There’s no requirement to actually run the
`baremetal-operator` to test the reconciliation logic of the acutator.

```bash
kubectl apply -f vendor/github.com/metalkube/baremetal-operator/deploy/crds/metalkube_v1alpha1_baremetalhost_cr.yaml
```

## Run the Actuator

You can manually run the actuator from outside of the cluster for development
and testing purposes.  There’s a `Makefile` target which makes this easy.

```bash
make run
```

You can follow the output on the console to see information about what the
actuator is doing.  You can also proceed to create/update/delete `Machines` and
`BareMetalHosts` to test the actuator logic.
