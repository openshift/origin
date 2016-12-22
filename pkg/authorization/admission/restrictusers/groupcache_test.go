package restrictusers

import (
	kapi "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"

	userapi "github.com/openshift/origin/pkg/user/api"
)

type groupCache struct {
	groups []userapi.Group
}

// GroupCache uses only ListGroups and WatchGroups.  Other methods can be stubs.
func (groupCache *groupCache) ListGroups(ctx kapi.Context, options *kapi.ListOptions) (*userapi.GroupList, error) {
	return &userapi.GroupList{Items: groupCache.groups}, nil
}
func (groupCache *groupCache) GetGroup(ctx kapi.Context, name string) (*userapi.Group, error) {
	return nil, nil
}
func (*groupCache) CreateGroup(ctx kapi.Context, group *userapi.Group) (*userapi.Group, error) {
	return nil, nil
}
func (*groupCache) UpdateGroup(ctx kapi.Context, group *userapi.Group) (*userapi.Group, error) {
	return nil, nil
}
func (*groupCache) DeleteGroup(ctx kapi.Context, name string) error {
	return nil
}
func (*groupCache) WatchGroups(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return watch.NewFake(), nil
}
