/*
  Copyright (c) Contributors to the Apptainer project, established as
    Apptainer a Series of LF Projects LLC.
    For website terms of use, trademark policy, privacy policy and other
    project policies see https://lfprojects.org/policies

  This software is licensed under a 3-clause BSD license.  Please
  consult LICENSE.md file distributed with the sources of this project
  regarding your rights to use or distribute this software.
*/

/*
   LD_PRELOAD wrapper to add an offset into a file read by fuse2fs.
   Set OFFSETPRELOAD_FILE to the path of the file and OFFSETPRELOAD_OFFSET
     to the value of the offset.
   This is not general purpose, it is specific to fuse2fs.
*/

#define _GNU_SOURCE

#include <sys/types.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>

static int offsetfd = -3;
static int offsetval;

ssize_t pread64(int fd, void *buf, size_t count, off_t offset) {
	static off_t (*original_pread64)(int, void *, size_t, off_t) = NULL;
	if (original_pread64 == NULL) {
		original_pread64 = dlsym(RTLD_NEXT, "pread64");
	}

	if (offsetfd == fd) {
		offset += offsetval;
	}

	return (*original_pread64)(fd, buf, count, offset);
}

ssize_t pwrite64(int fd, const void *buf, size_t count, off_t offset) {
	static off_t (*original_pwrite64)(int, const void *, size_t, off_t) = NULL;
	if (original_pwrite64 == NULL) {
		original_pwrite64 = dlsym(RTLD_NEXT, "pwrite64");
	}

	if (offsetfd == fd) {
		offset += offsetval;
	}

	return (*original_pwrite64)(fd, buf, count, offset);
}

static int ___open64(int (*original_open64)(const char *, int, int, int), const char *path, int flags1, int flags2, int flags3) {
	static char *offsetpath = NULL;
	if (offsetfd == -3) {
		offsetfd = -2;
		offsetpath = getenv("OFFSETPRELOAD_FILE");
		char *valenv = getenv("OFFSETPRELOAD_OFFSET");
		if (valenv != NULL) {
			offsetval = atoi(valenv);
		}
	}

	int fd = (*original_open64)(path, flags1, flags2, flags3);

	if (fd >= 0) {
		if ((offsetpath != NULL) && (strcmp(offsetpath, path) == 0)) {
			offsetfd = fd;
		}
	}

	return fd;
}

// This is the version used by some compilations of fuse2fs
int __open64_2(const char *path, int flags1, int flags2, int flags3) {
	static int (*original_open64_2)(const char*, int, int, int) = NULL;
	if (original_open64_2 == NULL) {
		original_open64_2 = dlsym(RTLD_NEXT, "__open64_2");
	}
        return ___open64(original_open64_2, path, flags1, flags2, flags3);
}

// This is more parameters than the real open64, but use that many because
// we want to use a common function with __open64_2
int open64(const char *path, int flags1, int flags2, int flags3) {
	static int (*original_open64)(const char*, int, int, int) = NULL;
	if (original_open64 == NULL) {
		original_open64 = dlsym(RTLD_NEXT, "open64");
	}
        return ___open64(original_open64, path, flags1, flags2, flags3);
}
