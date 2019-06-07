package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/system"
)

type (
	// Compression is the state represents if compressed or not.
	Compression int
	// WhiteoutFormat is the format of whiteouts unpacked
	WhiteoutFormat int

	// TarOptions wraps the tar options.
	TarOptions struct {
		IncludeFiles    []string
		ExcludePatterns []string
		Compression     Compression
		NoLchown        bool
		// REMOVED: use remap instead
		//UIDMaps          []idtools.IDMap
		//GIDMaps          []idtools.IDMap
		ChownOpts        *idtools.IDPair
		IncludeSourceDir bool
		// WhiteoutFormat is the expected on disk format for whiteout files.
		// This format will be converted to the standard format on pack
		// and from the standard format on unpack.
		WhiteoutFormat WhiteoutFormat
		// When unpacking, specifies whether overwriting a directory with a
		// non-directory is allowed and vice versa.
		NoOverwriteDirNonDir bool
		// For each include when creating an archive, the included name will be
		// replaced with the matching name from this map.
		RebaseNames map[string]string
		InUserNS    bool

		// ADDED: allow bypassing chown
		// If false, no chown will be performed
		Chown bool

		AlterHeaders AlterHeader
	}
)

// breakoutError is used to differentiate errors related to breaking out
// When testing archive breakout in the unit tests, this error is expected
// in order for the test to pass.
type breakoutError error

type tarWhiteoutConverter interface {
	ConvertWrite(*tar.Header, string, os.FileInfo) (*tar.Header, error)
	ConvertRead(*tar.Header, string) (bool, error)
}

type AlterHeader interface {
	Alter(*tar.Header) (bool, error)
}

type RemapIDs struct {
	mappings *idtools.IDMappings
}

func (r RemapIDs) Alter(hdr *tar.Header) (bool, error) {
	ids, err := r.mappings.ToHost(idtools.IDPair{UID: hdr.Uid, GID: hdr.Gid})
	hdr.Uid, hdr.Gid = ids.UID, ids.GID
	return true, err
}

// ApplyLayer is copied from github.com/docker/docker/pkg/archive
func ApplyLayer(dest string, layer io.Reader, options *TarOptions) (int64, error) {
	dest = filepath.Clean(dest)
	var err error
	layer, err = archive.DecompressStream(layer)
	if err != nil {
		return 0, err
	}
	return unpackLayer(dest, layer, options)
}

// unpackLayer is copied from github.com/docker/docker/pkg/archive
// unpackLayer unpack `layer` to a `dest`. The stream `layer` can be
// compressed or uncompressed.
// Returns the size in bytes of the contents of the layer.
func unpackLayer(dest string, layer io.Reader, options *TarOptions) (size int64, err error) {
	tr := tar.NewReader(layer)
	trBuf := pools.BufioReader32KPool.Get(tr)
	defer pools.BufioReader32KPool.Put(trBuf)

	var dirs []*tar.Header
	unpackedPaths := make(map[string]struct{})

	if options == nil {
		options = &TarOptions{Chown: true}
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}
	// idMappings := idtools.NewIDMappingsFromMaps(options.UIDMaps, options.GIDMaps)

	aufsTempdir := ""
	aufsHardlinks := make(map[string]*tar.Header)

	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return 0, err
		}

		size += hdr.Size

		// Normalize name, for safety and for a simple is-root check
		hdr.Name = filepath.Clean(hdr.Name)

		if options.AlterHeaders != nil {
			ok, err := options.AlterHeaders.Alter(hdr)
			if err != nil {
				return 0, err
			}
			if !ok {
				continue
			}
		}

		// Windows does not support filenames with colons in them. Ignore
		// these files. This is not a problem though (although it might
		// appear that it is). Let's suppose a client is running docker pull.
		// The daemon it points to is Windows. Would it make sense for the
		// client to be doing a docker pull Ubuntu for example (which has files
		// with colons in the name under /usr/share/man/man3)? No, absolutely
		// not as it would really only make sense that they were pulling a
		// Windows image. However, for development, it is necessary to be able
		// to pull Linux images which are in the repository.
		//
		// TODO Windows. Once the registry is aware of what images are Windows-
		// specific or Linux-specific, this warning should be changed to an error
		// to cater for the situation where someone does manage to upload a Linux
		// image but have it tagged as Windows inadvertently.
		if runtime.GOOS == "windows" {
			if strings.Contains(hdr.Name, ":") {
				continue
			}
		}

		// Note as these operations are platform specific, so must the slash be.
		if !strings.HasSuffix(hdr.Name, string(os.PathSeparator)) {
			// Not the root directory, ensure that the parent directory exists.
			// This happened in some tests where an image had a tarfile without any
			// parent directories.
			parent := filepath.Dir(hdr.Name)
			parentPath := filepath.Join(dest, parent)

			if _, err := os.Lstat(parentPath); err != nil && os.IsNotExist(err) {
				err = system.MkdirAll(parentPath, 0600, "")
				if err != nil {
					return 0, err
				}
			}
		}

		// Skip AUFS metadata dirs
		if strings.HasPrefix(hdr.Name, archive.WhiteoutMetaPrefix) {
			// Regular files inside /.wh..wh.plnk can be used as hardlink targets
			// We don't want this directory, but we need the files in them so that
			// such hardlinks can be resolved.
			if strings.HasPrefix(hdr.Name, archive.WhiteoutLinkDir) && hdr.Typeflag == tar.TypeReg {
				basename := filepath.Base(hdr.Name)
				aufsHardlinks[basename] = hdr
				if aufsTempdir == "" {
					if aufsTempdir, err = ioutil.TempDir("", "dockerplnk"); err != nil {
						return 0, err
					}
					defer os.RemoveAll(aufsTempdir)
				}
				if err := createTarFile(filepath.Join(aufsTempdir, basename), dest, hdr, tr, options.Chown, options.ChownOpts, options.InUserNS); err != nil {
					return 0, err
				}
			}

			if hdr.Name != archive.WhiteoutOpaqueDir {
				continue
			}
		}

		path := filepath.Join(dest, hdr.Name)
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return 0, err
		}

		// Note as these operations are platform specific, so must the slash be.
		if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return 0, breakoutError(fmt.Errorf("%q is outside of %q", hdr.Name, dest))
		}
		base := filepath.Base(path)

		if strings.HasPrefix(base, archive.WhiteoutPrefix) {
			dir := filepath.Dir(path)
			if base == archive.WhiteoutOpaqueDir {
				_, err := os.Lstat(dir)
				if err != nil {
					return 0, err
				}
				err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						if os.IsNotExist(err) {
							err = nil // parent was deleted
						}
						return err
					}
					if path == dir {
						return nil
					}
					if _, exists := unpackedPaths[path]; !exists {
						err := os.RemoveAll(path)
						return err
					}
					return nil
				})
				if err != nil {
					return 0, err
				}
			} else {
				originalBase := base[len(archive.WhiteoutPrefix):]
				originalPath := filepath.Join(dir, originalBase)
				if err := os.RemoveAll(originalPath); err != nil {
					return 0, err
				}
			}
		} else {
			// If path exits we almost always just want to remove and replace it.
			// The only exception is when it is a directory *and* the file from
			// the layer is also a directory. Then we want to merge them (i.e.
			// just apply the metadata from the layer).
			if fi, err := os.Lstat(path); err == nil {
				if !(fi.IsDir() && hdr.Typeflag == tar.TypeDir) {
					if err := os.RemoveAll(path); err != nil {
						return 0, err
					}
				}
			}

			trBuf.Reset(tr)
			srcData := io.Reader(trBuf)
			srcHdr := hdr

			// Hard links into /.wh..wh.plnk don't work, as we don't extract that directory, so
			// we manually retarget these into the temporary files we extracted them into
			if hdr.Typeflag == tar.TypeLink && strings.HasPrefix(filepath.Clean(hdr.Linkname), archive.WhiteoutLinkDir) {
				linkBasename := filepath.Base(hdr.Linkname)
				srcHdr = aufsHardlinks[linkBasename]
				if srcHdr == nil {
					return 0, fmt.Errorf("Invalid aufs hardlink")
				}
				tmpFile, err := os.Open(filepath.Join(aufsTempdir, linkBasename))
				if err != nil {
					return 0, err
				}
				defer tmpFile.Close()
				srcData = tmpFile
			}

			// if err := remapIDs(idMappings, srcHdr); err != nil {
			// 	return 0, err
			// }

			if err := createTarFile(path, dest, srcHdr, srcData, options.Chown, options.ChownOpts, options.InUserNS); err != nil {
				return 0, err
			}

			// Directory mtimes must be handled at the end to avoid further
			// file creation in them to modify the directory mtime
			if hdr.Typeflag == tar.TypeDir {
				dirs = append(dirs, hdr)
			}
			unpackedPaths[path] = struct{}{}
		}
	}

	for _, hdr := range dirs {
		path := filepath.Join(dest, hdr.Name)
		if err := system.Chtimes(path, hdr.AccessTime, hdr.ModTime); err != nil {
			return 0, err
		}
	}

	return size, nil
}

func createTarFile(path, extractDir string, hdr *tar.Header, reader io.Reader, Lchown bool, chownOpts *idtools.IDPair, inUserns bool) error {
	// hdr.Mode is in linux format, which we can use for sycalls,
	// but for os.Foo() calls we need the mode converted to os.FileMode,
	// so use hdrInfo.Mode() (they differ for e.g. setuid bits)
	hdrInfo := hdr.FileInfo()

	switch hdr.Typeflag {
	case tar.TypeDir:
		// Create directory unless it exists as a directory already.
		// In that case we just want to merge the two
		if fi, err := os.Lstat(path); !(err == nil && fi.IsDir()) {
			if err := os.Mkdir(path, hdrInfo.Mode()); err != nil {
				return err
			}
		}

	case tar.TypeReg, tar.TypeRegA:
		// Source is regular file. We use system.OpenFileSequential to use sequential
		// file access to avoid depleting the standby list on Windows.
		// On Linux, this equates to a regular os.OpenFile
		file, err := system.OpenFileSequential(path, os.O_CREATE|os.O_WRONLY, hdrInfo.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(file, reader); err != nil {
			file.Close()
			return err
		}
		file.Close()

	case tar.TypeBlock, tar.TypeChar:
		if inUserns { // cannot create devices in a userns
			return nil
		}
		// Handle this is an OS-specific way
		if err := handleTarTypeBlockCharFifo(hdr, path); err != nil {
			return err
		}

	case tar.TypeFifo:
		// Handle this is an OS-specific way
		if err := handleTarTypeBlockCharFifo(hdr, path); err != nil {
			return err
		}

	case tar.TypeLink:
		targetPath := filepath.Join(extractDir, hdr.Linkname)
		// check for hardlink breakout
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid hardlink %q -> %q", targetPath, hdr.Linkname))
		}
		if err := os.Link(targetPath, path); err != nil {
			return err
		}

	case tar.TypeSymlink:
		// 	path 				-> hdr.Linkname = targetPath
		// e.g. /extractDir/path/to/symlink 	-> ../2/file	= /extractDir/path/2/file
		targetPath := filepath.Join(filepath.Dir(path), hdr.Linkname)

		// the reason we don't need to check symlinks in the path (with FollowSymlinkInScope) is because
		// that symlink would first have to be created, which would be caught earlier, at this very check:
		if !strings.HasPrefix(targetPath, extractDir) {
			return breakoutError(fmt.Errorf("invalid symlink %q -> %q", path, hdr.Linkname))
		}
		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return err
		}

	case tar.TypeXGlobalHeader:
		return nil

	default:
		return fmt.Errorf("unhandled tar header type %d", hdr.Typeflag)
	}

	// Lchown is not supported on Windows.
	if Lchown && runtime.GOOS != "windows" {
		if chownOpts == nil {
			chownOpts = &idtools.IDPair{UID: hdr.Uid, GID: hdr.Gid}
		}
		if err := os.Lchown(path, chownOpts.UID, chownOpts.GID); err != nil {
			return err
		}
	}

	var errors []string
	for key, value := range hdr.Xattrs {
		if err := system.Lsetxattr(path, key, []byte(value), 0); err != nil {
			if err == syscall.ENOTSUP {
				// We ignore errors here because not all graphdrivers support
				// xattrs *cough* old versions of AUFS *cough*. However only
				// ENOTSUP should be emitted in that case, otherwise we still
				// bail.
				errors = append(errors, err.Error())
				continue
			}
			return err
		}

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
