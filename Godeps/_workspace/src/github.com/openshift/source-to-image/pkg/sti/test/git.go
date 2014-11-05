package test

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

func (f *FakeGit) ValidCloneSpec(source string) bool {
	f.ValidCloneSpecSource = source
	return f.ValidCloneSpecResult
}

func (f *FakeGit) Clone(source, target string) error {
	f.CloneSource = source
	f.CloneTarget = target
	return f.CloneError
}

func (f *FakeGit) Checkout(repo, ref string) error {
	f.CheckoutRepo = repo
	f.CheckoutRef = ref
	return f.CheckoutError
}
