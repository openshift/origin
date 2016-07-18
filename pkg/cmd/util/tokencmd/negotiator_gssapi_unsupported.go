// +build !gssapi

package tokencmd

import "errors"

func GSSAPIEnabled() bool {
	return false
}

type gssapiUnsupported struct{}

func NewGSSAPINegotiator(principalName string) Negotiater {
	return &gssapiUnsupported{}
}

func (g *gssapiUnsupported) Load() error {
	return errors.New("GSSAPI support is not enabled")
}
func (g *gssapiUnsupported) InitSecContext(requestURL string, challengeToken []byte) (tokenToSend []byte, err error) {
	return nil, errors.New("GSSAPI support is not enabled")
}
func (g *gssapiUnsupported) IsComplete() bool {
	return false
}
func (g *gssapiUnsupported) Release() error {
	return errors.New("GSSAPI support is not enabled")
}
