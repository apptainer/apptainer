# This tells go's link command to add a GNU Build Id, needed for later
#   symbol stripping for example as is done by rpmbuild.
GO_LDFLAGS += -ldflags="-B 0x12345678"
