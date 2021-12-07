# Build Apptainer

## Summary

This is a build container that generates installable apptainer packages for
apptainer v3.X.X. The container will output a deb and rpm in the current
directory.

## Known Bugs

Some versions of apptainer contain the character 'v', such as v3.0.0. The
container will have to be rebuilt with the following statement modified:

```sh
curl -L -o apptainer-${VERSION}.tar.gz
    https://github.com/apptainer/apptainer/releases/download/v${VERSION}/apptainer-${VERSION}.tar.gz
```

## Usage

```sh
sudo apptainer build build-apptainer.sif build-singularity.def

./build-apptainer.sif {version}

./build-apptainer.sif 3.8.0
```
