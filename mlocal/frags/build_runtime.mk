# This file contains all of the rules for building the apptainer runtime
#   and installing the necessary config files.

# contain starter_SOURCE variable list
starter_deps := $(BUILDDIR_ABSPATH)/starter.deps

-include $(starter_deps)

$(starter_deps): $(GO_MODFILES)
	@echo " GEN GO DEP" $@
	$(V)cd $(SOURCEDIR) && ./makeit/gengodep -v3 "$(GO)" "starter_SOURCE" "$(GO_TAGS)" "$@" "$(SOURCEDIR)/cmd/starter"

starter_CSOURCE := $(wildcard $(SOURCEDIR)/cmd/starter/c/*.c)
starter_CSOURCE += $(wildcard $(SOURCEDIR)/cmd/starter/c/include/*.h)

$(BUILDDIR_ABSPATH)/.clean-starter: $(starter_CSOURCE)
	@echo " GO clean -cache"
	-$(V)$(GO) clean -cache 2>/dev/null
	$(V)touch $@


# starter
# Look at dependencies file changes via starter_deps
# because it means that a module was updated.
starter := $(BUILDDIR_ABSPATH)/cmd/starter/c/starter
$(starter): $(BUILDDIR_ABSPATH)/.clean-starter $(apptainer_build_config) $(starter_deps) $(starter_SOURCE)
	@echo " GO" $@
	$(V)cd $(SOURCEDIR) && $(GO) build $(GO_MODFLAGS) $(GO_BUILDMODE) -tags "$(GO_TAGS)" $(GO_LDFLAGS) \
		-o $@ cmd/starter/main_linux.go

starter_INSTALL := $(DESTDIR)$(LIBEXECDIR)/apptainer/bin/starter
$(starter_INSTALL): $(starter)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0755 $(starter) $@

CLEANFILES += $(starter)
INSTALLFILES += $(starter_INSTALL)
ALL += $(starter)


# preload library for offsetting accesses into a file
offsetpreload := $(BUILDDIR_ABSPATH)/offsetpreload.so
offsetpreload_SOURCE := $(SOURCEDIR)/tools/offsetpreload.c
$(offsetpreload): $(offsetpreload_SOURCE)
	@echo " CC" $@
	$(V)$(CC) $(CFLAGS) -shared -o $@ $(offsetpreload_SOURCE) -ldl

offsetpreload_INSTALL := $(DESTDIR)$(LIBEXECDIR)/apptainer/lib/offsetpreload.so
$(offsetpreload_INSTALL): $(offsetpreload)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0755 $(offsetpreload) $@

CLEANFILES += $(offsetpreload)
INSTALLFILES += $(offsetpreload_INSTALL)
ALL += $(offsetpreload)


# sessiondir
sessiondir_INSTALL := $(DESTDIR)$(LOCALSTATEDIR)/apptainer/mnt/session
$(sessiondir_INSTALL):
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $@

INSTALLFILES += $(sessiondir_INSTALL)


# run-singularity script
run_singularity := $(SOURCEDIR)/scripts/run-singularity

run_singularity_INSTALL := $(DESTDIR)$(BINDIR)/run-singularity
$(run_singularity_INSTALL): $(run_singularity)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0755 $< $@

INSTALLFILES += $(run_singularity_INSTALL)


# capability config file
capability_config_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/capability.json
$(capability_config_INSTALL):
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)touch $@

INSTALLFILES += $(capability_config_INSTALL)


# syecl config file
syecl_config := $(SOURCEDIR)/internal/pkg/syecl/syecl.toml.example

syecl_config_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/ecl.toml
$(syecl_config_INSTALL): $(syecl_config)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(syecl_config_INSTALL)


# seccomp profile
seccomp_profile := $(SOURCEDIR)/etc/seccomp-profiles/default.json

seccomp_profile_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/seccomp-profiles/default.json
$(seccomp_profile_INSTALL): $(seccomp_profile)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(seccomp_profile_INSTALL)


# nvidia liblist config file
nvidia_liblist := $(SOURCEDIR)/etc/nvliblist.conf

nvidia_liblist_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/nvliblist.conf
$(nvidia_liblist_INSTALL): $(nvidia_liblist)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(nvidia_liblist_INSTALL)


# rocm liblist config file
rocm_liblist := $(SOURCEDIR)/etc/rocmliblist.conf

 rocm_liblist_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/rocmliblist.conf
$(rocm_liblist_INSTALL): $(rocm_liblist)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(rocm_liblist_INSTALL)


# cgroups config file
cgroups_config := $(SOURCEDIR)/internal/pkg/cgroups/example/cgroups.toml

cgroups_config_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/cgroups/cgroups.toml
$(cgroups_config_INSTALL): $(cgroups_config)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(cgroups_config_INSTALL)

# global keyring
global_keyring := $(SOURCEDIR)/etc/global-pgp-public

global_keyring_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/global-pgp-public
$(global_keyring_INSTALL): $(global_keyring)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(global_keyring_INSTALL)

# dmtcp  config file
dmtcp_conf := $(SOURCEDIR)/etc/dmtcp-conf.yaml

dmtcp_conf_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/dmtcp-conf.yaml
$(dmtcp_conf_INSTALL): $(dmtcp_conf)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(dmtcp_conf_INSTALL)
