package basicauthpassword

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
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
	url    string
	mapper authapi.UserIdentityMapper
}

// RemoteUserData holds user data returned from a remote basic-auth protected endpoint.
// These field names can not be changed unless external integrators are also updated.
type RemoteUserData struct {
	ID    string
	Name  string
	Email string
}

// RemoteError holds error data returned from a remote authentication request
type RemoteError struct {
	Error string
}

// New returns an authenticator which will make a basic auth call to the given url.
func New(url string, mapper authapi.UserIdentityMapper) authenticator.Password {
	return &Authenticator{url, mapper}
}

func (a *Authenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	req, err := http.NewRequest("GET", a.url, nil)
	if err != nil {
		return nil, false, err
	}

	req.SetBasicAuth(username, password)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}

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

	if len(remoteUserData.ID) == 0 {
		return nil, false, errors.New("Could not retrieve user data")
	}

	identity := &authapi.DefaultUserIdentityInfo{
		UserName: username,
		Extra: map[string]string{
			"name":  remoteUserData.Name,
			"email": remoteUserData.Email,
		},
	}
	user, err := a.mapper.UserFor(identity)
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)
	if err != nil {
		return nil, false, fmt.Errorf("Error creating or updating mapping for: %#v due to %v", identity, err)
	}

	return user, true, nil
}
