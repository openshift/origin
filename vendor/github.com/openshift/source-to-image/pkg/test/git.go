package test

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util"
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

// LsTree returns a slice of os.FileInfo objects populated with the paths and
// file modes of files known to Git.  This is used on Windows systems where the
// executable mode metadata is lost on git checkout.
func (f *FakeGit) LsTree(repo, ref string, recursive bool) ([]os.FileInfo, error) {
	return []os.FileInfo{}, nil
}

// GetInfo retrieves the information about the source code and commit
func (f *FakeGit) GetInfo(repo string) *api.SourceInfo {
	return &api.SourceInfo{
		Ref:      "master",
		CommitID: "1bf4f04",
		Location: "file:///foo",
	}
}

// CreateLocalGitDirectory creates a git directory with a commit
func CreateLocalGitDirectory(t *testing.T) string {
	cr := util.NewCommandRunner()
	dir := CreateEmptyLocalGitDirectory(t)
	f, err := os.Create(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	err = cr.RunWithOptions(util.CommandOpts{Dir: dir}, "git", "add", ".")
	if err != nil {
		t.Fatal(err)
	}
	err = cr.RunWithOptions(util.CommandOpts{Dir: dir, EnvAppend: []string{"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test"}}, "git", "commit", "-m", "testcommit")
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

// CreateEmptyLocalGitDirectory creates a git directory with no checkin yet
func CreateEmptyLocalGitDirectory(t *testing.T) string {
	cr := util.NewCommandRunner()

	dir, err := ioutil.TempDir(os.TempDir(), "gitdir-s2i-test")
	if err != nil {
		t.Fatal(err)
	}
	err = cr.RunWithOptions(util.CommandOpts{Dir: dir}, "git", "init")
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

// CreateLocalGitDirectoryWithSubmodule creates a git directory with a submodule
func CreateLocalGitDirectoryWithSubmodule(t *testing.T) string {
	cr := util.NewCommandRunner()

	submodule := CreateLocalGitDirectory(t)
	defer os.RemoveAll(submodule)

	if util.UsingCygwinGit {
		var err error
		submodule, err = util.ToSlashCygwin(submodule)
		if err != nil {
			t.Fatal(err)
		}
	}

	dir := CreateEmptyLocalGitDirectory(t)
	err := cr.RunWithOptions(util.CommandOpts{Dir: dir}, "git", "submodule", "add", submodule, "submodule")
	if err != nil {
		t.Fatal(err)
	}

	return dir
}
