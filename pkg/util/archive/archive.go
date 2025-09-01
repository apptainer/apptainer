// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
/*
Contains code adapted from:

   https://github.com/moby/moby/tree/master/pkg/archive

Copyright 2013-2018 Docker, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       https://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/system"
	"github.com/moby/go-archive"
	"github.com/moby/sys/sequential"
	"github.com/moby/sys/user"
	"github.com/moby/sys/userns"
	"github.com/pkg/errors"
)

// Unpack unpacks the decompressedArchive to dest with options. The target of
// any symlinks and hard links must be under destRoot. This differs from the
// unmodified upstream code, which requires that they are under dest.
func UnpackWithRoot(decompressedArchive io.Reader, dest, destRoot string, options *archive.TarOptions) error {
	tr := tar.NewReader(decompressedArchive)
	trBuf := pools.BufioReader32KPool.Get(nil)
	defer pools.BufioReader32KPool.Put(trBuf)

	var dirs []*tar.Header

	if options.WhiteoutFormat != 0 {
		return fmt.Errorf("options.WhiteoutFormat is not supported by UnpackWithRoot")
	}

	// Iterate through the files in the archive.
loop:
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}

		// ignore XGlobalHeader early to avoid creating parent directories for them
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			sylog.Debugf("PAX Global Extended Headers found for %s and ignored", hdr.Name)
			continue
		}

		// Normalize name, for safety and for a simple is-root check
		// This keeps "../" as-is, but normalizes "/../" to "/". Or Windows:
		// This keeps "..\" as-is, but normalizes "\..\" to "\".
		hdr.Name = filepath.Clean(hdr.Name)

		for _, exclude := range options.ExcludePatterns {
			if strings.HasPrefix(hdr.Name, exclude) {
				continue loop
			}
		}

		// Ensure that the parent directory exists.
		err = createImpliedDirectories(dest, hdr, options)
		if err != nil {
			return err
		}

		// #nosec G305 -- The joined path is checked for path traversal.
		path := filepath.Join(dest, hdr.Name)
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("%q is outside of %q", hdr.Name, dest)
		}

		// If path exits we almost always just want to remove and replace it
		// The only exception is when it is a directory *and* the file from
		// the layer is also a directory. Then we want to merge them (i.e.
		// just apply the metadata from the layer).
		if fi, err := os.Lstat(path); err == nil {
			if options.NoOverwriteDirNonDir && fi.IsDir() && hdr.Typeflag != tar.TypeDir {
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing directory with a non-directory from the archive.
				return fmt.Errorf("cannot overwrite directory %q with non-directory %q", path, dest)
			}

			if options.NoOverwriteDirNonDir && !fi.IsDir() && hdr.Typeflag == tar.TypeDir {
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing non-directory with a directory from the archive.
				return fmt.Errorf("cannot overwrite non-directory %q with directory %q", path, dest)
			}

			if fi.IsDir() && hdr.Name == "." {
				continue
			}

			if !fi.IsDir() || hdr.Typeflag != tar.TypeDir {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}
		trBuf.Reset(tr)

		if err := remapIDs(options.IDMap, hdr); err != nil {
			return err
		}

		if err := createTarFile(path, dest, destRoot, hdr, trBuf, options); err != nil {
			return err
		}

		// Directory mtimes must be handled at the end to avoid further
		// file creation in them to modify the directory mtime
		if hdr.Typeflag == tar.TypeDir {
			dirs = append(dirs, hdr)
		}
	}

	for _, hdr := range dirs {
		// #nosec G305 -- The header was checked for path traversal before it was appended to the dirs slice.
		path := filepath.Join(dest, hdr.Name)

		if err := system.Chtimes(path, hdr.AccessTime, hdr.ModTime); err != nil {
			return err
		}
	}
	return nil
}

// createImpliedDirectories will create all parent directories of the current path with default permissions, if they do
// not already exist. This is possible as the tar format supports 'implicit' directories, where their existence is
// defined by the paths of files in the tar, but there are no header entries for the directories themselves, and thus
// we most both create them and choose metadata like permissions.
//
// The caller should have performed filepath.Clean(hdr.Name), so hdr.Name will now be in the filepath format for the OS
// on which the daemon is running. This precondition is required because this function assumes a OS-specific path
// separator when checking that a path is not the root.
func createImpliedDirectories(dest string, hdr *tar.Header, options *archive.TarOptions) error {
	// Not the root directory, ensure that the parent directory exists
	if !strings.HasSuffix(hdr.Name, string(os.PathSeparator)) {
		parent := filepath.Dir(hdr.Name)
		parentPath := filepath.Join(dest, parent)
		if _, err := os.Lstat(parentPath); err != nil && os.IsNotExist(err) {
			// RootPair() is confined inside this loop as most cases will not require a call, so we can spend some
			// unneeded function calls in the uncommon case to encapsulate logic -- implied directories are a niche
			// usage that reduces the portability of an image.
			uid, gid := options.IDMap.RootPair()

			err = user.MkdirAllAndChown(parentPath, archive.ImpliedDirectoryMode, uid, gid, user.WithOnlyNew)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func remapIDs(idMapping user.IdentityMapping, hdr *tar.Header) error {
	uid, gid, err := idMapping.ToHost(hdr.Uid, hdr.Gid)
	hdr.Uid, hdr.Gid = uid, gid
	return err
}

const paxSchilyXattr = "SCHILY.xattr."

// createTarFile creates a file from a tar record. The target of any symlinks
// and hard links must be under extractRoot. This differs from the unmodified upstream
// code, which requires that they are under extractDir.
func createTarFile(path, extractDir, extractRoot string, hdr *tar.Header, reader io.Reader, opts *archive.TarOptions) error {
	var (
		Lchown           = true
		bestEffortXattrs bool
		chownOpts        *archive.ChownOpts
	)
	if opts != nil {
		Lchown = !opts.NoLchown
		chownOpts = opts.ChownOpts
		bestEffortXattrs = opts.BestEffortXattrs
	}

	// hdr.Mode is in linux format, which we can use for sycalls,
	// but for os.Foo() calls we need the mode converted to os.FileMode,
	// so use hdrInfo.Mode() (they differ for e.g. setuid bits)
	hdrInfo := hdr.FileInfo()

	switch hdr.Typeflag {
	case tar.TypeDir:
		// Create directory unless it exists as a directory already.
		// In that case we just want to merge the two
		if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
			if err := os.Mkdir(path, hdrInfo.Mode()); err != nil {
				return err
			}
		}

	case tar.TypeReg:
		// Source is regular file. We use sequential file access to avoid depleting
		// the standby list on Windows. On Linux, this equates to a regular os.OpenFile.
		file, err := sequential.OpenFile(path, os.O_CREATE|os.O_WRONLY, hdrInfo.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(file, reader); err != nil {
			file.Close()
			return err
		}
		file.Close()

	case tar.TypeBlock, tar.TypeChar:
		sylog.Warningf("Skipping %s - block / char devices are not copied", path)
		return nil

	case tar.TypeFifo:
		// Handle this is an OS-specific way
		sylog.Warningf("Skipping %s - fifos are not copied", path)

	case tar.TypeLink:
		// #nosec G305 -- The target path is checked for path traversal.
		targetPath := filepath.Join(extractDir, hdr.Linkname)
		// check for hardlink breakout
		if !strings.HasPrefix(targetPath, extractRoot) {
			return fmt.Errorf("invalid symlink target: %s, resolves to %s, not in root: %s", hdr.Linkname, targetPath, extractRoot)
		}

		if err := os.Link(targetPath, path); err != nil {
			return err
		}

	case tar.TypeSymlink:
		// 	path 				-> hdr.Linkname = targetPath
		// e.g. /extractDir/path/to/symlink 	-> ../2/file	= /extractDir/path/2/file
		targetPath := filepath.Join(filepath.Dir(path), hdr.Linkname) // #nosec G305 -- The target path is checked for path traversal.

		// the reason we don't need to check symlinks in the path (with FollowSymlinkInScope) is because
		// that symlink would first have to be created, which would be caught earlier, at this very check:
		if !strings.HasPrefix(targetPath, extractRoot) {
			return fmt.Errorf("invalid symlink target: %s, resolves to %s, not in root: %s", hdr.Linkname, targetPath, extractRoot)
		}
		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return err
		}

	case tar.TypeXGlobalHeader:
		sylog.Debugf("PAX Global Extended Headers found and ignored")
		return nil

	default:
		return fmt.Errorf("unhandled tar header type %d", hdr.Typeflag)
	}

	// Lchown is not supported on Windows.
	if Lchown && runtime.GOOS != "windows" {
		if chownOpts == nil {
			chownOpts = &archive.ChownOpts{UID: hdr.Uid, GID: hdr.Gid}
		}
		if err := os.Lchown(path, chownOpts.UID, chownOpts.GID); err != nil {
			msg := "failed to Lchown %q for UID %d, GID %d"
			if errors.Is(err, syscall.EINVAL) && userns.RunningInUserNS() {
				msg += " (try increasing the number of subordinate IDs in /etc/subuid and /etc/subgid)"
			}
			return errors.Wrapf(err, msg, path, hdr.Uid, hdr.Gid)
		}
	}

	var xattrErrs []string
	for key, value := range hdr.PAXRecords {
		xattr, ok := strings.CutPrefix(key, paxSchilyXattr)
		if !ok {
			continue
		}
		if err := system.Lsetxattr(path, xattr, []byte(value), 0); err != nil {
			if bestEffortXattrs && errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EPERM) {
				// EPERM occurs if modifying xattrs is not allowed. This can
				// happen when running in userns with restrictions (ChromeOS).
				xattrErrs = append(xattrErrs, err.Error())
				continue
			}
			return err
		}
	}

	if len(xattrErrs) > 0 {
		sylog.Warningf("Ignored xattrs in archive: underlying filesystem doesn't support them: %v", xattrErrs)
	}

	// There is no LChmod, so ignore mode for symlink. Also, this
	// must happen after chown, as that can modify the file mode
	if err := handleLChmod(hdr, path, hdrInfo); err != nil {
		return err
	}

	aTime := hdr.AccessTime
	if aTime.Before(hdr.ModTime) {
		// Last access time should never be before last modified time.
		aTime = hdr.ModTime
	}

	// system.Chtimes doesn't support a NOFOLLOW flag atm
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := system.Chtimes(path, aTime, hdr.ModTime); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := system.Chtimes(path, aTime, hdr.ModTime); err != nil {
			return err
		}
	} else {
		ts := []syscall.Timespec{timeToTimespec(aTime), timeToTimespec(hdr.ModTime)}
		if err := system.LUtimesNano(path, ts); err != nil && err != system.ErrNotSupportedPlatform {
			return err
		}
	}
	return nil
}

func handleLChmod(hdr *tar.Header, path string, hdrInfo os.FileInfo) error {
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := os.Chmod(path, hdrInfo.Mode()); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := os.Chmod(path, hdrInfo.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func timeToTimespec(time time.Time) (ts syscall.Timespec) {
	if time.IsZero() {
		// Return UTIME_OMIT special value
		ts.Sec = 0
		ts.Nsec = (1 << 30) - 2
		return
	}
	return syscall.NsecToTimespec(time.UnixNano())
}
