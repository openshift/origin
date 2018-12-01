package session

import (
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/bootstrap"
)

const (
	userNameKey = "user.name"
	userUIDKey  = "user.uid"

	// expKey is stored as an int64 unix time
	expKey = "exp"
)

type Authenticator struct {
	store   Store
	maxAge  time.Duration
	secrets v1.SecretInterface
}

func NewAuthenticator(store Store, maxAge time.Duration, secrets v1.SecretsGetter) *Authenticator {
	return &Authenticator{
		store:   store,
		maxAge:  maxAge,
		secrets: secrets.Secrets(metav1.NamespaceSystem),
	}
}

func (a *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	values := a.store.Get(req)

	expires, ok := values.GetInt64(expKey)
	if !ok {
		return nil, false, nil
	}

	if expires < time.Now().Unix() {
		return nil, false, nil
	}

	name, ok := values.GetString(userNameKey)
	if !ok {
		return nil, false, nil
	}

	uid, ok := values.GetString(userUIDKey)
	if !ok {
		return nil, false, nil
	}

	// make sure that the password has not changed since this cookie was issued
	// note that this is not really for security - it is so that we do not annoy the user
	// by letting them log in successfully only to have a token that does not work
	if name == bootstrap.BootstrapUser {
		_, currentUID, ok, err := bootstrap.HashAndUID(a.secrets)
		if err != nil || !ok {
			return nil, ok, err
		}
		if currentUID != uid {
			return nil, false, nil
		}
	}

	return &user.DefaultInfo{
		Name: name,
		UID:  uid,
	}, true, nil
}

func (a *Authenticator) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	return false, a.put(w, user.GetName(), user.GetUID(), time.Now().Add(a.getMaxAge(user)).Unix())
}

// TODO figure out how to extract the BootstrapUser logic from this function
// as that is the only thing that prevents us from layering the BootstrapUser
// bits as a separate unit on top of the standard session logic
func (a *Authenticator) getMaxAge(user user.Info) time.Duration {
	// since osin is the IDP for this user, we increase the length
	// of the session to allow for transitions between components
	if user.GetName() == bootstrap.BootstrapUser {
		// this means the user could stay authenticated for one hour + OAuth access token lifetime
		return time.Hour
	}

	return a.maxAge
}

func (a *Authenticator) InvalidateAuthentication(w http.ResponseWriter, user user.Info) error {
	// the IDP is responsible for maintaining the user's session
	// since osin is the IDP for this user, we do not invalidate its session
	// this is safe to do because we tie the cookie to the password hash
	if user.GetName() == bootstrap.BootstrapUser {
		return nil
	}

	// zero out all fields
	return a.put(w, "", "", 0)
}

func (a *Authenticator) put(w http.ResponseWriter, name, uid string, expires int64) error {
	values := Values{}

	values[userNameKey] = name
	values[userUIDKey] = uid

	values[expKey] = expires

	return a.store.Put(w, values)
}
