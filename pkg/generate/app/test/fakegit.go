package test

import (
	"io"
)

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

func (g *FakeGit) GetOriginURL(dir string) (string, bool, error) {
	return g.GitURL, true, nil
}

func (g *FakeGit) GetRef(dir string) string {
	return g.Ref
}

func (g *FakeGit) Clone(dir string, url string) error {
	g.CloneCalled = true
	return nil
}

func (g *FakeGit) CloneBare(dir string, url string) error {
	g.CloneCalled = true
	return nil
}

func (g *FakeGit) CloneMirror(source, target string) error {
	return nil
}

func (g *FakeGit) Checkout(dir string, ref string) error {
	g.CheckoutCalled = true
	return nil
}

func (f *FakeGit) Fetch(source string) error {
	return nil
}

func (f *FakeGit) Init(source string, _ bool) error {
	return nil
}

func (f *FakeGit) AddLocalConfig(source, key, value string) error {
	return nil
}

func (f *FakeGit) Archive(source, ref, format string, w io.Writer) error {
	return nil
}

func (f *FakeGit) AddRemote(source, remote, url string) error {
	return nil
}

func (f *FakeGit) ShowFormat(source, ref, format string) (string, error) {
	return "", nil
}
