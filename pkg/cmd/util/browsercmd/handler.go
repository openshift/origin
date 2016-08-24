package browsercmd

import (
	"errors"
	"net/http"

	"github.com/RangelReale/osincli"
)

type HandlerImplementation struct {
	ad       chan *osincli.AccessData
	done     chan struct{}
	oaClient *osincli.Client
}

func (h *HandlerImplementation) HandleError(err error) error {
	return err
}

func (h *HandlerImplementation) HandleData(data *osincli.AuthorizeData) error {
	tokenReq := h.oaClient.NewAccessRequest(osincli.AUTHORIZATION_CODE, data)
	token, err := tokenReq.GetToken()
	if err != nil {
		h.done <- struct{}{}
		return err
	}
	h.ad <- token
	return nil
}

func (h *HandlerImplementation) HandleRequest(r *http.Request) (*osincli.AuthorizeData, error) {
	return h.oaClient.NewAuthorizeRequest(osincli.CODE).HandleRequest(r)
}

func (h *HandlerImplementation) GetAccessData() (*osincli.AccessData, error) {
	select {
	case ad := <-h.ad:
		return ad, nil
	case <-h.done:
		return nil, errors.New("FAIL")
	}
}

func NewHandlerImplementation(rt http.RoundTripper, serverURL string) (*HandlerImplementation, error) {
	oaClientConfig := &osincli.ClientConfig{
		ClientId:     "openshift-browser-client",             //TODO fix
		ClientSecret: "45a3d382-59ca-4ed5-b5e6-f90495fb59d9", //TODO fix
		RedirectUrl:  "http://127.0.0.1:80/token",
		AuthorizeUrl: serverURL + "/oauth/authorize",
		TokenUrl:     serverURL + "/oauth/token",
	}
	oaClient, err := osincli.NewClient(oaClientConfig)
	if err != nil {
		return nil, err
	}
	oaClient.Transport = rt
	ch := make(chan *osincli.AccessData, 1)
	done := make(chan struct{}, 1)
	return &HandlerImplementation{ch, done, oaClient}, nil
}
