package basicauthpassword

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

// Authenticator uses basic auth to make a request to a JSON-returning URL.
// A 401 status indicate failed auth.
// A non-200 status or the presence of an "error" key with a non-empty
//   value indicates an error:
//   {"error":"Error message"}
// A 200 status with an "id" key indicates success:
//   {"id":"userid"}
// A successful response may also include name and/or email:
//   {"id":"userid", "name": "User Name", "email":"user@example.com"}
type Authenticator struct {
	providerName string
	url          string
	client       *http.Client
	mapper       authapi.UserIdentityMapper
}

// RemoteUserData holds user data returned from a remote basic-auth protected endpoint.
// These field names can not be changed unless external integrators are also updated.
// Names are based on standard OpenID Connect claims: http://openid.net/specs/openid-connect-core-1_0.html#StandardClaims
type RemoteUserData struct {
	// Subject - Identifier for the End-User at the Issuer. Required.
	Subject string `json:"sub"`
	// Name is the end-User's full name in displayable form including all name parts, possibly including titles and suffixes,
	// ordered according to the End-User's locale and preferences.  Optional.
	Name string `json:"name"`
	// PreferredUsername is a shorthand name by which the End-User wishes to be referred. Optional.
	// Useful when the immutable subject is different than the login used by the user to authenticate
	PreferredUsername string `json:"preferred_username"`
	// Email is the end-User's preferred e-mail address. Optional.
	Email string `json:"email"`
}

// RemoteError holds error data returned from a remote authentication request
type RemoteError struct {
	Error string
}

// New returns an authenticator which will make a basic auth call to the given url.
// A custom transport can be provided (typically to customize TLS options like trusted roots or present a client certificate).
// If no transport is provided, http.DefaultTransport is used
func New(providerName string, url string, transport http.RoundTripper, mapper authapi.UserIdentityMapper) authenticator.Password {
	if transport == nil {
		transport = http.DefaultTransport
	}
	client := &http.Client{Transport: transport}
	return &Authenticator{providerName, url, client, mapper}
}

func (a *Authenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	req, err := http.NewRequest("GET", a.url, nil)
	if err != nil {
		return nil, false, err
	}

	req.SetBasicAuth(username, password)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, false, nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	remoteError := RemoteError{}
	json.Unmarshal(body, &remoteError)
	if remoteError.Error != "" {
		return nil, false, errors.New(remoteError.Error)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("An error occurred while authenticating (%d)", resp.StatusCode)
	}

	remoteUserData := RemoteUserData{}
	err = json.Unmarshal(body, &remoteUserData)
	if err != nil {
		return nil, false, err
	}

	if len(remoteUserData.Subject) == 0 {
		return nil, false, errors.New("Could not retrieve user data")
	}
	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, remoteUserData.Subject)

	if len(remoteUserData.Name) > 0 {
		identity.Extra[authapi.IdentityDisplayNameKey] = remoteUserData.Name
	}
	if len(remoteUserData.PreferredUsername) > 0 {
		identity.Extra[authapi.IdentityPreferredUsernameKey] = remoteUserData.PreferredUsername
	}
	if len(remoteUserData.Email) > 0 {
		identity.Extra[authapi.IdentityEmailKey] = remoteUserData.Email
	}

	user, err := a.mapper.UserFor(identity)
	if err != nil {
		return nil, false, fmt.Errorf("Error creating or updating mapping for: %#v due to %v", identity, err)
	}
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)

	return user, true, nil
}
