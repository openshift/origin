package git

import (
	"net/url"
	"os"
	"testing"

	"github.com/openshift/source-to-image/pkg/test"
)

func TestParseRepository(t *testing.T) {
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

	for scenario, want := range tests {
		got, err := ParseRepository(scenario)
		if err != nil {
			t.Errorf("ParseRepository returned err: %v", err)
		}

		// go1.5 added the RawPath field to url.URL; it is not a field we need to manipulate with the
		// ParseRepository path, but it impacts the values set in our test results array and doing a
		// DeepEqual compare; hence, we have reverted back to a field by field compare (which
		// this test originally did)
		if got.Scheme != want.Scheme ||
			got.Host != want.Host ||
			got.Path != want.Path ||
			(got.User != nil && want.User != nil && got.User.Username() != want.User.Username()) ||
			(got.User == nil && want.User != nil) ||
			(got.User != nil && want.User == nil) {
			t.Errorf("%s: got %#v, want %#v", scenario, *got, want)
		}
	}
}
