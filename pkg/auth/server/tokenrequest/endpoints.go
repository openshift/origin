package tokenrequest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/RangelReale/osincli"

	"github.com/openshift/origin/pkg/auth/server/login"
)

const (
	RequestTokenEndpoint = "/token/request"
	DisplayTokenEndpoint = "/token/display"
)

type endpointDetails struct {
	// Jordan has done the osincli review and declares the osinclit.Client reusable and thread-safe
	originOAuthClient *osincli.Client
}

type Endpoints interface {
	Install(mux login.Mux, paths ...string)
}

func NewEndpoints(originOAuthClient *osincli.Client) Endpoints {
	return &endpointDetails{originOAuthClient}
}

// Install registers the request token endpoints into a mux. It is expected that the
// provided prefix will serve all operations. Path MUST NOT end in a slash.
func (endpoints *endpointDetails) Install(mux login.Mux, paths ...string) {
	for _, prefix := range paths {
		prefix = strings.TrimRight(prefix, "/")

		mux.HandleFunc(prefix+RequestTokenEndpoint, endpoints.requestToken)
		mux.HandleFunc(prefix+DisplayTokenEndpoint, endpoints.displayToken)
	}
}

// this works for getting a token in your browser and seeing what your token is
func (endpoints *endpointDetails) requestToken(w http.ResponseWriter, req *http.Request) {
	authReq := endpoints.originOAuthClient.NewAuthorizeRequest(osincli.CODE)
	oauthURL := authReq.GetAuthorizeUrlWithParams("")

	http.Redirect(w, req, oauthURL.String(), http.StatusFound)
}

func (endpoints *endpointDetails) displayToken(w http.ResponseWriter, req *http.Request) {
	authorizeReq := endpoints.originOAuthClient.NewAuthorizeRequest(osincli.CODE)
	authorizeData, err := authorizeReq.HandleRequest(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error handling auth request: %v", err)
		return
	}

	accessReq := endpoints.originOAuthClient.NewAccessRequest(osincli.AUTHORIZATION_CODE, authorizeData)
	accessData, err := accessReq.GetToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error getting token: %v", err)
		return
	}

	jsonBytes, err := json.Marshal(accessData.ResponseData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error marshalling json: %v", err)
		return
	}

	w.Write(jsonBytes)
}
