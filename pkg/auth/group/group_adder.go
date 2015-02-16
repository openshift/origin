package group

import (
	"net/http"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

// GroupAdder wraps a request authenticator, and adds the specified groups to the returned user when authentication succeeds
type GroupAdder struct {
	Authenticator authenticator.Request
	Groups        []string
}

func (g *GroupAdder) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	u, ok, err := g.Authenticator.AuthenticateRequest(req)
	if err != nil || !ok {
		return nil, ok, err
	}
	return &user.DefaultInfo{
		Name:   u.GetName(),
		UID:    u.GetUID(),
		Groups: append(u.GetGroups(), g.Groups...),
	}, true, nil
}

func NewGroupAdder(auth authenticator.Request, groups []string) *GroupAdder {
	return &GroupAdder{auth, groups}
}
