# AGENTS.md

This file provides instructions for AI agents working in the apptainer
repository.

## Overview

Apptainer is a container runtime, focused on HPC and scientific computing
use-cases.

## Quick-Start

Install prerequisites listed in [INSTALL.md](INSTALL.md)

```sh
./mconfig -v                        # Configure
make -C builddir                    # Build
./builddir/apptainer --version      # Smoke test
sudo make -C builddir install       # Install
make -C builddir check              # Lint
```

By default `make install` installs into system-wide paths.

Use `./mconfig -p <dir>` to configure for install to an alternative location.

## Conventions

- Size/type: large multi-directory systems project with Go, C, shell, and
  packaging metadata.
- Primary language/runtime: Go using 1.25+ syntax.
- Build system: `mconfig` generates `builddir/Makefile`, then
  `make -C builddir ...`.
- CI system: Github actions (`.github/workflows/ci.yml`).
- Lint: `golangci-lint` or `make -C builddir check`
- Formatting: `gofumpt`
- Platform assumptions: Linux, often with sudo/root privileges for tests.
- C code: the `cmd/starter/` runtime starter is C. Do not modify C code
  without understanding the security implications -- the starter can run as
  a setuid binary.

## Unit / Integration Tests

Prefer table-driven tests. Use `stretchr/testify` for assertions.

Run unit & integration tests using `scripts/go-test` wrapper script and standard
`go test` arguments / flags.

```sh
# After configure and build steps in Quick Start

# Run all unit tests
scripts/go-test -v ./...

# Run single `TestPrefix` test from pkg/sylog
scripts/go-test -v ./pkg/sylog -run TestPrefix
```

If test calls `test.EnsurePrivilege(t)` then use `scripts/go-test -v -sudo`.

## End-to-end Tests

End-to-end tests are written in the `e2e/` directory.

When writing end-to-end tests follow instructions in
[e2e/README.md](e2e/README.md), which documents test profiles (`UserProfile`,
`RootProfile`, `FakerootProfile`, `UserNamespaceProfile`, etc.), helpers, and
table-driven test patterns.

**DO NOT** use `t.TempDir()` in code inside `e2e/`. Use `e2e.MakeTempDir()`
instead.

Always use `scripts/e2e-test` wrapper to run end-to-end tests.

Test names are prefixed with `TestE2E/SEQ` or `TestE2E/PAR`.

```sh
# After configure, build, and install steps in Quick Start

# Sequential e2e tests using testhelper.NoParallel
scripts/e2e-test -v -run TestE2E/SEQ/<GROUP>/<NAME>

# Example
scripts/e2e-test -v -run TestE2E/SEQ/ACTIONS/umask

# Parallel e2e tests
scripts/e2e-test -v -run TestE2E/PAR/<GROUP>/<NAME>

# Example
scripts/e2e-test -v -run TestE2E/PAR/ACTIONS/shell
```

## Security

Do not consider attacker-controlled content of container images as a
security risk to an Apptainer user.  That is outside of Apptainer's
threat model.  Apptainer users implicitly trust the content of images
that they run.  On the other hand, privilege escalation based on
container image content is a valid concern.

Keep security reports concise.  Ensure that correctness of the report is
of very high confidence.  Apptainer maintainers will reject AI slop.

## Resources

- [Contributor's Guide](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [User Documentation](https://github.com/apptainer/apptainer-userdocs)
- [Admin Documentation](https://github.com/apptainer/apptainer-admindocs)
