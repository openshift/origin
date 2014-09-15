package accesstoken

import (
	"fmt"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/oauth/api"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new AccessToken for use with Create and Update.
func (s *REST) New() interface{} {
	return &api.AccessToken{}
}

// Get retrieves an AccessToken by id.
func (s *REST) Get(id string) (interface{}, error) {
	token, err := s.registry.GetAccessToken(id)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// List retrieves a list of AccessTokens that match selector.
func (s *REST) List(selector labels.Selector) (interface{}, error) {
	tokens, err := s.registry.ListAccessTokens(selector)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

// Create registers the given AccessToken.
func (s *REST) Create(obj interface{}) (<-chan interface{}, error) {
	token, ok := obj.(*api.AccessToken)
	if !ok {
		return nil, fmt.Errorf("not an token: %#v", obj)
	}

	token.CreationTimestamp = util.Now()

	// if errs := validation.ValidateAccessToken(token); len(errs) > 0 {
	// 	return nil, errors.NewInvalid("token", token.Name, errs)
	// }

	return apiserver.MakeAsync(func() (interface{}, error) {
		if err := s.registry.CreateAccessToken(token); err != nil {
			return nil, err
		}
		return s.Get(token.Name)
	}), nil
}

// Update is not supported for AccessTokens, as they are immutable.
func (s *REST) Update(obj interface{}) (<-chan interface{}, error) {
	return nil, fmt.Errorf("AccessTokens may not be changed.")
}

// Delete asynchronously deletes an AccessToken specified by its id.
func (s *REST) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, s.registry.DeleteAccessToken(id)
	}), nil
}
