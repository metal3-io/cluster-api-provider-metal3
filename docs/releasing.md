# Releasing

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Prerequisites](#prerequisites)
  - [`docker`](#docker)
- [Output](#output)
  - [Expected artifacts](#expected-artifacts)
  - [Artifact locations](#artifact-locations)
- [Process](#process)
  - [Permissions](#permissions)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prerequisites

### `docker`

You must have docker installed.

## Output

### Expected artifacts

1. A container image of the shared cluster-api-provider-baremetal manager
1. A git tag
1. A release on Github containing:
    - A manifest file - `infrastructure-components.yaml`
    - A metadata file - `metadata.yaml`
    - A cluster template - `cluster-template.yaml`

### Artifact locations

1. The container image is found in the registry `quay.io/metal3-io` with an image
   name of `cluster-api-provider-baremetal` and a tag that matches the release
   version. The image is automatically built once the release has been created.

## Process

For version v0.x.y:

1. Create the release notes `make release-notes`. Copy the output and sort
   manually the items that need to be sorted.
1. Create an annotated tag `git tag -a v0.x.y -m v0.x.y`. To use your GPG
   signature when pushing the tag, use `git tag -s [...]` instead
1. Push the tag to the GitHub repository `git push origin v0.x.y`
   NB: `origin` should be the name of the remote pointing to
   `github.com/metal3-io/cluster-api-provider-baremetal`
1. Run `make release` to build artifacts (the image is automatically built by CI)
1. Run `make release` to build artifacts
1. [Create a release in GitHub](https://help.github.com/en/github/administering-a-repository/creating-releases)
   that contains the elements listed above that have been created in the `out`
   folder

### Permissions

Releasing requires a particular set of permissions.

- Tag push access to the GitHub repository
- GitHub Release creation access
