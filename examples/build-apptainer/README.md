# Build Apptainer

## Summary

This is a build container that generates installable packages for
apptainer v1.x.x. The container will output deb and rpm packages
in the current directory.

## Usage

```sh
sudo apptainer build build-apptainer.sif build-apptainer.def

./build-apptainer.sif {version}

./build-apptainer.sif 1.0.0
```
