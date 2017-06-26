package restrictusers

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

type groupCache struct {
	groups []userapi.Group
}

// GroupCache uses only ListGroups and WatchGroups.  Other methods can be stubs.
func (groupCache *groupCache) ListGroups(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.GroupList, error) {
	return &userapi.GroupList{Items: groupCache.groups}, nil
}
func (groupCache *groupCache) GetGroup(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.Group, error) {
	return nil, nil
}
func (*groupCache) CreateGroup(ctx apirequest.Context, group *userapi.Group) (*userapi.Group, error) {
	return nil, nil
}
func (*groupCache) UpdateGroup(ctx apirequest.Context, group *userapi.Group) (*userapi.Group, error) {
	return nil, nil
}
func (*groupCache) DeleteGroup(ctx apirequest.Context, name string) error {
	return nil
}
func (*groupCache) WatchGroups(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return watch.NewFake(), nil
}
