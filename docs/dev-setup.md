# Setting up a development environment

## Pre-requisites

CAPM3 requires two external tools for running the tests
during development.

### Install kustomize

```bash
./hack/ensure-kustomize.sh
```

### Install kubebuilder

```bash
./hack/tools/install_kubebuilder.sh
```

## Tilt development environment

Both of the [Tilt](https://tilt.dev) setups below will get you started
developing CAPM3 in a local kind cluster. The main difference is the number of
components you will build from source and the scope of the changes you'd like
to make.
If you only want to make changes in CAPM3, then follow
[CAPM3 instructions](#tilt-for-dev-in-capm3). This will save you from having to
build all of the images for CAPI, which can take a while. If the scope of your
development will span both CAPM3 and CAPI, then follow the
[CAPI and CAPM3 instructions](#tilt-for-dev-in-both-capm3-and-capi).

### Tilt for dev in CAPM3

In order to develop CAPM3 using Tilt, there is a requirement to have kind
and Tilt installed.

```sh
hack/ensure-kind.sh
curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash
```

Once installed, you will need a tilt-settings.json. You can generate one using

```sh
make tilt-settings
```

The tilt-settings file can be customized. Tilt will also consider everything
under `tilt.d` directory.

Then start Tilt:

```sh
make tilt-up
```

The Tilt dashboard can be accessed at <http://127.0.0.1:10350>. If tilt is running remotely, it can be accessed using ssh tunneling. An example of ```~/.ssh/config```  is show below.

```bash
Host tiltvm
  HostName <SERVER_IP>
  LocalForward 10350 127.0.0.1:10350
  User ubuntu
```

Changes in the go code will trigger an update of the images running to run the
latest code of CAPM3.

Tilt can be used in Metal3-dev-env, as long as the
03_launch_mgmt_cluster.sh script is **NOT** run. In that case Ironic needs to
be deployed locally first, following the
[local Ironic setup](https://github.com/metal3-io/baremetal-operator/blob/main/docs/dev-setup.md#running-a-local-instance-of-ironic)

By default, the Cluster API components deployed by Tilt have experimental
features turned off. If you would like to enable these features, add
`extra_args` as specified in
[The Cluster API Book](https://cluster-api.sigs.k8s.io/developer/tilt.html#create-a-tilt-settingsjson-file).

Once your kind management cluster is up and running, you can
[deploy an example cluster](#deploying-an-example-cluster).

You can also
[deploy a flavor cluster as a local tilt resource](#Running-flavor-clusters-as-a-tilt-resource).

To tear down the kind cluster built by the command above, just run:

```shell
make kind-reset
```

### Tilt for dev in both CAPM3 and CAPI

If you want to develop in both CAPI and CAPM3 at the same time, then this is
the path for you.

First, you will need a kind setup with a registry. You can use the following
command

```sh
    make kind-create
```

To use [Tilt](https://tilt.dev/) for a simplified development workflow, follow
the [instructions](https://cluster-api.sigs.k8s.io/developer/tilt.html) in the
cluster API repo.  The instructions will walk you through cloning the Cluster
API (CAPI) repository and configuring Tilt to use `kind` to deploy the cluster
api management components.

> you may wish to checkout out the correct version of CAPI to match the
[version used in CAPM3](go.mod)

Note that `tilt up` will be run from the `cluster-api repository` directory and
the `tilt-settings.json` file will point back to the
`cluster-api-provider-metal3` repository directory.  Any changes you make to the
source code in `cluster-api` or `cluster-api-provider-metal3` repositories will
automatically redeployed to the `kind` cluster.

After you have cloned both repositories, your folder structure should look like:

```tree
|-- src/cluster-api-provider-metal3
|-- src/cluster-api (run `tilt up` here)
```

Checkout a stable CAPI release, for example:

```bash
git checkout release-0.3
```

Install kustomize from within the cluster-api repo

```bash
make kustomize

# verify installation
./hack/tools/bin/kustomize version
```

If the above installation does not work, install kustomize using a package manager and move the binary to ```hack/tools/bin/kustomize``` in the cluster-api repo.

After configuring the environment variables, run the following to generate your
`tilt-settings.json` file in the cluster API repository:

```shell
cat <<EOF > tilt-settings.json
{
    "allowed_contexts": ["kind-capm3"],
    "default_registry": "gcr.io/cluster-api-provider",
    "deploy_cert_manager": true,
    "preload_images_for_kind": true,
    "kind_cluster_name": "capm3",
    "provider_repos": ["../cluster-api-provider-metal3"],
    "enable_providers": ["metal3", "docker", "kubeadm-bootstrap", "kubeadm-control-plane"],
    "kustomize_substitutions": {
        "DEPLOY_KERNEL_URL": "${DEPLOY_KERNEL_URL}",
        "DEPLOY_RAMDISK_URL": "${DEPLOY_RAMDISK_URL}",
        "IRONIC_INSPECTOR_URL": "${IRONIC_INSPECTOR_URL}",
        "IRONIC_URL": "${IRONIC_URL}"
  }
}
EOF
```

This requires the following environment variables to be exported :

* `DEPLOY_KERNEL_URL`
* `DEPLOY_RAMDISK_URL`
* `IRONIC_INSPECTOR_URL`
* `IRONIC_URL`

Those need to be set up accordingly with the
[local Ironic setup](https://github.com/metal3-io/baremetal-operator/blob/main/docs/dev-setup.md#running-a-local-instance-of-ironic)

The cluster-api management components that are deployed are configured at the
`/config` folder of each repository respectively. Making changes to those files
will trigger a redeploy of the management cluster components.

### Including Baremetal Operator and IP Address Manager

If you want to develop on Baremetal Operator or IP Address Manager repository
you can include them. First you need to clone the repositories you intend to
modify locally, then modify your tilt-settings.json to point to the correct
repositories. In case you have a tree like:

```bash
#-|
# |- cluster-api
# |- cluster-api-provider-metal3
# |- baremetal-operator
# |- ip-address-manager
```

and you create your `tilt-settings.json` in the ./cluster-api or in the
./cluster-api-provider-metal3 folder. Then your tilt-settings.json file would
be :

```json
{
  "provider_repos": [ ... , "../baremetal-operator", "../ip-address-manager"],
  "enable_providers": [ ... , "metal3-bmo", "metal3-ipam"],
}
```

The provider name for Baremetal Operator is `metal3-bmo` and for IP Address
Manager `metal3-ipam`. Those names are defined in their respective repository.

> In case you are modifying the manifests for BMO or IPAM, then you should also
modify the `kustomization.yaml` files in `./config/bmo` and `./config/ipam` to
refer to the modified manifests locally instead of downloading released ones.

### Running flavor clusters as a tilt resource

For a successful deployment, you need to have exported the proper variables.
You can use the following command:

```sh
    source ./examples/clusterctl-templates/example_variables.rc
```

However, the example variables do not guarantee a successful deployment, they
need to be adapted. If deploying on Metal3-dev-env, please rather use the
Metal3-dev-env deployment scripts that are tailored for it.

#### From CLI

run `tilt up ${flavors}` to spin up worker clusters represented by
a Tilt local resource.  You can also modify flavors while Tilt is up by running
`tilt args ${flavors}`

#### From Tilt Config

Add your desired flavors to tilt_config.json:

```json
{
    "worker-flavors": ["default"]
}
```

#### Requirements

Please note your tilt-settings.json must contain at minimum the following
fields when using tilt resources to deploy cluster flavors:

```json
{
  "kustomize_substitutions": {
    "DEPLOY_KERNEL_URL": "...",
    "DEPLOY_RAMDISK_URL": "...",
    "IRONIC_INSPECTOR_URL": "...",
    "IRONIC_URL": "..."
  }
}
```

#### Defining Variable Overrides

If you wish to override the default variables for flavor workers, you can
specify them as part of your tilt-settings.json as seen in the example
below.  Please note, the precedence of variables is as follows:

1. explicitly defined vars for each flavor i.e.
   `worker-templates.flavors[0].WORKER_MACHINE_COUNT`
1. vars defined at 'metadata' level-- spans workers i.e.
   `metadata.WORKER_MACHINE_COUNT`
1. programmatically defined default vars i.e. everything except Ironic
   related URL variables "DEPLOY_KERNEL_URL", "DEPLOY_RAMDISK_URL",
   "IRONIC_INSPECTOR_URL" and "IRONIC_URL"

```json
{
    "kustomize_substitutions": {
        "DEPLOY_KERNEL_URL": "...",
        "DEPLOY_RAMDISK_URL": "...",
        "IRONIC_INSPECTOR_URL": "...",
        "IRONIC_URL": "..."
    },
    "worker-templates": {
        "flavors": {
            "default": {
                "CLUSTER_NAME": "example-default-cluster-name",
            }
        },
        "metadata": {,
            "CONTROL_PLANE_MACHINE_COUNT": "1",
            "KUBERNETES_VERSION": "v1.23.3",
            "WORKER_MACHINE_COUNT": "2",
        }
    }
}
```

## Development using Kind or Minikube

See the [Kind docs](https://kind.sigs.k8s.io/docs/user/quick-start) for
instructions on launching a Kind cluster and the
[Minikube docs](https://kubernetes.io/docs/setup/minikube/) for
instructions on launching a Minikube cluster.

### Deploy CAPI and CAPM3

The following command will deploy the controllers from CAPI, CABPK and CAPM3 and
the requested CRDs and creates BareMetalHosts custom resources. The provider
uses the `BareMetalHost` custom resource defined by the `baremetal-operator`.

```sh
make deploy
```

When a `Metal3Machine` is created, the provider looks for an available
`BareMetalHost` object to claim and then sets it to be provisioned to fulfill the
request expressed by the `Metal3Machine`. There’s no requirement to actually run
the `baremetal-operator` to test the reconciliation logic of the provider.

Refer to the [baremetal-operator developer
documentation](https://github.com/metal3-io/baremetal-operator/blob/main/docs/dev-setup.md)
for instructions and tools for creating BareMetalHost objects.

### Run the Controller locally

You will first need to scale down the controller deployment:

```sh
kubectl scale -n capm3-system deployment.v1.apps/capm3-controller-manager --replicas 0
```

You can manually run the controller from outside of the cluster for development
and testing purposes. There’s a `Makefile` target which makes this easy.

```bash
make run
```

You can follow the output on the console to see information about what the
controller is doing. You can also proceed to create/update/delete
`Metal3Machines` and `BareMetalHosts` to test the controller logic.

### Deploy an example cluster

Make sure you run `make deploy` and wait until all pods are in `running` state
before deploying an example cluster with:

```sh
make deploy-examples
```

### Delete the example cluster

```sh
make delete-examples
```
