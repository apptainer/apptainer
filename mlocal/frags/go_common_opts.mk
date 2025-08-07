# go tool default build options
GO_TAGS := containers_image_openpgp sylog oci_engine apptainer_engine fakeroot_engine
GO_TAGS_SUID := containers_image_openpgp sylog apptainer_engine fakeroot_engine
GO_LDFLAGS :=
# Need to use non-pie build on ppc64le
# https://github.com/apptainer/singularity/issues/5762
# Need to disable race detector on ppc64le
# https://github.com/apptainer/singularity/issues/5914
uname_m := $(shell uname -m)
ifeq ($(uname_m),ppc64le)
GO_BUILDMODE := -buildmode=default
GO_RACE :=
else
GO_BUILDMODE := -buildmode=pie
GO_RACE := -race
endif
# -trimpath is necessary for plugin compiles, but it interferes with
# the generation of debugsource rpms.  So include it by default, but
# override SUPPORT_PLUGINS to empty when building an rpm.  Plugins aren't
# usable with rpms anyway because they have to be compiled from the exact
# source directory that apptainer is originally compiled with.
SUPPORT_PLUGINS := true
GO_MODFLAGS := $(if $(SUPPORT_PLUGINS),-trimpath)
GO_MODFILES := $(SOURCEDIR)/go.mod $(SOURCEDIR)/go.sum
GOPROXY := https://proxy.golang.org

export GOPROXY
