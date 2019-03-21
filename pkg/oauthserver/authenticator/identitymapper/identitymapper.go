package identitymapper

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/openshift/origin/pkg/oauthserver/api"
)

// ResponseFor bridges the UserIdentityMapper interface with the authenticator.{Password|Request} interfaces
func ResponseFor(mapper api.UserIdentityMapper, identity api.UserIdentityInfo) (*authenticator.Response, bool, error) {
	user, err := mapper.UserFor(identity)
	if err != nil {
		logf("error creating or updating mapping for: %#v due to %v", identity, err)
		return nil, false, err
	}
	logf("got userIdentityMapping: %#v", user)

	// only set User field as IDPs have no concept of Audiences
	return &authenticator.Response{User: user}, true, nil
}

// logf(...) is the same as glog.V(4).Infof(...) except it reports the caller as the line number
func logf(format string, args ...interface{}) {
	if glog.V(4) {
		glog.InfoDepth(2, fmt.Sprintf("identitymapper: "+format, args...))
	}
}
