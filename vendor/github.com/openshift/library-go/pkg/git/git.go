package git

import (
	"bufio"
	"io"
	"net/url"
	"path"
	"strings"
)

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
