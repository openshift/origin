package browsercmd

type BrowserImplementation struct{}

func (*BrowserImplementation) Open(rawurl string) error {
	return nil
}
