package session

import (
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"

	userv1 "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	authapi "github.com/openshift/origin/pkg/oauthserver/api"
)

const (
	userNameKey = "user.name"
	userUIDKey  = "user.uid"

	// expKey is stored as an int64 unix time
	expKey = "exp"

	// identityMetadataNameKey is used to optionally store the name of an IdentityMetadata object
	// which contains group information that may be too large to store in a cookie directly
	identityMetadataNameKey = "metadata"
)

type Authenticator struct {
	store            Store
	maxAge           time.Duration
	expiresIn        int64
	identityMetadata userclient.IdentityMetadataInterface
}

func NewAuthenticator(store Store, maxAge time.Duration, identityMetadata userclient.IdentityMetadataInterface) *Authenticator {
	return &Authenticator{
		store:            store,
		maxAge:           maxAge,
		expiresIn:        int64(maxAge / time.Second),
		identityMetadata: identityMetadata,
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

	u := &user.DefaultInfo{
		Name: name,
		UID:  uid,
	}

	// check if we reference an identity metadata object
	identityMetadataName, ok := values.GetString(identityMetadataNameKey)

	// just use the name and uid when we do not reference an identity metadata object
	if !ok {
		return u, true, nil
	}

	// fetch the referenced identity metadata object
	metadata, err := a.identityMetadata.Get(identityMetadataName, metav1.GetOptions{})
	if err != nil {
		// if the object does not exist, it probably expired and was deleted, no need to fail
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// use the data from the identity metadata object that we reference
	return authapi.NewDefaultUserIdentityMetadata(u, metadata.ProviderName, metadata.ProviderGroups), true, nil
}

func (a *Authenticator) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	name := user.GetName()
	uid := user.GetUID()

	// assume we do not need to create an identity metadata object by default
	identityMetadata := ""
	// check if we need to store group information
	if userIdentityMetadata, ok := user.(authapi.UserIdentityMetadata); ok {
		idpName := userIdentityMetadata.GetIdentityProviderName()
		metadata, err := a.identityMetadata.Create(&userv1.IdentityMetadata{
			ObjectMeta: metav1.ObjectMeta{
				// let the server generate something unique.
				// there is no specific format for this object's name, but it
				// does not hurt to include all of the information available to us.
				// the object itself is transient so it likely does not matter.
				GenerateName: idpName + ":" + name + ":" + uid + ":",
			},
			ProviderName:   idpName,
			ProviderGroups: userIdentityMetadata.GetIdentityProviderGroups(),
			ExpiresIn:      a.expiresIn,
		})
		if err != nil {
			return false, err
		}
		identityMetadata = metadata.Name
	}

	return false, a.put(w, name, uid, identityMetadata, time.Now().Add(a.maxAge).Unix())
}

func (a *Authenticator) InvalidateAuthentication(w http.ResponseWriter, req *http.Request) error {
	// zero out all fields
	return a.put(w, "", "", "", 0)
}

func (a *Authenticator) put(w http.ResponseWriter, name, uid, identityMetadata string, expires int64) error {
	values := Values{}

	values[userNameKey] = name
	values[userUIDKey] = uid

	values[expKey] = expires

	values[identityMetadataNameKey] = identityMetadata

	return a.store.Put(w, values)
}
