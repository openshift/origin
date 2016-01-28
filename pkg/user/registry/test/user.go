package test

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/user/api"
)

type UserRegistry struct {
	GetErr map[string]error
	Get    map[string]*api.User

	CreateErr error
	Create    *api.User

	UpdateErr map[string]error
	Update    *api.User

	ListErr error
	List    *api.UserList

	Actions *[]Action
}

func NewUserRegistry() *UserRegistry {
	return &UserRegistry{
		GetErr:    map[string]error{},
		Get:       map[string]*api.User{},
		UpdateErr: map[string]error{},
		Actions:   &[]Action{},
	}
}

func (r *UserRegistry) GetUser(ctx kapi.Context, name string) (*api.User, error) {
	*r.Actions = append(*r.Actions, Action{"GetUser", name})
	if user, ok := r.Get[name]; ok {
		return user, nil
	}
	if err, ok := r.GetErr[name]; ok {
		return nil, err
	}
	return nil, kerrs.NewNotFound(api.Resource("user"), name)
}

func (r *UserRegistry) CreateUser(ctx kapi.Context, u *api.User) (*api.User, error) {
	*r.Actions = append(*r.Actions, Action{"CreateUser", u})
	if r.Create == nil && r.CreateErr == nil {
		return u, nil
	}
	return r.Create, r.CreateErr
}

func (r *UserRegistry) UpdateUser(ctx kapi.Context, u *api.User) (*api.User, error) {
	*r.Actions = append(*r.Actions, Action{"UpdateUser", u})
	err, _ := r.UpdateErr[u.Name]
	if r.Update == nil && err == nil {
		return u, nil
	}
	return r.Update, err
}

func (r *UserRegistry) ListUsers(ctx kapi.Context, options *kapi.ListOptions) (*api.UserList, error) {
	*r.Actions = append(*r.Actions, Action{"ListUsers", options})
	if r.List == nil && r.ListErr == nil {
		return &api.UserList{}, nil
	}
	return r.List, r.ListErr
}
