package git

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// According to git-clone(1), a "Git URL" can be in one of three broad types:
// 1) A standards-compliant URL.
//    a) The scheme may be followed by '://',
//       e.g. https://github.com/openshift/origin, file:///foo/bar, etc.  In
//       this case, note that among other things, a standards-compliant file URL
//       must have an empty host part, an absolute path and no backslashes.  The
//       Git for Windows URL parser accepts many non-compliant URLs, but we
//       don't.
//    b) Alternatively, the scheme may be followed by '::', in which case it is
//       treated as an transport/opaque address pair, e.g.
//       http::http://github.com/openshift/origin.git .
// 2) The "alternative scp-like syntax", including a ':' with no preceding '/',
//    but not of the form C:... on Windows, e.g.
//    git@github.com:openshift/origin, etc.
// 3) An OS-specific relative or absolute local file path, e.g. foo/bar,
//    C:\foo\bar, etc.
//
// We extend all of the above URL types to additionally accept an optional
// appended #fragment, which is given to specify a git reference.
//
// The git client allows Git URL rewriting rules to be defined.  The meaning of
// a Git URL cannot be 100% guaranteed without consulting the rewriting rules.

// URLType indicates the type of the URL (see above)
type URLType int

const (
	// URLTypeURL is the URL type (see above)
	URLTypeURL URLType = iota
	// URLTypeSCP is the SCP type (see above)
	URLTypeSCP
	// URLTypeLocal is the local type (see above)
	URLTypeLocal
)

// String returns a string representation of the URLType
func (t URLType) String() string {
	switch t {
	case URLTypeURL:
		return "URLTypeURL"
	case URLTypeSCP:
		return "URLTypeSCP"
	case URLTypeLocal:
		return "URLTypeLocal"
	}
	panic("unknown URLType")
}

// GoString returns a Go string representation of the URLType
func (t URLType) GoString() string {
	return t.String()
}

// URL represents a "Git URL"
type URL struct {
	URL  url.URL
	Type URLType
}

var urlSchemeRegexp = regexp.MustCompile("(?i)^[a-z][-a-z0-9+.]*:") // matches scheme: according to RFC3986
var dosDriveRegexp = regexp.MustCompile("(?i)^[a-z]:")
var scpRegexp = regexp.MustCompile("^" +
	"(?:([^@/]*)@)?" + // user@ (optional)
	"([^/]*):" + //            host:
	"(.*)" + //                     path
	"$")

func splitOnByte(s string, c byte) (string, string) {
	if i := strings.IndexByte(s, c); i != -1 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// Parse parses a "Git URL"
func Parse(rawurl string) (*URL, error) {
	if urlSchemeRegexp.MatchString(rawurl) &&
		(runtime.GOOS != "windows" || !dosDriveRegexp.MatchString(rawurl)) {
		u, err := url.Parse(rawurl)
		if err != nil {
			return nil, err
		}

		if u.Scheme == "file" && u.Opaque == "" {
			if u.Host != "" {
				return nil, fmt.Errorf("url %q has non-empty host", rawurl)
			}
			if runtime.GOOS == "windows" && (len(u.Path) == 0 || !filepath.IsAbs(u.Path[1:])) {
				return nil, fmt.Errorf("url %q has non-absolute path", rawurl)
			}
		}

		return &URL{
			URL:  *u,
			Type: URLTypeURL,
		}, nil
	}

	s, fragment := splitOnByte(rawurl, '#')

	if m := scpRegexp.FindStringSubmatch(s); m != nil &&
		(runtime.GOOS != "windows" || !dosDriveRegexp.MatchString(s)) {
		u := &url.URL{
			Host:     m[2],
			Path:     m[3],
			Fragment: fragment,
		}
		if m[1] != "" {
			u.User = url.User(m[1])
		}

		return &URL{
			URL:  *u,
			Type: URLTypeSCP,
		}, nil
	}

	return &URL{
		URL: url.URL{
			Path:     s,
			Fragment: fragment,
		},
		Type: URLTypeLocal,
	}, nil
}

// MustParse parses a "Git URL" and panics on failure
func MustParse(rawurl string) *URL {
	u, err := Parse(rawurl)
	if err != nil {
		panic(err)
	}
	return u
}

// String returns a string representation of the URL
func (u URL) String() string {
	var s string
	switch u.Type {
	case URLTypeURL:
		return u.URL.String()
	case URLTypeSCP:
		if u.URL.User != nil {
			s = u.URL.User.Username() + "@"
		}
		s += u.URL.Host + ":" + u.URL.Path
	case URLTypeLocal:
		s = u.URL.Path
	}
	if u.URL.RawQuery != "" {
		s += "?" + u.URL.RawQuery
	}
	if u.URL.Fragment != "" {
		s += "#" + u.URL.Fragment
	}
	return s
}

// StringNoFragment returns a string representation of the URL without its
// fragment
func (u URL) StringNoFragment() string {
	u.URL.Fragment = ""
	return u.String()
}

// IsLocal returns true if the Git URL refers to a local repository
func (u URL) IsLocal() bool {
	return u.Type == URLTypeLocal || (u.Type == URLTypeURL && u.URL.Scheme == "file" && u.URL.Opaque == "")
}

// LocalPath returns the path to a local repository in OS-native format.  It is
// assumed that IsLocal() is true
func (u URL) LocalPath() string {
	switch {
	case u.Type == URLTypeLocal:
		return u.URL.Path
	case u.Type == URLTypeURL && u.URL.Scheme == "file" && u.URL.Opaque == "":
		if runtime.GOOS == "windows" && len(u.URL.Path) > 0 && u.URL.Path[0] == '/' {
			return filepath.FromSlash(u.URL.Path[1:])
		}
		return filepath.FromSlash(u.URL.Path)
	}
	panic("LocalPath called on non-local URL")
}
