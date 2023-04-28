# Cluster API Provider Metal3 for Managed Bare Metal Hardware

[![Ubuntu E2E Integration 1.2 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-2_integration_test_ubuntu&subject=Ubuntu%20e2e%20integration%201.2)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-2_integration_test_ubuntu/)
[![CentOS E2E Integration 1.2 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-2_integration_test_centos&subject=Centos%20e2e%20integration%201.2)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-2_integration_test_centos/)
[![Ubuntu E2E feature 1.2 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-2_e2e_feature_test_ubuntu/&subject=Ubuntu%20E2E%20feature%201.2)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-2_e2e_feature_test_ubuntu/)
[![CentOS E2E feature 1.2 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-2_e2e_feature_test_centos/&subject=CentOS%20E2E%20feature%201.2)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-2_e2e_feature_test_centos/)

Kubernetes-native declarative infrastructure for Metal3.

## What is the Cluster API Provider Metal3

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api/) brings declarative,
Kubernetes-style APIs to cluster creation, configuration and management. The API
itself is shared across multiple cloud providers. Cluster API Provider Metal3 is
one of the providers for Cluster API and enables users to deploy a Cluster API based
cluster on top of bare metal infrastructure using Metal3.

## Compatibility with Cluster API

| CAPM3 version | Cluster API version | CAPM3 Release | CAPI Release |
|---------------|---------------------|---------------|--------------|
| v1alpha5      | v1alpha4            | v0.5.X        | v0.4.X       |
| v1beta1       | v1beta1             | v1.1.X        | v1.1.X       |
| v1beta1       | v1beta1             | v1.2.X        | v1.2.X       |

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
    clusterctl init --core cluster-api:v1.2.11 --bootstrap kubeadm:v1.2.11 \
        --control-plane kubeadm:v1.2.11 -v5
    ```

1. Install Metal3 provider. This will install the latest version of Cluster API Provider Metal3 CRDs and controllers.

    ```shell
    clusterctl init --infrastructure metal3
    ```

    You can also specify the provider version by appending a version tag to the provider name as follows:

    ```shell
    clusterctl init --infrastructure metal3:v1.2.0
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

- **/test-ubuntu-e2e-main** runs e2e tests with CAPM3 API version v1beta1 and branch main on Ubuntu
- **/test-centos-e2e-main** runs e2e tests with CAPM3 API version v1beta1 and branch main on CentOS

Or use parallel for faster tests. Note that these tests run in multiple VMs by creating an independent VM for each test spec:

- **/test-ubuntu-e2e-parallel-main** runs e2e parallel tests with CAPM3 API version v1beta1 and branch main on Ubuntu
- **/test-centos-e2e-parallel-main** runs e2e parallel tests with CAPM3 API version v1beta1 and branch main on CentOS

Release-1.2 branch:

- **/test-ubuntu-e2e-release-1-2** runs e2e tests with CAPM3 API version v1beta1 and branch release-1.2 on Ubuntu
- **/test-centos-e2e-release-1-2** runs e2e tests with CAPM3 API version v1beta1 and branch release-1.2 on CentOS

Release-1.1 branch:

- **/test-ubuntu-e2e-release-1-1** runs e2e tests with CAPM3 API version v1beta1 and branch release-1.1 on Ubuntu
- **/test-centos-e2e-release-1-1** runs e2e tests with CAPM3 API version v1beta1 and branch release-1.1 on CentOS

Release-0.5 branch:

- **/test-ubuntu-e2e-release-0-5** runs e2e tests with CAPM3 API version v1alpha5 and branch release-0.5 on Ubuntu
- **/test-centos-e2e-release-0-5** runs e2e tests with CAPM3 API version v1alpha5 and branch release-0.5 on CentOS

Upgrade tests:

- **/test-upgrade-e2e-main** runs e2e upgrade tests from CAPM3 API version v1alpha5/branch release-0.5 to CAPM3 API version v1beta1/branch main on Ubuntu

More info about e2e test can be found [here](docs/e2e-test.md)
