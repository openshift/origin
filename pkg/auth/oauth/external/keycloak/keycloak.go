package keycloak

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/external"
)

type provider struct {
	Realm          string `json:"realm"`
	RealmPublicKey string `json:"realm-public-key"`
	ClientID       string `json:"resource"`
	Credentials    struct {
		Secret string `json:"secret"`
	} `json:"credentials"`
	AuthServerURL string `json:"auth-server-url"`
}

type keycloakUser struct {
	id         string
	username   string
	email      string
	firstName  string
	lastName   string
	realmRoles string
}

func NewProviderFromFile(keycloakClientConfigFile string) (external.Provider, error) {
	keycloakClientConfigBytes, err := ioutil.ReadFile(keycloakClientConfigFile)
	if err != nil {
		glog.Errorf("Error loading Keycloak config: %s", err)
		return nil, err
	}
	return NewProviderFromBytes(keycloakClientConfigBytes)
}

func NewProviderFromBytes(keycloakClientConfigBytes []byte) (external.Provider, error) {
	p := provider{}
	err := json.Unmarshal(keycloakClientConfigBytes, &p)
	if err != nil {
		glog.Errorf("Error parsing Keycloak config: %s", err)
		return nil, err
	}
	return p, nil
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p provider) NewConfig() (*osincli.ClientConfig, error) {
	realmURL, err := url.Parse(p.AuthServerURL)
	if err != nil {
		return nil, err
	}
	realmURL.Path += "/realms/" + p.Realm + "/"
	loginPath := url.URL{
		Path: "tokens/login",
	}
	tokenPath := url.URL{
		Path: "tokens/access/codes",
	}
	config := &osincli.ClientConfig{
		ClientId:                 p.ClientID,
		ClientSecret:             p.Credentials.Secret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             realmURL.ResolveReference(&loginPath).String(),
		TokenUrl:                 realmURL.ResolveReference(&tokenPath).String(),
	}
	return config, nil
}

// AddCustomParameters implements external/interfaces/Provider.AddCustomParameters
func (p provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
}

// GetUserIdentity implements external/interfaces/Provider.GetUserIdentity
func (p provider) GetUserIdentity(data *osincli.AccessData) (authapi.UserIdentityInfo, bool, error) {
	idToken, ok := data.ResponseData["id_token"].(string)
	if !ok {
		return nil, false, fmt.Errorf("No id_token returned in %v", data.ResponseData)
	}

	userdata, err := decodeJWT(idToken)
	glog.V(4).Infof("userdata=%v", userdata)
	if err != nil {
		return nil, false, err
	}

	username, _ := userdata["preferred_username"].(string)
	if username == "" {
		return nil, false, errors.New("Could not retrieve Keycloak username")
	}

	identity := &authapi.DefaultUserIdentityInfo{
		UserName: username,
		Extra: map[string]string{
			"name":  userdata["name"].(string),
			"email": userdata["email"].(string),
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
