# Cluster API Provider Metal3 for Managed Bare Metal Hardware

[![Ubuntu Integration v1beta1 build status](https://jenkins.nordix.org/view/Metal3/job/metal3_main_v1b1_integration_test_ubuntu/badge/icon?subject=Ubuntu%20Integration%20V1beta1)](https://jenkins.nordix.org/view/Metal3/job/metal3_main_v1b1_integration_test_ubuntu/)
[![CentOS Integration v1beta1 build status](https://jenkins.nordix.org/view/Metal3/job/metal3_main_v1b1_integration_test_centos/badge/icon?subject=CentOS%20Integration%20V1beta1)](https://jenkins.nordix.org/view/Metal3/job/metal3_main_v1b1_integration_test_centos/)
[![Ubuntu e2e v1beta1 build status](https://jenkins.nordix.org/view/Metal3%20Main/job/metal3_main_v1b1_e2e_test_ubuntu/badge/icon?subject=Ubuntu%20E2E%20V1beta1)](https://jenkins.nordix.org/view/Metal3%20Main/job/metal3_main_v1b1_e2e_test_ubuntu/)
[![CentOS e2e v1beta1 build status](https://jenkins.nordix.org/view/Metal3%20Main/job/metal3_main_v1b1_e2e_test_centos/badge/icon?subject=CentOS%20E2E%20V1beta1)](https://jenkins.nordix.org/view/Metal3%20Main/job/metal3_main_v1b1_e2e_test_centos/)

Kubernetes-native declarative infrastructure for Metal3.

## What is the Cluster API Provider Metal3

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api/) brings declarative,
Kubernetes-style APIs to cluster creation, configuration and management. The API
itself is shared across multiple cloud providers. Cluster API Provider Metal3 is
one of the providers for Cluster API and enables users to deploy a Cluster API based
cluster on top of bare metal infrastructure using Metal3.

## Compatibility with Cluster API

| CAPM3 version | Cluster API version | CAPM3 Release |
|---------------|---------------------|---------------|
| ~~v1alpha4~~  | ~~v1alpha3~~        | ~~v0.4.X~~    |
| v1alpha5      | v1alpha4            | v0.5.X        |
| v1beta1       | v1beta1             | v1.1.X        |

**Note:** We have stopped supporting CAPM3 v1alpha4 API version.

## Deploying the metal3 provider

The recommended method is using
[Clusterctl](https://main.cluster-api.sigs.k8s.io/clusterctl/overview.html).

Starting from `v0.5.0` release of Cluster API Provider Metal3, Baremetal Operator is decoupled
from Cluster API Provider Metal3 deployments when deployed via `clusterctl`. For this reason,
Baremetal Operator will not be installed when initializing the Metal3 provider with clusterctl,
and its CRDs and controller need to be manually installed. Example flow of installing Metal3
provider:

1. Install Cluster API core, bootstrap and control-plane providers. This will also install
  cert-manager if it is not already installed. To have more verbose logs you can use the -v flag
  when running the clusterctl and set the level of the logging verbose with a positive integer number, ie. -v5.

    ```shell
    clusterctl init --core cluster-api:v1.1.4 --bootstrap kubeadm:v1.1.4 \
        --control-plane kubeadm:v1.1.4 -v5
    ```

1. Install Metal3 provider. This will install the latest version of Cluster API Provider Metal3 CRDs and controllers.

    ```shell
    clusterctl init --infrastructure metal3
    ```

    You can also specify the provider version by appending a version tag to the provider name as follows:

    ```shell
    clusterctl init --infrastructure metal3:v1.1.1
    ```

1. Deploy Baremetal Operator manifests and CRDs. You need to install cert-manager for Baremetal Operator,
  but since step 1 already does it, we can skip it and only install the operator. Depending on
  whether you want TLS, or basic-auth enabled, kustomize paths may differ. Check operator [dev-setup doc](https://github.com/metal3-io/baremetal-operator/blob/main/docs/dev-setup.md)
  for more info.

    ```shell
    git clone https://github.com/metal3-io/baremetal-operator.git
    kubectl create namespace baremetal-operator-system
    cd baremetal-operator
    kustomize build config/default | kubectl apply -f -
    ```

1. Install Ironic. There are a couple of ways to do it.
    - Run within a Kubernetes cluster as a pod, refer to the [deploy.sh](https://github.com/metal3-io/baremetal-operator/blob/main/tools/deploy.sh)
      script.
    - Outside of a Kubernetes cluster as a container. Please refer to the [run_local_ironic.sh](https://github.com/metal3-io/baremetal-operator/blob/main/tools/run_local_ironic.sh) script.

Please refer to the [getting-started](docs/getting-started.md) for more info.

## Pivoting ⚠️

Starting from `v0.5.0` release of Cluster API Provider Metal3, Baremetal Operator is decoupled
from Cluster API Provider Metal3 deployments when deployed via `clusterctl`. For that reason,
when performing `clusterctl move`, custom objects outside of the Cluster API chain or not part
of CAPM3 will not be pivoted to a target cluster. An example of those objects is BareMetalHost, or
user created ConfigMaps and Secrets which are reconciled by Baremetal Operator. To ensure that those objects are
also pivoted as part of `clusterctl move`, `clusterctl.cluster.x-k8s.io` label needs to be set
on the BareMetalHost CRD before pivoting. If there are other CRDs also need to be pivoted to the
target cluster, the same label needs to be set on them.

All the other objects owned by BareMetalHost, such as Secret and ConfigMap don't require this
label to be set, because they hold ownerReferences to BareMetalHost, and that is good enough
for clusterctl to move all the hierarchy of BareMetalHost object.

## Development Environment

There are multiple ways to setup a development environment:

- [Using Tilt](docs/dev-setup.md#tilt-development-environment)
- [Other management cluster](docs/dev-setup.md#development-using-Kind-or-Minikube)
- See [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env) for an
  end-to-end development and test environment for
  `cluster-api-provider-metal3` and
  [baremetal-operator](https://github.com/metal3-io/baremetal-operator).

## API

See the [API Documentation](docs/api.md) for details about the objects used with
this Cluster API provider. You can also see the [cluster deployment
workflow](docs/deployment_workflow.md) for the outline of the
deployment process.

## Architecture

The architecture with the components involved is documented [here](docs/architecture.md)

## E2E test

To trigger e2e test on a PR, use the following phrases:

On main branch:

- **/test-v1b1-ubuntu-e2e** runs v1b1 e2e tests on Ubuntu
- **/test-v1b1-centos-e2e** runs v1b1 e2e tests on CentOS

Release-0.5 branch:

- **/test-v1a5-ubuntu-e2e** runs v1a5 e2e tests on Ubuntu
- **/test-v1a5-centos-e2e** runs v1a5 e2e tests on CentOS

Upgrade tests:

- **/test-v1b1-upgrade-e2e** runs e2e upgrade tests from v1a5 to v1b1 on Ubuntu

More info about e2e test can be found [here](docs/e2e-test.md)
