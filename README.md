# Cluster API Provider Metal3 for Managed Bare Metal Hardware

[![Ubuntu V1alpha5 build status](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a5_integration_test_ubuntu/badge/icon?subject=Ubuntu%20E2E%20V1alpha5)](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a5_integration_test_ubuntu/)
[![CentOS V1alpha5 build status](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a5_integration_test_centos/badge/icon?subject=CentOS%20E2E%20V1alpha5)](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a5_integration_test_centos/)

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
| v1alpha4      | v1alpha3            | v0.4.X        |
| v1alpha5      | v1alpha4            | v0.5.X        |

You can find information on how to use this provider with Cluster API and
clusterctl in the [getting-started](docs/getting-started.md).

## Deploying the metal3 provider

The recommended method is using
[Clusterctl](https://master.cluster-api.sigs.k8s.io/clusterctl/overview.html).
Please refer to the [getting-started](docs/getting-started.md) for the
pre-requisites.

## Development Environment

There are multiple ways to setup a development environment:

* [Using Tilt](docs/dev-setup.md#tilt-development-environment)
* [Other management cluster](docs/dev-setup.md#development-using-Kind-or-Minikube)
* See [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env) for an
  end-to-end development and test environment for
  `cluster-api-provider-metal3` and
  [baremetal-operator](https://github.com/metal3-io/baremetal-operator).

## API

See the [API Documentation](docs/api.md) for details about the objects used with
this `cluster-api` provider. You can also see the [cluster deployment
workflow](docs/deployment_workflow.md) for the outline of the
deployment process.

## Architecture

The architecture with the components involved is documented [here](docs/architecture.md)
