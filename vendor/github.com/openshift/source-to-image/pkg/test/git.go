package test

import (
	"os"

	"github.com/openshift/source-to-image/pkg/scm/git"
)

// FakeGit provides a fake Git
type FakeGit struct {
	CloneSource *git.URL
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

// Clone clones the fake source Git repository to target directory
func (f *FakeGit) Clone(source *git.URL, target string, c git.CloneConfig) error {
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
func (f *FakeGit) GetInfo(repo string) *git.SourceInfo {
	return &git.SourceInfo{
		Ref:      "master",
		CommitID: "1bf4f04",
		Location: "file:///foo",
	}
}
