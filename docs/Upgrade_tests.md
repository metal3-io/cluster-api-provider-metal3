# E2E Upgrade Tests

## Initial setup

The tests begins by importing helper functions and as setting up variables to ensure access to the proper versions of `capm3` and `IPAM` repos. `metal3-dev-env` is then cloned and set as the working directory. `dev-env` configuration details are then set. The `SKIP_NODE_IMAGE_PREPULL` flag is set to true to ensure `dev-env` does not download the node. The `SKIP_APPLY_BMH` flag is also set to true to ensure bare metal hosts are not automatically applied.

dev-env is then made to  is then used to set up the development environment
### Setting up Dev-env

`dev-env` starts by installing all the required dependencies, CLIs and host packages. It removes any old containers from previous tests, configures the registry and kernel to enable VMs. It also configures the host and network

It then then kills any old ironic instances.

It starts a bootstrap (or source) cluster (using either kind or minikube) and creates the metal3 namespace. It configures the clusterctl provider and initializes it which installs CAPI + CAPM3 + IPAM providers and controllers.

The CAPM3 provders are then launched followed by the BMOs and Ironic. After the BMOs are rolled out. BMH manifest files are created **but not applied**.

the script then verifies that all the expected CRDS, deployements and  containers exist and they are no issues.


### Finalizing the start 
`ci-e2e.sh` then ensures `kustomize` exists  and configures the image refreshes and updates them if necessary

The script then generates credentials for BMO and ironic overlays and update the BMO and Ironic images in `kustomization.yaml` files to use the same image that was used before pivot in the metal3-dev-env
``
It then creates or reuses username and password for any basic ironic authentication if necessary. It copies the TLS key/certificate and key into the overlay files.

Finally, it runs the go E2E upgrade tests.

They are 4 upgrade tests available.


## upgrade_kubernetes_test

This test aims to manually update both control panel and worker nodes.


It starts by creating BMH resources for the target.. A target cluster is then created with an old out dated version of kubernetes passed and a number of control panel and worker nodes. `KubeadmControlPlane` and `MachineDeployment` objects are also created to control the control panel and worker nodes.

The upgrade flow is then triggered.


The test finds the latest version of kubernets and downloads the image. It then creates a `Metal3MachineTemplate` for the `KubeadmControlPlane` and patches the `KubeadmControlPlane`  pointing the template towards the new template, setting it to use the latest version kubernetes. `maxSurge` is also to 0 to ensure  a strict roll-out with one-by-one in-place replacement.

The KCP controller notices the changes and starts a rolling replacement of control-plane machines that no longer match desired state. The test waits until one BMH move to a de-provisioning state and replacement control panel machines are provisioned and began to run on the newest version.
The test waits until all the machines are running and updated

The CP nodes then have their `NoSchedule` taints removed to enable easier scheduling for future steps and `maxsurge` is restored to 1.


The worker side upgrade then begins.

A new `Metal3MachineTemplate` is created for `MachineDeployment` which points towards the updated image. `MachineDeployment` is then patched to point towards the new template, and be upgraded to the newest version. `maxSurge` is set to 0, and  `maxUnavailable` is set to 1 to ensure a one by one roll-out where one machine is replaced at a time.

Then the worker replacement begins. Worker nodes are de-provisioned, updated to the latest version and then provisioned again. The test waits until all the machines are provisioned.


The test then verifies that all machines are running and upgraded and if so marks the test as a success and the upgrade logs are passed






## upgrade_kubernetes_n3_test


This test ensures multiple minor version updates can happen consecutively.



It starts by creating BMH resources for the target.. A target cluster is then created with kubernetes version `N0` passed and a number of control panel and worker nodes. `KubeadmControlPlane` and `MachineDeployment` objects are also created to control the control panel and worker nodes.

The upgrade flow is then triggered. and kubernetes versions `N0`,`N1`,`N2`, and `N3` are loaded and logged.

Then the first control plane upgrade begins from `N0` to `N1`

The test downloads the new image. It then creates a `Metal3MachineTemplate` for the `KubeadmControlPlane` and patches the `KubeadmControlPlane`  pointing the template towards the new template, and setting the new version kubernetes. `maxSurge` is also to 0 to ensure  a strict roll-out with one-by-one in-place replacement.

The KCP controller notices the changes and starts a rolling replacement of control-plane machines that no longer match desired state. The test waits until one BMH move to a de-provisioning state and replacement control panel machines are provisioned and began to run on the newest version.

The test waits until all the machines are running and updated . `maxSurge` is then set to 1 and the upgrades are verified.

While similar the the this process is performed using the `UpgradeControlPlane` function instead of manually.

This process is repeated twice from `N1` to `N2` and from `N2` to `N3`.




## k8s_in_place_upgrade_test


It starts by creating BMH resources for the target.. A target cluster is then created with an out-dated kubernetes version passed and a number of control panel and worker nodes. `KubeadmControlPlane` and `MachineDeployment` objects are also created to control the control panel and worker nodes.

An extrensionconifg is then applied which makes the extenstion server register in-place hooks `CanUpdateMachine`,`CanUpdateMachineSet`, and `UpdateMachine`. These hooks tell CAPI to update in place

The test downloads the new image. It then creates a `Metal3MachineTemplate` for the `KubeadmControlPlane` and patches the `KubeadmControlPlane`  pointing the template towards the new template, and setting the new version kubernetes. 

CAPI request for the updates to be done in place. The `Canupdate` hooks returns patch responses desciping fields which can be safely upgraded in-place. 

 **Todo: Clarify hooks use**
For each machine `upgradeKubernetesInPlace` is called byt the CAPI which upgrades the kudeadm first then applies it to the node. then it updates the kubeclet and kubelet and applies them is reloads the systemd daemon, then it restarts the kubelet to apply the new version

The test later checks the UUID of the machines to ensure no machines have been replaced

Then the cleanup occurs.



## Cleanup

cleanup is the same for all tests

All BMHs, Metal3 machines, machines and nodes are logged. Then any temporary log folders are deleted.
The workload and target cluster logs are collected.  All the cluster API resources in the namespace are dumped, and all clusters are deleted. The test waits until all `Metal3Data`, `Metal3DataTemplate`, `Metal3DataClaim` objects are gone. and then ends

**Note:** If skipCleanup is set to true the clusters are not deleted