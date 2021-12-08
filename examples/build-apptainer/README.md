# Build Apptainer

## Summary

This is a build container that generates installable apptainer packages for
apptainer v1.X.X. The container will output a deb and rpm in the current
directory.

## Usage

```sh
sudo apptainer build build-apptainer.sif build-singularity.def

./build-apptainer.sif {version}

./build-apptainer.sif 1.0.0
```
