package browsercmd

import "github.com/pkg/browser"

type BrowserImplementation struct{}

func (*BrowserImplementation) Open(rawurl string) error {
	return browser.OpenURL(rawurl)
}
