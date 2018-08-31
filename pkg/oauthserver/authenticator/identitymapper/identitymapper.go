package identitymapper

import (
	"fmt"

	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oauthserver/api"
)

// UserFor bridges the UserIdentityMapper interface with the authenticator.{Password|Request} interfaces
func UserFor(mapper api.UserIdentityMapper, identity api.UserIdentityInfo) (user.Info, bool, error) {
	user, err := mapper.UserFor(identity)
	if err != nil {
		logf("error creating or updating mapping for: %#v due to %v", identity, err)
		return nil, false, err
	}
	logf("got userIdentityMapping: %#v", user)

	return user, true, nil
}

// logf(...) is the same as glog.V(4).Infof(...) except it reports the caller as the line number
func logf(format string, args ...interface{}) {
	if glog.V(4) {
		glog.InfoDepth(2, fmt.Sprintf("identitymapper: "+format, args...))
	}
}
