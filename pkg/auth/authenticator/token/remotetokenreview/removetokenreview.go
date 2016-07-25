package remotetokenreview

import (
	"errors"

	"k8s.io/kubernetes/pkg/apis/authentication"
	"k8s.io/kubernetes/pkg/auth/user"
	unversionedauthentication "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authentication/unversioned"
)

type Authenticator struct {
	authenticationClient unversionedauthentication.TokenReviewsGetter
}

// NewAuthenticator authenticates by doing a tokenreview
func NewAuthenticator(authenticationClient unversionedauthentication.TokenReviewsGetter) (*Authenticator, error) {
	return &Authenticator{
		authenticationClient: authenticationClient,
	}, nil
}

func (a *Authenticator) AuthenticateToken(value string) (user.Info, bool, error) {
	if len(value) == 0 {
		return nil, false, nil
	}
	tokenReview := &authentication.TokenReview{}
	tokenReview.Spec.Token = value

	response, err := a.authenticationClient.TokenReviews().Create(tokenReview)
	if err != nil {
		return nil, false, err
	}

	if len(response.Status.Error) > 0 {
		return nil, false, errors.New(response.Status.Error)
	}
	if !response.Status.Authenticated {
		return nil, false, nil
	}

	userInfo := &user.DefaultInfo{
		Name:   response.Status.User.Username,
		UID:    response.Status.User.UID,
		Groups: response.Status.User.Groups,
		Extra:  map[string][]string{},
	}
	for k, v := range response.Status.User.Extra {
		userInfo.Extra[k] = v
	}

	return userInfo, true, nil
}
