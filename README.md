# Cluster API Provider for Managed Bare Metal Hardware

This repository contains a Machine actuator implementation for the
Kubernetes [Cluster API](https://github.com/kubernetes-sigs/cluster-api/).

For more information about this actuator and related repositories, see
[metal3.io](http://metal3.io/).

## Development Environment

* See [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env) for an
  end-to-end development and test environment for
  `cluster-api-provider-baremetal` and
  [baremetal-operator](https://github.com/metal3-io/baremetal-operator).
* [Setting up for tests](docs/dev/setup.md)
* Using [Minikube](docs/dev/minikube.md)
* Using [OpenShift 4](docs/dev/openshift.md)

## ProviderSpec

In order to create a valid Machine resource, you must include a ProviderSpec
that looks like the following example. See the
[type definition](pkg/apis/baremetal/v1alpha1/baremetalmachineproviderspec_types.go)
for details on each field.

```
apiVersion: cluster.k8s.io/v1alpha1
kind: Machine
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: sample0
spec:
  providerSpec:
    value:
      apiVersion: "baremetal.cluster.k8s.io/v1alpha1"
      kind: "BareMetalMachineProviderSpec"
      image:
        url: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2"
        checksum: "http://172.22.0.1/images/rhcos-ootpa-latest.qcow2.md5sum"
      userData:
        Name: "worker-user-data"
```
