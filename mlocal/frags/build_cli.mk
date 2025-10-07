# This file contains all of the rules for building the apptainer CLI binary

# apptainer build config
apptainer_build_config := $(SOURCEDIR)/internal/pkg/buildcfg/config.go
$(apptainer_build_config): $(BUILDDIR_ABSPATH)/config.h $(SOURCEDIR)/scripts/go-generate
	$(V)rm -f $(apptainer_build_config)
	$(V) cd $(SOURCEDIR)/internal/pkg/buildcfg && $(SOURCEDIR)/scripts/go-generate

CLEANFILES += $(apptainer_build_config)

# contain apptainer_SOURCE variable list
apptainer_deps := $(BUILDDIR_ABSPATH)/apptainer.deps

-include $(apptainer_deps)

$(apptainer_deps): $(GO_MODFILES)
	@echo " GEN GO DEP" $@
	$(V)cd $(SOURCEDIR) && ./makeit/gengodep -v3 "$(GO)" "apptainer_SOURCE" "$(GO_TAGS)" "$@" "$(SOURCEDIR)/cmd/apptainer"

# Look at dependencies file changes via apptainer_deps
# because it means that a module was updated.
apptainer := $(BUILDDIR_ABSPATH)/apptainer
$(apptainer): $(apptainer_build_config) $(apptainer_deps) $(apptainer_SOURCE)
	@echo " GO" $@; echo "    [+] GO_TAGS" \"$(GO_TAGS)\"
	$(V)cd $(SOURCEDIR) && $(GO) build $(GO_MODFLAGS) $(GO_BUILDMODE) \
		$(GO_GCFLAGS) $(GO_LDFLAGS) -tags "$(GO_TAGS)" \
		-o $@ $(SOURCEDIR)/cmd/apptainer

apptainer_INSTALL := $(DESTDIR)$(BINDIR)/apptainer
$(apptainer_INSTALL): $(apptainer)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0755 $(apptainer) $(apptainer_INSTALL) # set cp to install

singularity_INSTALL := $(DESTDIR)$(BINDIR)/singularity
$(singularity_INSTALL):
	@echo " INSTALL" $@
	$(V)ln -sf apptainer $(singularity_INSTALL)

CLEANFILES += $(apptainer)
INSTALLFILES += $(apptainer_INSTALL) $(singularity_INSTALL)
ALL += $(apptainer)


# bash_completion files
bash_completion1 :=  $(BUILDDIR_ABSPATH)/bash-completion/completions/apptainer
bash_completion2 :=  $(BUILDDIR_ABSPATH)/bash-completion/completions/singularity
bash_completions := $(bash_completion1) $(bash_completion2)
$(bash_completions): $(apptainer_build_config)
	@echo " GEN" $@
	$(V)rm -f $@
	$(V)mkdir -p $(@D)
	$(V)cd $(SOURCEDIR) && $(GO) run $(GO_MODFLAGS) -tags "$(GO_TAGS)" \
		cmd/bash_completion/bash_completion.go $@

bash_completion1_INSTALL := $(DESTDIR)$(DATADIR)/bash-completion/completions/apptainer
bash_completion2_INSTALL := $(DESTDIR)$(DATADIR)/bash-completion/completions/singularity
bash_completions_INSTALL := $(bash_completion1_INSTALL) $(bash_completion2_INSTALL)
$(bash_completion1_INSTALL): $(bash_completion1)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

$(bash_completion2_INSTALL): $(bash_completion2)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

CLEANFILES += $(bash_completions)
INSTALLFILES += $(bash_completions_INSTALL)
ALL += $(bash_completions)


# apptainer.conf file
config := $(BUILDDIR_ABSPATH)/apptainer.conf
config_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/apptainer.conf
# override this to empty to avoid merging old configuration settings
old_config := $(config_INSTALL)

$(config): $(apptainer)
	@echo " GEN $@`if [ -n "$(old_config)" ]; then echo " from $(old_config)"; fi`"
	$(V)$(apptainer) confgen $(old_config) $(config)

$(config_INSTALL): $(config)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(config_INSTALL)
ALL += $(config)

# remote config file
remote_config := $(SOURCEDIR)/etc/remote.yaml

remote_config_INSTALL := $(DESTDIR)$(SYSCONFDIR)/apptainer/remote.yaml
$(remote_config_INSTALL): $(remote_config)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $(@D)
	$(V)install -m 0644 $< $@

INSTALLFILES += $(remote_config_INSTALL)

man_pages := $(BUILDDIR_ABSPATH)$(MANDIR)/man1
$(man_pages): apptainer
	@echo " MAN" $@
	mkdir -p $@
	$(V)cd $(SOURCEDIR) && $(GO) run $(GO_MODFLAGS) -tags "$(GO_TAGS)" \
		cmd/docs/docs.go man --dir $@
	$(V)cd $@;							\
		for M in apptainer*; do					\
			S="`echo $$M|sed 's/apptainer/singularity/'`";	\
			ln -fs $$M $$S;					\
		done

man_pages_INSTALL := $(DESTDIR)$(MANDIR)/man1
$(man_pages_INSTALL): $(man_pages)
	@echo " INSTALL" $@
	$(V)umask 0022 && mkdir -p $@
	$(V)install -m 0644 -t $@ $(man_pages)/*

INSTALLFILES += $(man_pages_INSTALL)
ALL += $(man_pages)
