package git

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func createLocalGitDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir(os.TempDir(), "s2i-test")
	if err != nil {
		t.Error(err)
	}
	os.Mkdir(filepath.Join(dir, ".git"), 0600)
	return dir
}

func TestParseRepository(t *testing.T) {
	gitLocalDir := createLocalGitDirectory(t)
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

	for scenario, test := range tests {
		out, err := ParseRepository(scenario)
		if err != nil {
			t.Errorf("ParseRepository returned err: %v", err)
		}

		// reflect.DeepEqual was not dealing with url.URL correctly, have to check each field individually
		// First, the easy string compares
		equal := out.Scheme == test.Scheme && out.Opaque == test.Opaque && out.Host == test.Host && out.Path == test.Path && out.RawQuery == test.RawQuery && out.Fragment == test.Fragment
		if equal {
			// now deal with User, a Userinfo struct ptr
			if out.User == nil && test.User != nil {
				equal = false
			} else if out.User != nil && test.User == nil {
				equal = false
			} else if out.User != nil && test.User != nil {
				equal = out.User.String() == test.User.String()
			}
		}
		if !equal {
			t.Errorf("For URL string %s, field by field check for scheme %v opaque %v host %v path %v rawq %v frag %v out user nil %v test user nil %v out scheme  %s out opaque %s out host %s out path %s  out raw query %s out frag %s", scenario,
				out.Scheme == test.Scheme, out.Opaque == test.Opaque, out.Host == test.Host, out.Path == test.Path, out.RawQuery == test.RawQuery,
				out.Fragment == test.Fragment, out.User == nil, test.User == nil, out.Scheme, out.Opaque, out.Host, out.Path, out.RawQuery, out.Fragment)
		}
	}
}
