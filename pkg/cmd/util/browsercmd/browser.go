package browsercmd

import "github.com/pkg/browser"

type BrowserImplementation struct{}

func (*BrowserImplementation) Open(rawURL string) error {
	return browser.OpenURL(rawURL)
}

func NewBrowser() Browser {
	return &BrowserImplementation{}
}
