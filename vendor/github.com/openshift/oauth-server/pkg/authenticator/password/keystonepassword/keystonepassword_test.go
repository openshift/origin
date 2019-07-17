package keystonepassword

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	th "github.com/gophercloud/gophercloud/testhelper"
	"github.com/openshift/oauth-server/pkg/api"
	"k8s.io/apiserver/pkg/authentication/user"
)

// This type emulates a mapper with "claim" provisioning strategy
type TestUserIdentityMapperClaim struct {
	idnts map[string]string
}

func (m *TestUserIdentityMapperClaim) UserFor(identityInfo api.UserIdentityInfo) (user.Info, error) {
	userName := identityInfo.GetProviderUserName()
	if login, ok := identityInfo.GetExtra()[api.IdentityPreferredUsernameKey]; ok && len(login) > 0 {
		userName = login
	}
	claimedIdentityName := identityInfo.GetProviderName() + ":" + identityInfo.GetProviderUserName()

	if identityName, ok := m.idnts[userName]; ok && identityName != claimedIdentityName {
		// A user with that user name is already mapped to another identity
		return nil, fmt.Errorf("Ooops")
	}
	// Map the user with new identity
	m.idnts[userName] = claimedIdentityName

	return &user.DefaultInfo{Name: userName}, nil
}

var keystoneID string

func TestKeystoneLogin(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	const ID = "0123456789"

	th.Mux.HandleFunc("/v3/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Subject-Token", ID)
		type AuthRequest struct {
			Auth struct {
				Identity struct {
					Password struct {
						User struct {
							Domain   struct{ Name string }
							Name     string
							Password string
						}
					}
				}
			}
		}
		var x AuthRequest
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &x)
		domainName := x.Auth.Identity.Password.User.Domain.Name
		userName := x.Auth.Identity.Password.User.Name
		password := x.Auth.Identity.Password.User.Password
		if domainName == "default" && userName == "testuser" && password == "testpw" {
			w.WriteHeader(http.StatusCreated)
			resp := `{"token": {
							"methods": [
								"password"
							],
							"expires_at": "2015-11-09T01:42:57.527363Z",
							"user": {
								"domain": {
									"id": "default",
									"name": "Default"
								},
								"id": "` + keystoneID + `",
								"name": "admin",
								"password_expires_at": null
							},
							"audit_ids": [
								"lC2Wj1jbQe-dLjLyOx4qPQ"
							],
							"issued_at": "2015-11-09T00:42:57.527404Z"
						}
					}`
			fmt.Fprintf(w, resp)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
	// -----Test Claim strategy with enabled Keystone identity-----
	mapperClaim := TestUserIdentityMapperClaim{map[string]string{}}
	keystoneID = "initial_keystone_id"
	keystoneAuth := New("keystone_auth", th.Endpoint(), http.DefaultTransport, "default", &mapperClaim, true)

	// 1. User authenticates for the first time, new identity is created
	_, ok, err := keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "testpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, true, ok)
	th.CheckEquals(t, "keystone_auth:initial_keystone_id", mapperClaim.idnts["testuser"])

	// 2. Authentication with wrong or empty password fails
	_, ok, err = keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "badpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, false, ok)
	_, ok, err = keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, false, ok)

	// 3. Id of "testuser" has changed, authentication will fail
	keystoneID = "new_keystone_id"
	_, ok, err = keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "testpw")
	th.CheckEquals(t, false, ok)
	th.CheckEquals(t, "Ooops", err.Error())

	// -----Test Claim strategy with disabled Keystone identity-----
	mapperClaim = TestUserIdentityMapperClaim{map[string]string{}}
	keystoneID = "initial_keystone_id"
	keystoneAuth = New("keystone_auth", th.Endpoint(), http.DefaultTransport, "default", &mapperClaim, false)

	// 1. User authenticates for the first time, new identity is created
	_, ok, err = keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "testpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, true, ok)
	th.CheckEquals(t, "keystone_auth:testuser", mapperClaim.idnts["testuser"])

	// 2. Authentication with wrong or empty password fails
	_, ok, err = keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "badpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, false, ok)
	_, ok, err = keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, false, ok)

	// 3. Id of "testuser" has changed, authentication will work as before
	keystoneID = "new_keystone_id"
	_, ok, err = keystoneAuth.AuthenticatePassword(context.TODO(), "testuser", "testpw")
	th.CheckEquals(t, true, ok)
	th.AssertNoErr(t, err)
}
