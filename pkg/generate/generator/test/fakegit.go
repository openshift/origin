package test

type FakeGit struct {
	RootDir        string
	GitURL         string
	Ref            string
	CloneCalled    bool
	CheckoutCalled bool
}

func (g *FakeGit) GetRootDir(dir string) (string, error) {
	return g.RootDir, nil
}

func (g *FakeGit) GetOriginURL(dir string) (string, error) {
	return g.GitURL, nil
}

func (g *FakeGit) GetRef(dir string) string {
	return g.Ref
}

func (g *FakeGit) Clone(dir string, url string) error {
	g.CloneCalled = true
	return nil
}

func (g *FakeGit) Checkout(dir string, ref string) error {
	g.CheckoutCalled = true
	return nil
}
