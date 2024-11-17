# Test Corpus

## Images

Various SIF images are available for testing in the [images](images/) directory:

- [empty.sif](images/empty.sif): image with no data objects.
- [one-group.sif](images/one-group.sif): image with two data objects in one
  object group.
- [one-group-signed-dsse.sif](images/one-group-signed-dsse.sif): image with a
  DSSE envelope containing Ed25519 and RSA signatures.
- [one-group-signed-legacy-all.sif](images/one-group-signed-legacy-all.sif):
  image with a legacy `--all` signature.
- [one-group-signed-legacy-group.sif](images/one-group-signed-legacy-group.sif):
  image with a legacy `--groupid` signature.
- [one-group-signed-legacy.sif](images/one-group-signed-legacy.sif): image with
  a legacy signature.
- [one-group-signed-pgp.sif](images/one-group-signed-pgp.sif): image with a PGP
  signature.

The above images originated from the
[SIF test imagecorpus](https://github.com/apptainer/sif/tree/main/test/images).
When adding new images, please consider updating
[gen_sifs.go](https://github.com/apptainer/sif/blob/main/test/images/gen_sifs.go)
in the SIF repository.

## Keys

Cryptographic key pairs using various algorithms are available for testing in
the [keys](keys/) directory:

- ECDSA: [ecdsa-public.pem](keys/ecdsa-public.pem)/
  [ecdsa-private.pem](keys/ecdsa-private.pem)
- Ed25519: [ed25519-public.pem](keys/ed25519-public.pem)/
  [ed25519-private.pem](keys/ed25519-private.pem)
- RSA: [rsa-public.pem](keys/rsa-public.pem)/
  [rsa-private.pem](keys/rsa-private.pem)

The above key pairs originated from the
[SIF test key corpus](https://github.com/apptainer/sif/tree/main/test/keys). When
adding new key pairs, please consider updating
[gen_keys.go](https://github.com/apptainer/sif/blob/main/test/keys/gen_keys.go) in
the SIF repository.

A PGP key pair is also available for testing:

- [pgp-public.asc](keys/pgp-public.asc)/
  [pgp-private.asc](keys/pgp-private.asc)
