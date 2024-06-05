---
name: Minor release tracking issue
title: Prepare for v<release-tag>
---

Please see this documentation for more details baout release process: https://github.com/metal3-io/metal3-docs/blob/main/processes/releasing.md


**Note**:
* The following is based on the v1.7 minor release. Modify according to the tracked minor release.

## Tasks

* [ ] Add/edit JJBs to acommodate new build jobs and PR jobs for new release branches. [Prior art](https://gerrit.nordix.org/c/infra/cicd/+/20875)
* [ ] Add Prow jobs for new release branches in Project-infra repo [Prior art](https://github.com/metal3-io/project-infra/pull/697)
* [ ] Prepare dev-env to acommodate new release related changes and configurations [Prior art](https://github.com/metal3-io/metal3-dev-env/pull/1381)
* [ ] Update Metal3 book. [Prior art](https://github.com/metal3-io/metal3-docs/pull/413)
* [ ] Release Ironic, also check new image is created in quay. [Prior art](https://github.com/metal3-io/ironic-image/tags)
* [ ] Pin Ironic in BMO and release BMO, also check new image is created in quay. [Prior art](https://github.com/metal3-io/baremetal-operator/pull/1679)
* [ ] Release IPAM (Branch out, add branch protection and required tests, also check new image is created in quay).
* [ ] Bump IPAM and BMO in CAPM3[Prior art](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1605)
* [ ] Release CAPM3 (Branch out, add branch protection and required tests, also check new image is created in quay).
* [ ] Check CI if new releases are tested properly or not
* [ ] Update CAPM3,IPAM and BMO README with the new e2e triggers. [Prior art][CAPM3](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1618), [IPAM](https://github.com/metal3-io/ip-address-manager/pull/504),[BMO](https://github.com/metal3-io/baremetal-operator/pull/1694)
* [ ] Announce the releases
