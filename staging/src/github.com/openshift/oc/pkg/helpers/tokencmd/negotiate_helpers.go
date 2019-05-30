package tokencmd

import (
	"errors"
	"net/url"
)

func getServiceName(sep rune, requestURL string) (string, error) {
	u, err := url.Parse(requestURL)
	if err != nil {
		return "", err
	}

	return "HTTP" + string(sep) + u.Hostname(), nil
}

type negotiateUnsupported struct {
	error
}

func newUnsupportedNegotiator(name string) Negotiator {
	return &negotiateUnsupported{error: errors.New(name + " support is not enabled")}
}

func (n *negotiateUnsupported) Load() error {
	return n
}

func (n *negotiateUnsupported) InitSecContext(requestURL string, challengeToken []byte) ([]byte, error) {
	return nil, n
}

func (*negotiateUnsupported) IsComplete() bool {
	return false
}

func (n *negotiateUnsupported) Release() error {
	return n
}
