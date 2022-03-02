# Release Procedure

The release procedure below can be performed by a project member with
"maintainer" or higher privileges on the GitHub repository. It assumes
that you will be working in an up-to-date local clone of the GitHub
repository, where the `upstream` remote points to
`github.com/apptainer/apptainer`.

## Prior to Release

1. Set a target date for the release candidate (if required) and release.
   Generally 2 weeks from RC -> release is appropriate for new 1.X.0 minor
   versions.
1. Aim to specifically discuss the release timeline and progress in community
   meetings at least 2 months prior to the scheduled date.
1. Use a GitHub milestone to track issues and PRs that will form part of the
   next release.
1. Ensure that the `CHANGELOG.md` is kept up-to-date on the `main` branch,
   with all relevant changes listed under a "Changes Since Last Release"
   section.
1. Monitor and merge dependabot updates, such that a release is made with as
   up-to-date versions of dependencies as possible. This lessens the burden in
   addressing patch release fixes that require dependency updates, as we use
   several dependencies that move quickly.

## Creating the Release Branch and Release Candidate

When a new 1.Y.0 minor version of Apptainer is issued the release
process begins by branching, and then issuing a release candidate for
broader testing.

When a new 1.Y.Z patch release is issued, the branch will already be present,
and steps 1-2 should be skipped.

1. From a repository that is up-to-date with main, create a release
   branch e.g. `git checkout upstream/main -b release-1.0`.
1. Push the release branch to GitHub via `git push upstream release-1.0`.
1. Examine the GitHub branch protection rules, to extend them to the
   new release branch if needed.  Also examine `.github/dependabot.yml`
   to see if the new branch should be added there.
1. Modify the `README.md`, `INSTALL.md`, `CHANGELOG.md` via PR against
   the release-1.Y branch, so that they reflect the version to be released.
   1. Apply an annotated tag via `git tag -a -m "Apptainer v1.0.0
      Release Candidate 1" v1.0.0-rc.1`.
1. Push the tag via `git push upstream v1.0.0-rc.1`.
1. Create a GitHub release, marked as a 'pre-release', incorporating
   `CHANGELOG.md` information.  A tarball, rpm packages, deb packages,
   and a `sha256sums` should get automatically attached.
1. Notify the community about the RC via the Google Group and Slack.

There will often be multiple release candidates issued prior to the final
release of a new 1.Y.0 minor version.

A small 1.Y.Z patch release may not require release candidates where the code
changes are contained, confirmed by the person reporting the bug(s), and well
covered by tests.

## Creating a Final Release

1. Ensure the user and admin documentation is up-to-date for the new
   version, branched, and tagged.
   - [User Docs](https://apptainer.org/docs/user/main/) can be
     edited [here](https://github.com/apptainer/apptainer-userdocs).
     Be sure that the `apptainer_source` submodule is up to date by
     doing the following commands followed by making an update with
     a pull request:
      - `git submodule deinit -f .`
      - `git submodule update --init`
      - `cd apptainer_source`
      - `git checkout main`
      - `cd ..`
      - `git add apptainer_source`
   - [Admin Docs](https://apptainer.org/docs/admin/main/) can be
     edited [here](https://github.com/apptainer/apptainer-admindocs)
   - If a new branch was created, add it to the docsVersion list in the
     [web page](https://github.com/apptainer/apptainer.org/blob/master/src/pages/docs.js)
1. Ensure the user and admin documentation has been deployed to the
   apptainer.org website.
1. Modify the `README.md`, `INSTALL.md`, `CHANGELOG.md` via PR against
   the release-1.Y branch, so that they reflect the version to be released.
1. Apply an annotated tag via `git tag -a -m "Apptainer v1.0.0" v1.0.0`.
1. Push the tag via `git push upstream v1.0.0`.
1. Create a GitHub release, incorporating `CHANGELOG.md` information.
1. Notify the community about the RC via the Google Group and Slack.

## After the Release

1. Create and merge a PR from the `release-1.x` branch into `main`, so that
   history from the RC process etc. is captured on `main`.
1. If the release is a new major/minor version, move the prior `release-1.x`
   branch to `vault/release-1.x`.
1. If the release is a new major/minor version, update the
   `.github/dependabot.yml` configuration so that dependabot is tracking the new
   stable release branch.
1. Start scheduling / setting up milestones etc. to track the next release!
