package git

import (
	"bufio"
	"io"
	"net/url"
	"path"
	"strings"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
)

// ParseRepository parses a string that may be in the Git format (git@) or URL format
// and extracts the appropriate value. Any fragment on the URL is preserved.
//
// Protocols returned:
// - http, https
// - file
// - git
// - ssh
func ParseRepository(s string) (*url.URL, error) {
	uri, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	// There are some shortcomings with url.Parse when it comes to GIT, namely wrt
	// the GIT local/file and ssh protocols - it does not handle implied schema (i.e. no <proto>:// prefix)well;
	// We handle those caveats here
	err = s2igit.New().MungeNoProtocolURL(s, uri)
	if err != nil {
		return nil, err
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
