# These are from the Red Hat rpm %gobuild macro.  Among other things, they
# enable generation of debugsource packages in addition to debuginfo.
# The libtrust_openssl tag is left out because it only works on Red Hat.
# "-B gobuildid" is a simplification of the macro, to tell go's link
# command to add a GNU Build Id itself.
GO_TAGS += rpm_crashtraceback
GO_LDFLAGS += -ldflags="-linkmode=external -compressdwarf=false -B gobuildid -extldflags '$(LDFLAGS)'"
