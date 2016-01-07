package git

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
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

	for scenario, want := range tests {
		got, err := ParseRepository(scenario)
		if err != nil {
			t.Errorf("ParseRepository returned err: %v", err)
		}

		if !reflect.DeepEqual(*got, want) {
			t.Errorf("%s: got %#v, want %#v", scenario, *got, want)
		}
	}
}
