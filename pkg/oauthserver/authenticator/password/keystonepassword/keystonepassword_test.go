package keystonepassword

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/api"
	th "github.com/rackspace/gophercloud/testhelper"
	"k8s.io/apiserver/pkg/authentication/user"
)

type TestUserIdentityMapper struct{}

func (m *TestUserIdentityMapper) UserFor(identityInfo api.UserIdentityInfo) (user.Info, error) {
	return &user.DefaultInfo{Name: identityInfo.GetProviderUserName()}, nil
}

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
			fmt.Fprintf(w, `{ "token": { "expires_at": "2020-02-02T18:30:59.000000Z" } }`)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})

	keystoneAuth := New("keystone_auth", th.Endpoint(), http.DefaultTransport, "default", &TestUserIdentityMapper{})
	_, ok, err := keystoneAuth.AuthenticatePassword("testuser", "testpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, ok, true)
	_, ok, err = keystoneAuth.AuthenticatePassword("testuser", "badpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, ok, false)
	_, ok, err = keystoneAuth.AuthenticatePassword("testuser", "")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, ok, false)
}
