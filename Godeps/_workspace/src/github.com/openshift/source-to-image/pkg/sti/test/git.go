package test

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
}

// ValidCloneSpec returns a valid GIT clone specification
func (f *FakeGit) ValidCloneSpec(source string) bool {
	f.ValidCloneSpecSource = source
	return f.ValidCloneSpecResult
}

// Clone clones the fake source GIT repository to target directory
func (f *FakeGit) Clone(source, target string) error {
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
