package git

import (
	"bufio"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// ParseRepository parses a string that may be in the Git format (git@) or URL format
// and extracts the appropriate value. Any fragment on the URL is preserved.
//
// Protocols returned:
// - http, https
// - file
// - git
func ParseRepository(s string) (*url.URL, error) {
	uri, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if uri.Scheme == "" && !strings.HasPrefix(uri.Path, "git@") {
		path := s
		ref := ""
		segments := strings.SplitN(path, "#", 2)
		if len(segments) == 2 {
			path, ref = segments[0], segments[1]
		}
		path, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		uri = &url.URL{
			Scheme:   "file",
			Path:     path,
			Fragment: ref,
		}
	}

	return uri, nil
}

// NameFromRepositoryURL suggests a name for a repository URL based on the last
// segment of the path, or returns false
func NameFromRepositoryURL(url *url.URL) (string, bool) {
	// from path
	if len(url.Path) > 0 {
		base := path.Base(url.Path)
		if len(base) > 0 && base != "/" {
			if ext := path.Ext(base); ext == ".git" {
				base = base[:len(base)-4]
			}
			return base, true
		}
	}
	return "", false
}

type ChangedRef struct {
	Ref string
	Old string
	New string
}

func ParsePostReceive(r io.Reader) ([]ChangedRef, error) {
	refs := []ChangedRef{}
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		segments := strings.Split(scan.Text(), " ")
		if len(segments) != 3 {
			continue
		}
		refs = append(refs, ChangedRef{
			Ref: segments[2],
			Old: segments[0],
			New: segments[1],
		})
	}
	if err := scan.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return refs, nil
}
