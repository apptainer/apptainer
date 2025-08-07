# This file contains generation rules for scripts

# go-test script
$(SOURCEDIR)/scripts/go-test: export GO := $(GO)
$(SOURCEDIR)/scripts/go-test: export GOFLAGS := $(GOFLAGS)
$(SOURCEDIR)/scripts/go-test: export GO_TAGS := $(GO_TAGS)
$(SOURCEDIR)/scripts/go-test: export SUDO_SCRIPT := $(SOURCEDIR)/scripts/test-sudo
$(SOURCEDIR)/scripts/go-test: $(SOURCEDIR)/scripts/go-test.in $(SOURCEDIR)/scripts/expand-env.go $(BUILDDIR_ABSPATH)/config.h
	@echo ' GEN $@'
	$(V) cd $(SOURCEDIR) && $(GO) run $(GO_MODFLAGS) scripts/expand-env.go < $< > $@
	$(V) chmod +x $@

ALL += $(SOURCEDIR)/scripts/go-test

# go-generate script
$(SOURCEDIR)/scripts/go-generate: export BUILDDIR := $(BUILDDIR_ABSPATH)
$(SOURCEDIR)/scripts/go-generate: export GO := $(GO)
$(SOURCEDIR)/scripts/go-generate: export GOFLAGS := $(GOFLAGS)
$(SOURCEDIR)/scripts/go-generate: export GO_TAGS := $(GO_TAGS)
$(SOURCEDIR)/scripts/go-generate: $(SOURCEDIR)/scripts/go-generate.in $(SOURCEDIR)/scripts/expand-env.go $(BUILDDIR_ABSPATH)/config.h
	@echo ' GEN $@'
	$(V) cd $(SOURCEDIR) && $(GO) run $(GO_MODFLAGS) scripts/expand-env.go < $< > $@
	$(V) chmod +x $@

.PHONY: codegen
codegen: $(SOURCEDIR)/scripts/go-generate
	cd $(SOURCEDIR) && ./scripts/go-generate -x ./...

ALL += $(SOURCEDIR)/scripts/go-generate

