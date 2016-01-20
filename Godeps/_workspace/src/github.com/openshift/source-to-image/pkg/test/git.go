package test

import (
	"github.com/openshift/source-to-image/pkg/api"
	"net/url"
)

// FakeGit provides a fake GIT
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

// ValidCloneSpec returns a valid GIT clone specification
func (f *FakeGit) ValidCloneSpec(source string) bool {
	f.ValidCloneSpecSource = source
	return f.ValidCloneSpecResult
}

//ValidCloneSpecRemoteOnly returns a valid GIT clone specification
func (f *FakeGit) ValidCloneSpecRemoteOnly(source string) bool {
	f.ValidCloneSpecSource = source
	return f.ValidCloneSpecResult
}

//MungeNoProtocolURL returns a valid no protocol GIT URL
func (f *FakeGit) MungeNoProtocolURL(source string, url *url.URL) error {
	f.ValidCloneSpecSource = source
	return nil
}

// Clone clones the fake source GIT repository to target directory
func (f *FakeGit) Clone(source, target string, c api.CloneConfig) error {
	f.CloneSource = source
	f.CloneTarget = target
	return f.CloneError
}

// Checkout checkouts a ref in the fake GIT repository
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
