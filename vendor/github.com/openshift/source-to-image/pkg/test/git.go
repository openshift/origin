package test

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
)

// FakeGit provides a fake Git
type FakeGit struct {
	ValidCloneSpecSource string
	ValidCloneSpecResult bool

	CloneSource string
	CloneTarget string
	CloneError  error

	CheckoutRepo  string
	CheckoutRef   string
	CheckoutError error

	SubmoduleInitRepo  string
	SubmoduleInitError error

	SubmoduleUpdateRepo      string
	SubmoduleUpdateInit      bool
	SubmoduleUpdateRecursive bool
	SubmoduleUpdateError     error
}

// ValidCloneSpec returns a valid Git clone specification
func (f *FakeGit) ValidCloneSpec(source string) (bool, error) {
	f.ValidCloneSpecSource = source
	return f.ValidCloneSpecResult, nil
}

//ValidCloneSpecRemoteOnly returns a valid Git clone specification
func (f *FakeGit) ValidCloneSpecRemoteOnly(source string) bool {
	f.ValidCloneSpecSource = source
	return f.ValidCloneSpecResult
}

//MungeNoProtocolURL returns a valid no protocol Git URL
func (f *FakeGit) MungeNoProtocolURL(source string, url *url.URL) error {
	f.ValidCloneSpecSource = source
	return nil
}

// Clone clones the fake source Git repository to target directory
func (f *FakeGit) Clone(source, target string, c api.CloneConfig) error {
	f.CloneSource = source
	f.CloneTarget = target
	return f.CloneError
}

// Checkout checkouts a ref in the fake Git repository
func (f *FakeGit) Checkout(repo, ref string) error {
	f.CheckoutRepo = repo
	f.CheckoutRef = ref
	return f.CheckoutError
}

// SubmoduleInit initializes / clones submodules.
func (f *FakeGit) SubmoduleInit(repo string) error {
	f.SubmoduleInitRepo = repo
	return f.SubmoduleInitError
}

// SubmoduleUpdate checks out submodules to their correct version
func (f *FakeGit) SubmoduleUpdate(repo string, init, recursive bool) error {
	f.SubmoduleUpdateRepo = repo
	f.SubmoduleUpdateRecursive = recursive
	f.SubmoduleUpdateInit = init
	return f.SubmoduleUpdateError
}

func (f *FakeGit) GetInfo(repo string) *api.SourceInfo {
	return &api.SourceInfo{
		Ref:      "master",
		CommitID: "1bf4f04",
		Location: "file:///foo",
	}
}

// Creates a git directory with one unlikely but possible commit hash
func CreateLocalGitDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir(os.TempDir(), "gitdir-s2i-test")
	if err != nil {
		t.Error(err)
	}
	os.MkdirAll(filepath.Join(dir, ".git/refs/heads"), 0777)
	os.MkdirAll(filepath.Join(dir, ".git/refs/remotes"), 0777)
	os.MkdirAll(filepath.Join(dir, ".git/branches"), 0777)
	os.MkdirAll(filepath.Join(dir, ".git/objects/fo"), 0777)
	os.Create(filepath.Join(dir, ".git/objects/fo") + "12345678901234567890123456789012345678") // 40 character SHA-1 hash
	return dir
}

func CreateEmptyLocalGitDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir(os.TempDir(), "gitdir-s2i-test")
	if err != nil {
		t.Error(err)
	}
	os.MkdirAll(filepath.Join(dir, ".git/refs/heads"), 0777)
	os.MkdirAll(filepath.Join(dir, ".git/refs/remotes"), 0777)
	os.MkdirAll(filepath.Join(dir, ".git/branches"), 0777)
	os.MkdirAll(filepath.Join(dir, ".git/objects"), 0777)
	return dir
}
