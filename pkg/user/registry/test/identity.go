package test

import (
	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

type Action struct {
	Name   string
	Object interface{}
}

type IdentityRegistry struct {
	GetErr map[string]error
	Get    map[string]*userapi.Identity

	CreateErr error
	Create    *userapi.Identity

	UpdateErr error
	Update    *userapi.Identity

	ListErr error
	List    *userapi.IdentityList

	Actions *[]Action
}

func NewIdentityRegistry() *IdentityRegistry {
	return &IdentityRegistry{
		GetErr:  map[string]error{},
		Get:     map[string]*userapi.Identity{},
		Actions: &[]Action{},
	}
}

func (r *IdentityRegistry) GetIdentity(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.Identity, error) {
	*r.Actions = append(*r.Actions, Action{"GetIdentity", name})
	if identity, ok := r.Get[name]; ok {
		return identity, nil
	}
	if err, ok := r.GetErr[name]; ok {
		return nil, err
	}
	return nil, kerrs.NewNotFound(userapi.Resource("identity"), name)
}

func (r *IdentityRegistry) CreateIdentity(ctx apirequest.Context, u *userapi.Identity) (*userapi.Identity, error) {
	*r.Actions = append(*r.Actions, Action{"CreateIdentity", u})
	if r.Create == nil && r.CreateErr == nil {
		return u, nil
	}
	return r.Create, r.CreateErr
}

func (r *IdentityRegistry) UpdateIdentity(ctx apirequest.Context, u *userapi.Identity) (*userapi.Identity, error) {
	*r.Actions = append(*r.Actions, Action{"UpdateIdentity", u})
	if r.Update == nil && r.UpdateErr == nil {
		return u, nil
	}
	return r.Update, r.UpdateErr
}

func (r *IdentityRegistry) ListIdentities(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.IdentityList, error) {
	*r.Actions = append(*r.Actions, Action{"ListIdentities", options})
	if r.List == nil && r.ListErr == nil {
		return &userapi.IdentityList{}, nil
	}
	return r.List, r.ListErr
}
