# Build Apptainer

## Summary

This is a build container that generates installable packages for
apptainer v1.x.x. The container will output deb and rpm packages
in the current directory.

## Usage

```sh
apptainer build build-apptainer.sif build-apptainer.def

apptainer run build-apptainer.sif {version}

apptainer run build-apptainer.sif 1.4.0
```
