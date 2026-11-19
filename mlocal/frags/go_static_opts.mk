GO_TAGS += no_openssl
GO_TAGS_SUID += no_openssl
# Static builds cannot load the host's NSS modules, so fall back to the
# host's getent for users and groups from sources such as LDAP or SSSD.
# Not for the setuid starter.
GO_TAGS += getent_fallback
