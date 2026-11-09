# A statically linked glibc crashes when it loads the host's NSS modules,
# so read /etc/passwd and /etc/group directly instead
GO_TAGS += osusergo
GO_TAGS_SUID += osusergo
