// +build !windows

package git

import (
	"net/url"
	"os"
	"testing"

	"github.com/openshift/source-to-image/pkg/test"
	"github.com/openshift/source-to-image/pkg/util"
)

// NB: MungeNoProtocolURL is only called by OpenShift, running on a Linux
// system.  It is unclear what its behaviour should be running on Windows / when
// passed Windows-style paths, therefore for now this test does not run on
// Windows builds.
// TODO: fix this.
func TestMungeNoProtocolURL(t *testing.T) {
	gitLocalDir := test.CreateLocalGitDirectory(t)
	defer os.RemoveAll(gitLocalDir)

	tests := map[string]url.URL{
		"git@github.com:user/repo.git": {
			Scheme: "ssh",
			Host:   "github.com",
			User:   url.User("git"),
			Path:   "user/repo.git",
		},
		"git://github.com/user/repo.git": {
			Scheme: "git",
			Host:   "github.com",
			Path:   "/user/repo.git",
		},
		"git://github.com/user/repo": {
			Scheme: "git",
			Host:   "github.com",
			Path:   "/user/repo",
		},
		"http://github.com/user/repo.git": {
			Scheme: "http",
			Host:   "github.com",
			Path:   "/user/repo.git",
		},
		"http://github.com/user/repo": {
			Scheme: "http",
			Host:   "github.com",
			Path:   "/user/repo",
		},
		"https://github.com/user/repo.git": {
			Scheme: "https",
			Host:   "github.com",
			Path:   "/user/repo.git",
		},
		"https://github.com/user/repo": {
			Scheme: "https",
			Host:   "github.com",
			Path:   "/user/repo",
		},
		"file://" + gitLocalDir: {
			Scheme: "file",
			Path:   gitLocalDir,
		},
		gitLocalDir: {
			Scheme: "file",
			Path:   gitLocalDir,
		},
		"git@192.168.122.1:repositories/authooks": {
			Scheme: "ssh",
			Host:   "192.168.122.1",
			User:   url.User("git"),
			Path:   "repositories/authooks",
		},
		"mbalazs@build.ulx.hu:/var/git/eap-ulx.git": {
			Scheme: "ssh",
			Host:   "build.ulx.hu",
			User:   url.User("mbalazs"),
			Path:   "/var/git/eap-ulx.git",
		},
		"ssh://git@[2001:db8::1]/repository.git": {
			Scheme: "ssh",
			Host:   "[2001:db8::1]",
			User:   url.User("git"),
			Path:   "/repository.git",
		},
		"ssh://git@mydomain.com:8080/foo/bar": {
			Scheme: "ssh",
			Host:   "mydomain.com:8080",
			User:   url.User("git"),
			Path:   "/foo/bar",
		},
		"git@[2001:db8::1]:repository.git": {
			Scheme: "ssh",
			Host:   "[2001:db8::1]",
			User:   url.User("git"),
			Path:   "repository.git",
		},
		"git@[2001:db8::1]:/repository.git": {
			Scheme: "ssh",
			Host:   "[2001:db8::1]",
			User:   url.User("git"),
			Path:   "/repository.git",
		},
	}

	gh := New(util.NewFileSystem())

	for scenario, test := range tests {
		uri, err := url.Parse(scenario)
		if err != nil {
			t.Errorf("Could not parse url %q", scenario)
			continue
		}

		err = gh.MungeNoProtocolURL(scenario, uri)
		if err != nil {
			t.Errorf("MungeNoProtocolURL returned err: %v", err)
			continue
		}

		// reflect.DeepEqual was not dealing with url.URL correctly, have to check each field individually
		// First, the easy string compares
		equal := uri.Scheme == test.Scheme && uri.Opaque == test.Opaque && uri.Host == test.Host && uri.Path == test.Path && uri.RawQuery == test.RawQuery && uri.Fragment == test.Fragment
		if equal {
			// now deal with User, a Userinfo struct ptr
			if uri.User == nil && test.User != nil {
				equal = false
			} else if uri.User != nil && test.User == nil {
				equal = false
			} else if uri.User != nil && test.User != nil {
				equal = uri.User.String() == test.User.String()
			}
		}
		if !equal {
			t.Errorf(`URL string %q, field by field check:
- Scheme: got %v, ok? %v
- Opaque: got %v, ok? %v
- Host: got %v, ok? %v
- Path: got %v, ok? %v
- RawQuery: got %v, ok? %v
- Fragment: got %v, ok? %v
- User: got %v`,
				scenario,
				uri.Scheme, uri.Scheme == test.Scheme,
				uri.Opaque, uri.Opaque == test.Opaque,
				uri.Host, uri.Host == test.Host,
				uri.Path, uri.Path == test.Path,
				uri.RawQuery, uri.RawQuery == test.RawQuery,
				uri.Fragment, uri.Fragment == test.Fragment,
				uri.User)
		}
	}
}
