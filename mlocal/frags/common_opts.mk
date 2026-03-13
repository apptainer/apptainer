# a list of extra files to clean augmented by each module (*.mconf) file
CLEANFILES :=

# general build-wide compile options
AFLAGS := -g

GO_EXTLDFLAGS := -Wl,-z,relro,-z,now
# The default LDFLAGS interfere on EL10 through Fedora on ppc64le arch
uname_m := $(shell uname -m)
ifneq ($(uname_m),ppc64le)
GO_EXTLDFLAGS += $(LDFLAGS)
endif
