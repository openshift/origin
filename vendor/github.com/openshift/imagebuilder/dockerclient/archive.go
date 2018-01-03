package dockerclient

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/golang/glog"
)

// TransformFileFunc is given a chance to transform an arbitrary input file.
type TransformFileFunc func(h *tar.Header, r io.Reader) (data []byte, update bool, skip bool, err error)

// FilterArchive transforms the provided input archive to a new archive,
// giving the fn a chance to transform arbitrary files.
func FilterArchive(r io.Reader, w io.Writer, fn TransformFileFunc) error {
	tr := tar.NewReader(r)
	tw := tar.NewWriter(w)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			return tw.Close()
		}
		if err != nil {
			return err
		}

		var body io.Reader = tr
		data, ok, skip, err := fn(h, tr)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if ok {
			h.Size = int64(len(data))
			body = bytes.NewBuffer(data)
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := io.Copy(tw, body); err != nil {
			return err
		}
	}
}

type CreateFileFunc func() (*tar.Header, io.ReadCloser, bool, error)

func NewLazyArchive(fn CreateFileFunc) io.ReadCloser {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	go func() {
		for {
			h, r, more, err := fn()
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			if h == nil {
				tw.Flush()
				pw.Close()
				return
			}
			if err := tw.WriteHeader(h); err != nil {
				r.Close()
				pw.CloseWithError(err)
				return
			}
			n, err := io.Copy(tw, &io.LimitedReader{R: r, N: h.Size})
			r.Close()
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			if n != h.Size {
				pw.CloseWithError(fmt.Errorf("short read for %s", h.Name))
				return
			}
			if !more {
				tw.Flush()
				pw.Close()
				return
			}
		}
	}()
	return pr
}

func archiveFromURL(src, dst, tempDir string) (io.Reader, io.Closer, error) {
	// get filename from URL
	u, err := url.Parse(src)
	if err != nil {
		return nil, nil, err
	}
	base := path.Base(u.Path)
	if base == "." {
		return nil, nil, fmt.Errorf("cannot determine filename from url: %s", u)
	}
	resp, err := http.Get(src)
	if err != nil {
		return nil, nil, err
	}
	archive := NewLazyArchive(func() (*tar.Header, io.ReadCloser, bool, error) {
		if resp.StatusCode >= 400 {
			return nil, nil, false, fmt.Errorf("server returned a status code >= 400: %s", resp.Status)
		}

		header := &tar.Header{
			Name: sourceToDestinationName(path.Base(u.Path), dst, false),
			Mode: 0600,
		}
		r := resp.Body
		if resp.ContentLength == -1 {
			f, err := ioutil.TempFile(tempDir, "url")
			if err != nil {
				return nil, nil, false, fmt.Errorf("unable to create temporary file for source URL: %v", err)
			}
			n, err := io.Copy(f, resp.Body)
			if err != nil {
				f.Close()
				return nil, nil, false, fmt.Errorf("unable to download source URL: %v", err)
			}
			if err := f.Close(); err != nil {
				return nil, nil, false, fmt.Errorf("unable to write source URL: %v", err)
			}
			f, err = os.Open(f.Name())
			if err != nil {
				return nil, nil, false, fmt.Errorf("unable to open downloaded source URL: %v", err)
			}
			r = f
			header.Size = n
		} else {
			header.Size = resp.ContentLength
		}
		return header, r, false, nil
	})
	return archive, closers{resp.Body.Close, archive.Close}, nil
}

func archiveFromDisk(directory string, src, dst string, allowDownload bool, excludes []string) (io.Reader, io.Closer, error) {
	var err error
	if filepath.IsAbs(src) {
		src, err = filepath.Rel(filepath.Dir(src), src)
		if err != nil {
			return nil, nil, err
		}
	}

	infos, err := CalcCopyInfo(src, directory, true)
	if err != nil {
		return nil, nil, err
	}

	options := archiveOptionsFor(infos, dst, excludes)

	glog.V(4).Infof("Tar of directory %s %#v", directory, options)
	rc, err := archive.TarWithOptions(directory, options)
	return rc, rc, err
}

// * -> test
// a (dir)  -> test
// a (file) -> test
// a (dir)  -> test/
// a (file) -> test/
//
func archivePathMapper(src, dst string, isDestDir bool) (fn func(name string, isDir bool) (string, bool)) {
	srcPattern := filepath.Clean(src)
	if srcPattern == "." {
		srcPattern = "*"
	}
	pattern := filepath.Base(srcPattern)

	// no wildcards
	if !containsWildcards(pattern) {
		return func(name string, isDir bool) (string, bool) {
			if name == srcPattern {
				if isDir {
					return "", false
				}
				if isDestDir {
					return filepath.Join(dst, filepath.Base(name)), true
				}
				return dst, true
			}

			remainder := strings.TrimPrefix(name, srcPattern+string(filepath.Separator))
			if remainder == name {
				return "", false
			}
			return filepath.Join(dst, remainder), true
		}
	}

	// root with pattern
	prefix := filepath.Dir(srcPattern)
	if prefix == "." {
		return func(name string, isDir bool) (string, bool) {
			// match only on the first segment under the prefix
			var firstSegment = name
			if i := strings.Index(name, string(filepath.Separator)); i != -1 {
				firstSegment = name[:i]
			}
			ok, _ := filepath.Match(pattern, firstSegment)
			if !ok {
				return "", false
			}
			return filepath.Join(dst, name), true
		}
	}
	prefix += string(filepath.Separator)

	// nested with pattern pattern
	return func(name string, isDir bool) (string, bool) {
		remainder := strings.TrimPrefix(name, prefix)
		if remainder == name {
			return "", false
		}
		// match only on the first segment under the prefix
		var firstSegment = remainder
		if i := strings.Index(remainder, string(filepath.Separator)); i != -1 {
			firstSegment = remainder[:i]
		}
		ok, _ := filepath.Match(pattern, firstSegment)
		if !ok {
			return "", false
		}
		return filepath.Join(dst, remainder), true
	}
}

func archiveFromFile(file string, src, dst string, excludes []string) (io.Reader, io.Closer, error) {
	var err error
	if filepath.IsAbs(src) {
		src, err = filepath.Rel(filepath.Dir(src), src)
		if err != nil {
			return nil, nil, err
		}
	}

	// TODO: multiple sources also require treating dst as a directory
	isDestDir := strings.HasSuffix(dst, "/") || path.Base(dst) == "."
	dst = path.Clean(dst)
	mapperFn := archivePathMapper(src, dst, isDestDir)

	pm, err := fileutils.NewPatternMatcher(excludes)
	if err != nil {
		return nil, nil, err
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		in, err := archive.DecompressStream(f)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		err = FilterArchive(in, pw, func(h *tar.Header, r io.Reader) ([]byte, bool, bool, error) {
			// skip a file if it doesn't match the src
			isDir := h.Typeflag == tar.TypeDir
			newName, ok := mapperFn(h.Name, isDir)
			if !ok {
				return nil, false, true, err
			}
			if newName == "." {
				return nil, false, true, nil
			}

			// skip based on excludes
			if ok, _ := pm.Matches(h.Name); ok {
				return nil, false, true, nil
			}

			h.Name = newName
			// include all files
			return nil, false, false, nil
		})
		pw.CloseWithError(err)
	}()
	return pr, closers{f.Close, pr.Close}, nil
}

func archiveOptionsFor(infos []CopyInfo, dst string, excludes []string) *archive.TarOptions {
	dst = trimLeadingPath(dst)
	options := &archive.TarOptions{}
	pm, err := fileutils.NewPatternMatcher(excludes)
	if err != nil {
		return options
	}
	for _, info := range infos {
		if ok, _ := pm.Matches(info.Path); ok {
			continue
		}
		options.IncludeFiles = append(options.IncludeFiles, info.Path)
		if len(dst) == 0 {
			continue
		}
		if options.RebaseNames == nil {
			options.RebaseNames = make(map[string]string)
		}
		if info.FromDir || strings.HasSuffix(dst, "/") || strings.HasSuffix(dst, "/.") || dst == "." {
			if strings.HasSuffix(info.Path, "/") {
				options.RebaseNames[info.Path] = dst
			} else {
				options.RebaseNames[info.Path] = path.Join(dst, path.Base(info.Path))
			}
		} else {
			options.RebaseNames[info.Path] = dst
		}
	}
	options.ExcludePatterns = excludes
	return options
}

func sourceToDestinationName(src, dst string, forceDir bool) string {
	switch {
	case forceDir, strings.HasSuffix(dst, "/"), path.Base(dst) == ".":
		return path.Join(dst, src)
	default:
		return dst
	}
}
