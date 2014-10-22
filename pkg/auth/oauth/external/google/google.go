package google

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/external"
)

const (
	googleAuthorizeUrl = "https://accounts.google.com/o/oauth2/auth"
	googleTokenUrl     = "https://accounts.google.com/o/oauth2/token"
	googleOauthScope   = "profile email"
)

type provider struct {
	client_id, client_secret string
}

func NewProvider(client_id, client_secret string) external.Provider {
	return provider{client_id, client_secret}
}

func (p provider) NewConfig() (*osincli.ClientConfig, error) {
	config := &osincli.ClientConfig{
		ClientId:                 p.client_id,
		ClientSecret:             p.client_secret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             googleAuthorizeUrl,
		TokenUrl:                 googleTokenUrl,
		Scope:                    googleOauthScope,
	}
	return config, nil
}

func (p provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
	req.CustomParameters["include_granted_scopes"] = "true"
	req.CustomParameters["access_type"] = "offline"
}

func (p provider) GetUserInfo(data *osincli.AccessData) (api.UserInfo, bool, error) {
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
		return nil, false, fmt.Errorf("Could not retrieve Google id")
	}

	user := &api.DefaultUserInfo{
		Name: id,
		Extra: map[string]string{
			"name":  email,
			"email": email,
		},
	}
	return user, true, nil
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
