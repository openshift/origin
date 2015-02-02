package google

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/external"
)

const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL     = "https://accounts.google.com/o/oauth2/token"
	googleOAuthScope   = "profile email"
)

type provider struct {
	clientID, clientSecret string
}

func NewProvider(clientID, clientSecret string) external.Provider {
	return provider{clientID, clientSecret}
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p provider) NewConfig() (*osincli.ClientConfig, error) {
	config := &osincli.ClientConfig{
		ClientId:                 p.clientID,
		ClientSecret:             p.clientSecret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             googleAuthorizeURL,
		TokenUrl:                 googleTokenURL,
		Scope:                    googleOAuthScope,
	}
	return config, nil
}

// AddCustomParameters implements external/interfaces/Provider.AddCustomParameters
func (p provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
	req.CustomParameters["include_granted_scopes"] = "true"
	req.CustomParameters["access_type"] = "offline"
}

// GetUserIdentity implements external/interfaces/Provider.GetUserIdentity
func (p provider) GetUserIdentity(data *osincli.AccessData) (authapi.UserIdentityInfo, bool, error) {
	idToken, ok := data.ResponseData["id_token"].(string)
	if !ok {
		return nil, false, fmt.Errorf("No id_token returned in %v", data.ResponseData)
	}

	userdata, err := decodeJWT(idToken)
	if err != nil {
		return nil, false, err
	}

	id, _ := userdata["id"].(string)
	email, _ := userdata["email"].(string)
	if id == "" || email == "" {
		return nil, false, errors.New("Could not retrieve Google id")
	}

	identity := &authapi.DefaultUserIdentityInfo{
		UserName: id,
		Extra: map[string]string{
			"name":  email,
			"email": email,
		},
	}
	glog.V(4).Infof("identity=%v", identity)

	return identity, true, nil
}

// Decode JWT
// http://openid.net/specs/draft-jones-json-web-token-07.html
func decodeJWT(jwt string) (map[string]interface{}, error) {
	jwtParts := strings.Split(jwt, ".")
	if len(jwtParts) != 3 {
		return nil, fmt.Errorf("Invalid JSON Web Token: expected 3 parts, got %d", len(jwtParts))
	}

	encodedPayload := jwtParts[1]
	glog.V(4).Infof("got encodedPayload")

	// Re-pad, if needed
	if l := len(encodedPayload) % 4; l != 0 {
		padding := strings.Repeat("=", 4-l)
		encodedPayload += padding
		glog.V(4).Infof("added padding: %s\n", padding)
	}

	decodedPayload, err := base64.StdEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("Error decoding payload: %v\n", err)
	}
	glog.V(4).Infof("got decodedPayload")

	var data map[string]interface{}
	err = json.Unmarshal([]byte(decodedPayload), &data)
	if err != nil {
		return nil, fmt.Errorf("Error parsing token: %v\n", err)
	}
	glog.V(4).Infof("got id_token data")

	return data, nil
}
