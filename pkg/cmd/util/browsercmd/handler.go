package browsercmd

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/RangelReale/osincli"
	"github.com/openshift/origin/pkg/cmd/server/origin"
)

type HandlerImplementation struct {
	ad       chan *osincli.AccessData
	done     chan struct{}
	oaClient *osincli.Client
	state    string
}

type CreateHandlerImplementation struct {
	rt         http.RoundTripper
	masterAddr string
	state      string
}

func (h *HandlerImplementation) HandleError(err error) error {
	return err
}

func (h *HandlerImplementation) HandleData(data *osincli.AuthorizeData) error {
	if data.State != h.state {
		return errors.New("State error")
	}
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

func (chi *CreateHandlerImplementation) Create(port string) (Handler, error) {
	oaClientConfig := &osincli.ClientConfig{
		ClientId:     origin.OpenShiftCLIClientID,
		ClientSecret: "8ee4f8bf-c7bc-4ca1-80f1-2ec7ff5af937", //TODO fix
		RedirectUrl:  fmt.Sprintf("http://127.0.0.1:%s/token", port),
		AuthorizeUrl: origin.OpenShiftOAuthAuthorizeURL(chi.masterAddr),
		TokenUrl:     origin.OpenShiftOAuthTokenURL(chi.masterAddr),
	}
	oaClient, err := osincli.NewClient(oaClientConfig)
	if err != nil {
		return nil, err
	}
	oaClient.Transport = chi.rt
	ch := make(chan *osincli.AccessData, 1)
	done := make(chan struct{}, 1)
	return &HandlerImplementation{ch, done, oaClient, chi.state}, nil
}

func NewCreateHandlerImplementation(rt http.RoundTripper, masterAddr, state string) *CreateHandlerImplementation {
	return &CreateHandlerImplementation{rt, masterAddr, state}
}
