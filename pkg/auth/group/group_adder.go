package group

import (
	"net/http"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

// GroupAdder wraps a request authenticator, and adds the specified groups to the returned user when authentication succeeds
type GroupAdder struct {
	Authenticator authenticator.Request
	Groups        []string
}

func (g *GroupAdder) AuthenticateRequest(req *http.Request) (api.UserInfo, bool, error) {
	user, ok, err := g.Authenticator.AuthenticateRequest(req)
	if err != nil || !ok {
		return nil, ok, err
	}
	return &api.DefaultUserInfo{
		Name:   user.GetName(),
		UID:    user.GetUID(),
		Groups: append(user.GetGroups(), g.Groups...),
		Scope:  user.GetScope(),
		Extra:  user.GetExtra(),
	}, true, nil
}

func NewGroupAdder(auth authenticator.Request, groups []string) *GroupAdder {
	return &GroupAdder{auth, groups}
}
