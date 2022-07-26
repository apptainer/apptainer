CFLAGS := $(filter-out -D_FORTIFY_SOURCE=2 -O2,$(CFLAGS)) -O0 -ggdb
CPPFLAGS += -DDBG
