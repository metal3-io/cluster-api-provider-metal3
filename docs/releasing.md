# cluster-api-provider-metal3 releasing

This document details the steps to create a release for
`cluster-api-provider-metal3` aka CAPM3.

**NOTE**: Always follow
[release documentation from the main branch](https://github.com/metal3-io/cluster-api-provider-metal3/blob/main/docs/releasing.md).
Release documentation in release branches may be outdated.

## Before making a release

Things you should check before making a release:

- Check the
  [Metal3 release process](https://github.com/metal3-io/metal3-docs/blob/main/processes/releasing.md)
  for high-level process and possible follow-up actions
- Use the `./hack/verify-release.sh` script as helper to identify possible
  issues to be addressed before creating any release tags. It verifies issues
  like:
   - Verify CAPI go module is uplifted in root, `api/` and `test/` go modules and
    `cluster-api/test` module in `test/` go module. Prior art:
    [#1157](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1157)
   - Verify controller Go modules use latest corresponding CAPI modules. Prior art:
     [#1145](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1145)
   - Verify BMO's `apis` and `pkg/hardwareutils` dependencies are the latest. Prior
     art:
     [#1163](https://github.com/metal3-io/cluster-api-provider-metal3/pull/1163)
   - Uplift IPAM `api` dependency
     . Prior art:
     [#999](https://github.com/metal3-io/cluster-api-provider-metal3/pull/999)
   - Verify any other direct or indirect dependency is uplifted to close any
     public vulnerabilities

## Permissions

Creating a release requires repository `write` permissions for:

- Tag pushing
- Branch creation
- GitHub Release publishing

These permissions are implicit for the org admins and repository admins.
Release team member gets his/her permissions via `metal3-release-team`
membership. This GitHub team has the required permissions in each repository
required to release CAPM3. Adding person to the team gives him/her the necessary
rights  in all relevant repositories in the organization. Individual persons
should not be given permissions directly.

## Process

CAPM3 uses [semantic versioning](https://semver.org).

- Regular releases: `v1.x.y`
- Beta releases: `v1.x.y-beta.z`
- Release candidate releases: `v1.x.y-rc.z`

### Repository setup

- Clone the repository:
`git clone git@github.com:metal3-io/cluster-api-provider-metal3`

or if using existing repository, make sure origin is set to the fork and
upstream is set to `metal3-io`. Verify if your remote is set properly or not
by using following command `git remote -v`.

- Fetch the remote (`metal3-io`): `git fetch upstream`
This makes sure that all the tags are accessible.

### Creating Release Notes

- Switch to the main branch: `git checkout main`

- Create a new branch for the release notes**:
  `git checkout -b release-notes-1.x.x origin/main`

- Generate the release notes: `RELEASE_TAG=v1.x.x make release-notes`
   - Replace `v1.x.x` with the new release tag you're creating.
   - This command generates the release notes here
     `releasenotes/<RELEASE_TAG>.md` .

- Next step is to clean up the release note manually.
   - If release is not a beta or release candidate, check for duplicates,
     reverts, and incorrect classifications of PRs, and whatever release
     creation tagged to be manually checked.
   - For any superseded PRs (like same dependency uplifted multiple times, or
     commit revertion) that provide no value to the release, move them to
     Superseded section. This way the changes are acknowledged to be part of the
     release, but not overwhelming the important changes contained by the
     release.

- Commit your changes, push the new branch and create a pull request:
   - The commit and PR title should be 🚀 Release v1.x.y:
      - `git commit -S -s -m ":rocket: Release v1.x.x"`
      - `git push -u origin release-notes-1.x.x`
   - Important! The commit should only contain the release notes file, nothing
     else, otherwise automation will not work.

- Ask maintainers and release team members to review your pull request.

Once PR is merged following GitHub actions are triggered:

- GitHub action `Create Release` runs following jobs:
   - GitHub job `push_release_tags` will create and push the tags. This action
     will also create release branch if its missing and release is `rc` or
     minor.
   - GitHub job `create draft release` creates draft release. Don't publish the
     release until release tag is visible in. Running actions are visible on the
     [Actions](https://github.com/metal3-io/cluster-api-provider-metal3/actions)
     page, and draft release will be visible on top of the
     [Releases](https://github.com/metal3-io/cluster-api-provider-metal3/releases).
     If the release you're making is not a new major release, new minor release,
     or a new patch release from the latest release branch, uncheck the box for
     latest release. If it is a release candidate (RC) or a beta release,
     tick pre-release box.
   - GitHub job `build_CAPM3` builds release image with the release tag,
     and pushes it to Quay. Make sure the release tag is visible in
     [Quay tags page](https://quay.io/repository/metal3-io/cluster-api-provider-metal3?tab=tags).
     If the release tag build is not visible, check if the action has failed and
     retrigger as necessary.

### Release artifacts

We need to verify all release artifacts are correctly built or generated by
the release workflow.

We can use `./hack/verify-release.sh` to check for existence of release artifacts,
which should include the following:

Git tags pushed:

- Primary release tag: `v1.x.y`
- Go module tags: `api/v1.x.y`, `test/v1.x.y`

Container images at Quay registry:

- [cluster-api-provider-metal3:v1.x.y](https://quay.io/repository/metal3-io/cluster-api-provider-metal3?tab=tags)

Files included in the release page:

- A manifest file - `infrastructure-components.yaml`
- A metadata file - `metadata.yaml`
- A cluster template - `cluster-template.yaml`
- A file containing an example of variables to set - `example_variables.rc`

### Make the release

After everything is checked out, hit the `Publish` button your GitHub draft
release!

## Post-release actions for new release branches

Some post-release actions are needed if new minor or major branch was created.

### Dependabot configuration

In `main` branch, Dependabot configuration must be amended to allow updates
to release branch dependencies and GitHub Workflows.

If project dependencies or modules have not changed, previous release branch
configuration can be copied and amend the `target-branch` to point to our new
release branch. Release branches that are End-of-Life should be removed in the
same PR, as updating `dependabot.yml` causes Dependabot to run the rules,
ignoring the configured schedules, causing unnecessary PR creation for EOL
branches.

If project dependencies have changed, then copy the configuration of `main`,
and adjust the `ignore` rules to match release branches. As generic rule we
don't allow major or minor bumps in release branches.

[Prior art](https://github.com/metal3-io/cluster-api-provider-metal3/pull/2527)

### Branch protection rules

Branch protection rules need to be applied to the new release branch. Copy the
settings after the previous release branch, with the exception of
`Required tests` selection. Required tests can only be selected after new
keywords are implemented in Jenkins JJB, and in project-infra, and have been run
at least once in the PR targeting the branch in question.

NOTE: Branch protection rules need repository `admin` rights to modify.
Organization admins and security team members can help you.

### Update README.md and build badges

Update `README.md` with release specific information, both on `main` and
in the new `release-1.x` branch as necessary.

[Example](https://github.com/metal3-io/cluster-api-provider-metal3/pull/949)

In the `release-1.x` branch, update the build badges in the `README.md` to point
to correct Jenkins jobs, so the build statuses of the release branch are
visible.

[Example](https://github.com/metal3-io/cluster-api-provider-metal3/pull/951)

## Additional actions outside this repository

Further additional actions are required in the Metal3 project after CAPM3
release. For that, please continue following the instructions provided in
[Metal3 release process](https://github.com/metal3-io/metal3-docs/blob/main/processes/releasing.md)
