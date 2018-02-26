package keystonepassword

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/openshift/api/user/v1"
	th "github.com/rackspace/gophercloud/testhelper"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type TestUserIdentityMapper struct{}

func (m *TestUserIdentityMapper) UserFor(identityInfo api.UserIdentityInfo) (user.Info, error) {
	return &user.DefaultInfo{Name: identityInfo.GetProviderUserName()}, nil
}

type TestUser struct {}

var idnts = []string{}

func (c *TestUser) Get(name string, options metav1.GetOptions) (result *v1.User, err error) {
	u := v1.User{}
	u.Identities = idnts
	return &u, nil
}

// Stubs to make TestUser compatible with UserInterface
func (c *TestUser) List(opts metav1.ListOptions) (result *v1.UserList, err error) {return &v1.UserList{}, nil}
func (c *TestUser) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return watch.NewEmptyWatch(), nil
}
func (c *TestUser) Create(user *v1.User) (result *v1.User, err error) {return &v1.User{}, nil}
func (c *TestUser) Update(user *v1.User) (result *v1.User, err error) {return &v1.User{}, nil}
func (c *TestUser) Delete(name string, options *metav1.DeleteOptions) error {return nil}
func (c *TestUser) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {return nil}
func (c *TestUser) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.User, err error) {return &v1.User{}, nil}


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
								"id": "ee4dfb6e5540447cb3741905149d9b6e",
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

	keystoneAuth := New("keystone_auth", th.Endpoint(), http.DefaultTransport, "default", &TestUserIdentityMapper{}, &TestUser{})
	// correct password with correct identity in the database
	idnts = []string{"keystone_auth:ee4dfb6e5540447cb3741905149d9b6e"}
	_, ok, err := keystoneAuth.AuthenticatePassword("testuser", "testpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, ok, true)
	// correct password with invalid identity
	idnts = []string{"keystone_auth:wrong_keystone_id"}
	_, ok, err = keystoneAuth.AuthenticatePassword("testuser", "testpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, ok, false)
	_, ok, err = keystoneAuth.AuthenticatePassword("testuser", "badpw")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, ok, false)
	_, ok, err = keystoneAuth.AuthenticatePassword("testuser", "")
	th.AssertNoErr(t, err)
	th.CheckEquals(t, ok, false)
}
