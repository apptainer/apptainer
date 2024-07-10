# Apptainer

[![CI](https://github.com/apptainer/apptainer/actions/workflows/ci.yml/badge.svg)](https://github.com/apptainer/apptainer/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/pkg.golang.dev/github.com/apptainer/apptainer.svg)](https://pkg.go.dev/github.com/apptainer/apptainer)
[![Go Report Card](https://goreportcard.com/badge/github.com/apptainer/apptainer)](https://goreportcard.com/report/github.com/apptainer/apptainer)

- [Documentation](https://apptainer.org/docs/)
- [Support](#support)
- [Community Meetings / Minutes / Roadmap](https://drive.google.com/drive/u/0/folders/1npfBhIDxqeJIUHZ0tMeuHPvc_iB4T2B6)
- [Project License](LICENSE.md)
- [Guidelines for Contributing](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Citation](#citing-apptainer)

## What is Apptainer?

Apptainer is an open source container platform designed to be simple, fast,
and secure. Many container platforms are available, but Apptainer is designed
for ease-of-use on shared systems and in high performance computing (HPC)
environments. It features:

- An immutable single-file container image format, supporting cryptographic
  signatures and encryption.
- Integration over isolation by default. Easily make use of GPUs, high speed
  networks, parallel filesystems on a cluster or server.
- Mobility of compute. The single file SIF container format is easy to transport
  and share.
- A simple, effective security model. You are the same user inside a container
  as outside, and cannot gain additional privilege on the host system by
  default.

Apptainer is open source software, distributed under the [BSD License](LICENSE.md).

Apptainer was formerly known as Singularity and is now [a part of the
Linux Foundation](https://apptainer.org/news/community-announcement-20211130).
When migrating from Singularity see the
[admin migration documentation](https://apptainer.org/docs/admin/main/singularity_migration.html)
and
[user compatibility documentation](https://apptainer.org/docs/user/main/singularity_compatibility.html).

Check out [talks about Apptainer](https://apptainer.org/talks)
and some [use cases of Apptainer](https://apptainer.org/usecases)
on our website.

## Getting Started with Apptainer

To install Apptainer from source, see the [installation
instructions](INSTALL.md). For other installation options, see [our
guide](https://apptainer.org/docs/admin/main/installation.html).

System administrators can learn how to configure Apptainer, and get an
overview of its architecture and security features in the [administrator
guide](https://apptainer.org/docs/admin/main/).

For users, see the [user guide](https://apptainer.org/docs/user/main/)
for details on how to run and build containers with Apptainer.

## Contributing to Apptainer

Community contributions are always greatly appreciated. To start developing
Apptainer, check out the [guidelines for contributing](CONTRIBUTING.md).

Please note we have a [code of conduct](CODE_OF_CONDUCT.md). Please follow it in
all your interactions with the project members and users.

Our roadmap, other documents, and user/developer meeting information can be
found in the [apptainer community page](https://apptainer.org/help).

We also welcome contributions to our [user
guide](https://github.com/apptainer/apptainer-userdocs) and [admin
guide](https://github.com/apptainer/apptainer-admindocs).

## Support

To get help with Apptainer, check out the [Apptainer
Help](https://apptainer.org/help) web page.

## Go Version Compatibility

Apptainer aims to maintain support for the two most recent stable versions
of Go. This corresponds to the Go
[Release Maintenance
Policy](https://github.com/golang/go/wiki/Go-Release-Cycle#release-maintenance)
and [Security Policy](https://golang.org/security),
ensuring critical bug fixes and security patches are available for all
supported language versions.

## Citing Apptainer

Apptainer can be cited using its former name Singularity.

The Singularity software may be cited using our Zenodo DOI `10.5281/zenodo.1310023`:

> Singularity Developers (2021) Singularity. 10.5281/zenodo.1310023
> <https://doi.org/10.5281/zenodo.1310023>

This is an 'all versions' DOI for referencing Singularity in a manner that is
not version-specific. You may wish to reference the particular version of
Singularity used in your work. Zenodo creates a unique DOI for each release,
and these can be found in the 'Versions' sidebar on the [Zenodo record page](https://doi.org/10.5281/zenodo.1310023).

Please also consider citing the original publication describing Singularity:

> Kurtzer GM, Sochat V, Bauer MW (2017) Singularity: Scientific containers for
> mobility of compute. PLoS ONE 12(5): e0177459.
> <https://doi.org/10.1371/journal.pone.0177459>

## License

_Unless otherwise noted, this project is licensed under a 3-clause BSD license
found in the [license file](LICENSE.md)._
