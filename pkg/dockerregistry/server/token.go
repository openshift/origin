package server

import (
	"encoding/json"
	"net/http"

	context "github.com/docker/distribution/context"

	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/openshift/origin/pkg/client"
)

type tokenHandler struct {
	ctx             context.Context
	anonymousConfig restclient.Config
}

// NewTokenHandler returns a handler that implements the docker token protocol
func NewTokenHandler(ctx context.Context, client RegistryClient) http.Handler {
	return &tokenHandler{
		ctx:             ctx,
		anonymousConfig: client.SafeClientConfig(),
	}
}

// bearer token issued to token requests that present no credentials
// recognized by the openshift auth provider as identifying the anonymous user
const anonymousToken = "anonymous"

func (t *tokenHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := context.WithRequest(t.ctx, req)

	// If no authorization is provided, return a token the auth provider will treat as an anonymous user
	if len(req.Header.Get("Authorization")) == 0 {
		context.GetRequestLogger(ctx).Debugf("anonymous token request")
		t.writeToken(anonymousToken, w, req)
		return
	}

	// use the password as the token
	_, token, ok := req.BasicAuth()
	if !ok {
		context.GetRequestLogger(ctx).Debugf("no basic auth credentials provided")
		t.writeUnauthorized(w, req)
		return
	}

	// TODO: if this doesn't validate as an API token, attempt to obtain an API token using the given username/password
	copied := t.anonymousConfig
	copied.BearerToken = token
	osClient, err := client.New(&copied)
	if err != nil {
		context.GetRequestLogger(ctx).Errorf("error building client: %v", err)
		t.writeError(w, req)
		return
	}

	if _, err := osClient.Users().Get("~"); err != nil {
		context.GetRequestLogger(ctx).Debugf("invalid token: %v", err)
		t.writeUnauthorized(w, req)
		return
	}

	t.writeToken(token, w, req)
}

func (t *tokenHandler) writeError(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid_request"})
}

func (t *tokenHandler) writeToken(token string, w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":        token,
		"access_token": token,
	})
}

func (t *tokenHandler) writeUnauthorized(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
}
