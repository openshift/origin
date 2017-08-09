package restrictusers

import (
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

type fakeGroupCache struct {
	groups []userapi.Group
}

func (g fakeGroupCache) GroupsFor(user string) ([]*userapi.Group, error) {
	ret := []*userapi.Group{}
	for i := range g.groups {
		group := &g.groups[i]
		for _, currUser := range group.Users {
			if user == currUser {
				ret = append(ret, group)
				break
			}
		}

	}
	return ret, nil
}
