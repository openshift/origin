package session

import (
	"net/http"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
)

func putUser(store Store, w http.ResponseWriter, user user.Info, expiresIn time.Duration) error {
	values := Values{}

	values[userNameKey] = user.GetName()
	values[userUIDKey] = user.GetUID()

	var expires int64
	if expiresIn > 0 {
		expires = time.Now().Add(expiresIn).Unix()
	}
	values[expKey] = expires

	return store.Put(w, values)
}
