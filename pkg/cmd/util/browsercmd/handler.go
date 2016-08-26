package browsercmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/pborman/uuid"
)

type HandlerImplementation struct {
	success  chan *osincli.AccessData
	failure  chan error
	oaClient *osincli.Client
	state    string
}

type CreateHandlerImplementation struct {
	rt         http.RoundTripper
	masterAddr string
}

func (h *HandlerImplementation) HandleSuccess(w http.ResponseWriter, req *http.Request) {
	glog.V(4).Info("Successful browser CLI OAuth flow.")
	w.Write([]byte("Success.  You may close this tab now."))
}

func (h *HandlerImplementation) HandleError(err error, w http.ResponseWriter, req *http.Request) {
	glog.V(4).Infof("Error during browser OAuth CLI flow: %q", err.Error())
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(fmt.Sprintf("Failure.  The error message was %q.  Please see the CLI for manual instructions.", err.Error())))
}

func (h *HandlerImplementation) HandleData(data *osincli.AuthorizeData) error {
	if !h.CheckState(data) {
		err := errors.New("State is invalid")
		h.failure <- err
		return err
	}
	// TODO support PKCE using state?
	// https://tools.ietf.org/html/draft-ietf-oauth-native-apps-03#section-8.2
	tokenReq := h.oaClient.NewAccessRequest(osincli.AUTHORIZATION_CODE, data)
	token, err := tokenReq.GetToken()
	if err != nil {
		h.failure <- err
	} else {
		h.success <- token
	}
	return err
}

func (h *HandlerImplementation) HandleRequest(w http.ResponseWriter, req *http.Request) {
	var success = true
	var data = new(osincli.AuthorizeData)
	var err error

	data, err = h.oaClient.NewAuthorizeRequest(osincli.CODE).HandleRequest(req)
	if err != nil {
		success = false
	} else {
		err = h.HandleData(data)
		if err != nil {
			success = false
		}
	}
	if success {
		h.HandleSuccess(w, req)
	} else {
		h.HandleError(err, w, req)
	}
}

func (h *HandlerImplementation) GetData() (*osincli.AccessData, error) {
	select {
	case ad := <-h.success:
		return ad, nil
	case err := <-h.failure:
		return nil, err
	}
}

func (h *HandlerImplementation) GenerateState() string {
	if h.state == "" {
		h.state = base64.URLEncoding.EncodeToString([]byte(uuid.New()))
	}
	return h.state
}

func (h *HandlerImplementation) CheckState(data *osincli.AuthorizeData) bool {
	return data.State == h.state && data.State != ""
}

func (chi *CreateHandlerImplementation) Create(port string) (Handler, error) {
	oaClientConfig := &osincli.ClientConfig{
		ClientId: origin.OpenShiftCLIClientID, //TODO should we import origin or just hard code?
		//ClientSecret: "none",                      //"8ee4f8bf-c7bc-4ca1-80f1-2ec7ff5af937", //TODO fix
		RedirectUrl:  fmt.Sprintf("http://127.0.0.1:%s/token", port),
		AuthorizeUrl: origin.OpenShiftOAuthAuthorizeURL(chi.masterAddr),
		TokenUrl:     origin.OpenShiftOAuthTokenURL(chi.masterAddr),
	}
	oaClient, err := osincli.NewClient(oaClientConfig)
	if err != nil {
		return nil, err
	}
	oaClient.Transport = chi.rt
	success := make(chan *osincli.AccessData, 1)
	failure := make(chan error, 1)
	return &HandlerImplementation{success, failure, oaClient, ""}, nil
}

func NewCreateHandler(rt http.RoundTripper, masterAddr string) CreateHandler {
	return &CreateHandlerImplementation{rt, masterAddr}
}
