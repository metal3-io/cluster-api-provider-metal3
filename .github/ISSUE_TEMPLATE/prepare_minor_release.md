---
name: Minor release tracking issue
title: Prepare for v<release-tag>
---

Please see this documentation for more details baout release process: https://github.com/metal3-io/metal3-docs/blob/main/processes/releasing.md


**Note**:
* The following is based on the v1.7 minor release. Modify according to the tracked minor release.

## Tasks
* [ ] Uplift CAPI to latest minor in CAPM3, also check migration guide: [Migration guide for providers](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/book/src/developer/providers/migrations). [Prior art CAPM3](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1604).
IMPORTANT: Always read migration guide and make sure to do the changes accordingly in CAPM3 and IPAM.
* [x] Uplift k8s to 1.31 in CAPM3 and metal3-dev-env repo. [Prior art metal3-dev-env](https://github.com/metal3-io/metal3-dev-env/pull/1389), [Prior art CAPM3](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1622)
* [x] Uplift k8s to 1.31 in image building pipeline. [Prior art jjb](https://gerrit.nordix.org/c/infra/cicd/+/22205) [Prior art pipeline](https://github.com/metal3-io/project-infra/pull/836)
* [ ] Wait for Ironic release. It is tentative depending on when upstream Ironic has bugfix branch available. [Prior art](https://github.com/metal3-io/ironic-image/tags)
* [ ] Wait for BMO release if needed.
* [ ] Wait for IPAM release if needed.
* [ ] Bump IPAM and BMO in CAPM3 [Prior art](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1605)
* [ ] Release CAPM3 (Branch out, add branch protection and required tests, also check new image is created in quay).
* [ ] Prepare dev-env to accommodate new release related changes and configurations [Prior art](https://github.com/metal3-io/metal3-dev-env/pull/1381)
* [ ] Add/edit JJBs to accommodate new build jobs and PR jobs for new release branches. [Prior art](https://gerrit.nordix.org/c/infra/cicd/+/20875)
* [ ] Add Prow jobs for new release branches in Project-infra repo [Prior art](https://github.com/metal3-io/project-infra/pull/697)
* [ ] Check CI if new releases are tested properly or not.
* [ ] Update CAPM3 README.md with the new e2e triggers. [Prior art](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1618)
* [ ] Update Metal3 book. [Prior art](https://github.com/metal3-io/metal3-docs/pull/413)
* [ ] Announce the releases

## Post-release tasks
Do the following on the main branch after the release is done.

* [ ] Add CAPI contract for new minor release. [Prior art](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1937/files)
* [ ] Update clusterctl tests to accommodate new release.
* [ ] Update k8s uplift tests to accommodate new release.