# Cluster API Provider Metal3 for Managed Bare Metal Hardware

[![CLOMonitor](https://img.shields.io/endpoint?url=https://clomonitor.io/api/projects/cncf/metal3-io/badge)](https://clomonitor.io/projects/cncf/metal3-io)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/9970/badge)](https://www.bestpractices.dev/projects/9970)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/metal3-io/cluster-api-provider-metal3/badge)](https://securityscorecards.dev/viewer/?uri=github.com/metal3-io/cluster-api-provider-metal3)
[![Ubuntu E2E Integration 1.10 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3-periodic-ubuntu-e2e-integration-test-release-1-10&subject=Ubuntu%20E2E%20integration%201.10)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3-periodic-ubuntu-e2e-integration-test-release-1-10/)
[![CentOS E2E Integration 1.10 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3-periodic-centos-e2e-integration-test-release-1-10&subject=Centos%20E2E%20integration%201.10)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3-periodic-centos-e2e-integration-test-release-1-10/)
[![CentOS E2E feature 1.10 pivot build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3-periodic-centos-e2e-feature-test-release-1-10-pivoting/&subject=CentOS%20E2E%20feature%201.10%20pivot)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3-periodic-centos-e2e-feature-test-release-1-10-pivoting/)
[![CentOS E2E feature 1.10 remediation build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3-periodic-centos-e2e-feature-test-release-1-10-remediation/&subject=CentOS%20E2E%20feature%201.10%20remediatoin)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3-periodic-centos-e2e-feature-test-release-1-10-remediation/)
[![CentOS E2E feature 1.10 other features build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3-periodic-centos-e2e-feature-test-release-1-10-features/&subject=CentOS%20E2E%20feature%201.10%20other%20features)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3-periodic-centos-e2e-feature-test-release-1-10-features/)

Kubernetes-native declarative infrastructure for Metal3.

## What is the Cluster API Provider Metal3

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api/) brings
declarative, Kubernetes-style APIs to cluster creation, configuration and
management. The API itself is shared across multiple cloud providers. Cluster
API Provider Metal3 is one of the providers for Cluster API and enables users to
deploy a Cluster API based cluster on top of bare metal infrastructure using
Metal3.

## Compatibility with Cluster API

| CAPM3 version | Cluster API version | CAPM3 Release |  CAPI Release  |
| ------------- | ------------------- | ------------- | -------------- |
| v1beta1       | v1beta1             | v1.1.X        |  v1.1.X        |
| v1beta1       | v1beta1             | v1.2.X        |  v1.2.X        |
| v1beta1       | v1beta1             | v1.3.X        |  v1.3.X        |
| v1beta1       | v1beta1             | v1.4.X        |  v1.4.X        |
| v1beta1       | v1beta1             | v1.5.X        |  v1.5.X        |
| v1beta1       | v1beta1             | v1.6.X        |  v1.6.X        |
| v1beta1       | v1beta1             | v1.7.X        |  v1.7.X        |
| v1beta1       | v1beta1             | v1.8.X        |  v1.8.X        |
| v1beta1       | v1beta1             | v1.9.X        |  v1.9.X        |
| v1beta1       | v1beta1             | v1.10.X       |  v1.10.X       |

## Deploying the metal3 provider

The recommended method is using
[Clusterctl](https://main.cluster-api.sigs.k8s.io/clusterctl/overview.html).

Starting from `v0.5.0` release of Cluster API Provider Metal3, Baremetal
Operator is decoupled from Cluster API Provider Metal3 deployments when deployed
via `clusterctl`. For this reason, Baremetal Operator will not be installed when
initializing the Metal3 provider with clusterctl, and its CRDs and controller
need to be manually installed. Example flow of installing Metal3 provider:

1. Install Cluster API core, bootstrap and control-plane providers. This will
   also install cert-manager if it is not already installed. To have more
   verbose logs you can use the -v flag when running the clusterctl and set the
   level of the logging verbose with a positive integer number, ie. -v5.

   ```shell
   clusterctl init --core cluster-api:v1.10.0 \
       --bootstrap kubeadm:v1.10.0 \
       --control-plane kubeadm:v1.10.0 -v5
   ```

1. Install Metal3 provider. This will install the latest version of Cluster API
   Provider Metal3 CRDs and controllers.

   ```shell
   clusterctl init --infrastructure metal3
   ```

   You can also specify the provider version by appending a version tag to the
   provider name as follows:

   ```shell
   clusterctl init --infrastructure metal3:v1.10.0
   ```

1. Deploy Baremetal Operator manifests and CRDs. You need to install
   cert-manager for Baremetal Operator, but since step 1 already does it, we can
   skip it and only install the operator. Depending on whether you want TLS, or
   basic-auth enabled, kustomize paths may differ. Check operator
   [dev-setup doc](https://github.com/metal3-io/baremetal-operator/blob/main/docs/dev-setup.md)
   for more info.

   ```shell
   git clone https://github.com/metal3-io/baremetal-operator.git
   kubectl create namespace baremetal-operator-system
   cd baremetal-operator
   kustomize build config/default | kubectl apply -f -
   ```

1. Install Ironic. There are a couple of ways to do it.
   - Run within a Kubernetes cluster as a pod, refer to the
     [deploy.sh](https://github.com/metal3-io/baremetal-operator/blob/main/tools/deploy.sh)
     script.
   - Outside of a Kubernetes cluster as a container. Please refer to the
     [run_local_ironic.sh](https://github.com/metal3-io/baremetal-operator/blob/main/tools/run_local_ironic.sh)
     script.

Please refer to the [getting-started](docs/getting-started.md) for more info.

## Pivoting ⚠️

Starting from `v0.5.0` release of Cluster API Provider Metal3, Baremetal
Operator is decoupled from Cluster API Provider Metal3 deployments when deployed
via `clusterctl`. For that reason, when performing `clusterctl move`, custom
objects outside of the Cluster API chain or not part of CAPM3 will not be
pivoted to a target cluster. An example of those objects is BareMetalHost, or
user created ConfigMaps and Secrets which are reconciled by Baremetal Operator.
To ensure that those objects are also pivoted as part of `clusterctl move`,
`clusterctl.cluster.x-k8s.io` label needs to be set on the BareMetalHost CRD
before pivoting. If there are other CRDs also need to be pivoted to the target
cluster, the same label needs to be set on them.

All the other objects owned by BareMetalHost, such as Secret and ConfigMap don't
require this label to be set, because they hold ownerReferences to
BareMetalHost, and that is good enough for clusterctl to move all the hierarchy
of BareMetalHost object.

## Development Environment

There are multiple ways to setup a development environment:

- [Using Tilt](docs/dev-setup.md#tilt-development-environment)
- [Other management cluster](docs/dev-setup.md#development-using-Kind-or-Minikube)
- See [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env) for an
  end-to-end development and test environment for `cluster-api-provider-metal3`
  and [baremetal-operator](https://github.com/metal3-io/baremetal-operator).

## API

See the [API Documentation](docs/api.md) for details about the objects used with
this Cluster API provider. You can also see the
[cluster deployment workflow](docs/deployment_workflow.md) for the outline of
the deployment process.

## Architecture

The architecture with the components involved is documented
[here](docs/architecture.md)

## E2E test

To trigger e2e test on a PR, use the following phrases:

### Integration tests

- **/test metal3-ubuntu-e2e-integration-test-main** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch main on Ubuntu
- **/test-centos-e2e-integration-test-main** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch main on CentOS

Release-1.10 branch:

- **/test metal3-ubuntu-e2e-integration-test-release-1-10** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.10 on Ubuntu
- **/test metal3-centos-e2e-integration-test-release-1-10** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.10 on CentOS

Release-1.9 branch:

- **/test metal3-ubuntu-e2e-integration-test-release-1-9** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.9 on Ubuntu
- **/test metal3-centos-e2e-integration-test-release-1-9** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.9 on CentOS

Release-1.8 branch:

- **/test metal3-ubuntu-e2e-integration-test-release-1-8** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.8 on Ubuntu
- **/test metal3-centos-e2e-integration-test-release-1-8** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.8 on CentOS

Release-1.7 branch:

- **/test metal3-ubuntu-e2e-integration-test-release-1-7** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.7 on Ubuntu
- **/test metal3-centos-e2e-integration-test-release-1-7** runs integration e2e
  tests with CAPM3 API version v1beta1 and branch release-1.7 on CentOS

## Basic tests

Unlike integration tests, basic tests focus on the target cluster creation
without involving pivoting from the bootstrap cluster. To run basic tests use:

- **/test metal3-ubuntu-e2e-basic-test-main** runs basic e2e tests with main
 branch on Ubuntu
- **/test metal3-centos-e2e-basic-test-release-1-10** runs basic e2e tests on
 release-1.10 branch with centos
- **/test metal3-centos-e2e-basic-test-release-1-9** runs basic e2e tests on
 release-1.9 branch with centos
- **/test metal3-centos-e2e-basic-test-release-1-8** runs basic e2e tests on
 release-1.8 branch with centos

### Feature tests

On main branch:

- **/test metal3-ubuntu-e2e-feature-test-main-pivoting** runs e2e pivot based
feature tests with CAPM3 API version v1beta1 and branch main on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-main-remediation** runs e2e remediation
 based feature tests with CAPM3 API version v1beta1 and branch main on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-main-features** runs e2e non pivot
 based feature tests with CAPM3 API version v1beta1 and branch main on Ubuntu
- **/test metal3-centos-e2e-feature-test-main-pivoting** runs e2e pivot based
 feature tests with CAPM3 API version v1beta1 and branch main on CentOS
- **/test metal3-centos-e2e-feature-test-main-remediation** runs e2e remediation
 based feature tests with CAPM3 API version v1beta1 and branch main on CentOS
- **/test metal3-centos-e2e-feature-test-main-features** runs e2e non pivot based
 feature tests with CAPM3 API version v1beta1 and branch main on CentOS

Release-1.10 branch:

- **/test metal3-ubuntu-e2e-feature-test-release-1-10-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.10
 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-10-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.10 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-10-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch release-1.10
  on Ubuntu
- **/test metal3-centos-e2e-feature-test-release-1-10-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.10 on
 CentOS
- **/test metal3-centos-e2e-feature-test-release-1-10-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.10 on CentOS
- **/test metal3-centos-e2e-feature-test-release-1-10-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch
 release-1.10 on CentOS

Release-1.9 branch:

- **/test metal3-ubuntu-e2e-feature-test-release-1-9-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.9
 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-9-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.9 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-9-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch release-1.9
  on Ubuntu
- **/test metal3-centos-e2e-feature-test-release-1-9-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.9 on
 CentOS
- **/test metal3-centos-e2e-feature-test-release-1-9-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.9 on CentOS
- **/test metal3-centos-e2e-feature-test-release-1-9-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch
 release-1.9 on CentOS

Release-1.8 branch:

- **/test metal3-ubuntu-e2e-feature-test-release-1-8-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.8
 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-8-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.8 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-8-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch release-1.8
  on Ubuntu
- **/test metal3-centos-e2e-feature-test-release-1-8-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.8 on
 CentOS
- **/test metal3-centos-e2e-feature-test-release-1-8-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.8 on CentOS
- **/test metal3-centos-e2e-feature-test-release-1-8-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch
 release-1.8 on CentOS

Release-1.7 branch:

- **/test metal3-ubuntu-e2e-feature-test-release-1-7-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.7
 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-7-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.7 on Ubuntu
- **/test metal3-ubuntu-e2e-feature-test-release-1-7-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch release-1.7
  on Ubuntu
- **/test metal3-centos-e2e-feature-test-release-1-7-pivoting** runs e2e pivot
 based feature tests with CAPM3 API version v1beta1 and branch release-1.7 on
 CentOS
- **/test metal3-centos-e2e-feature-test-release-1-7-remediation** runs e2e
 remediation based feature tests with CAPM3 API version v1beta1 and branch
 release-1.7 on CentOS
- **/test metal3-centos-e2e-feature-test-release-1-7-features** runs e2e non
 pivot based feature tests with CAPM3 API version v1beta1 and branch
 release-1.7 on CentOS

### Upgrade tests

#### Clusterctl upgrade tests

CAPM3 tests upgrade from all supported release to the current one.
We run upgrade test on main branch from different releases:

- **/test metal3-e2e-clusterctl-upgrade-test-main** runs e2e clusterctl
  upgrade tests on main with Ubuntu

- **/test metal3-e2e-clusterctl-upgrade-test-release-1-10** runs e2e clusterctl
  upgrade tests on release-1.10 with Ubuntu

- **/test metal3-e2e-clusterctl-upgrade-test-release-1-9** runs e2e clusterctl
  upgrade tests on release-1.9 with Ubuntu

- **/test metal3-e2e-clusterctl-upgrade-test-release-1-8** runs e2e clusterctl
  upgrade tests on release-1.8 with Ubuntu

- **/test metal3-e2e-clusterctl-upgrade-test-release-1-7** runs e2e clusterctl
  upgrade tests on release-1.7 with Ubuntu

#### K8s upgrade tests

CAPM3 tests upgrading kubernetes between last 3 releases.
The trigger takes the format:
`/test metal3-e2e-<from-minor-k8s-v>-<to-minor-k8s-v>-upgrade-test-<branch>`

- **/test metal3-e2e-1-29-1-30-upgrade-test-main**
- **/test metal3-e2e-1-28-1-29-upgrade-test-main**
- **/test metal3-e2e-1-27-1-28-upgrade-test-main**
- **/test metal3-e2e-1-31-1-32-upgrade-test-release-1-9**
- **/test metal3-e2e-1-29-1-30-upgrade-test-release-1-8**
- **/test metal3-e2e-1-29-1-30-upgrade-test-release-1-7**

Note:

- Triggers follow the pattern: `/test metal3-<image-os>-e2e-<test-type>-test-<branch>`

More info about e2e test can be found [here](docs/e2e-test.md)
