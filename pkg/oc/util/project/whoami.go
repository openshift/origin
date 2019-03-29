package project

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	userv1 "github.com/openshift/api/user/v1"
	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
)

func WhoAmI(clientConfig *restclient.Config) (*userv1.User, error) {
	client, err := userv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	me, err := client.Users().Get("~", metav1.GetOptions{})

	// if we're talking to kube (or likely talking to kube),
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		switch {
		case len(clientConfig.BearerToken) > 0:
			// the user has already been willing to provide the token on the CLI, so they probably
			// don't mind using it again if they switch to and from this user
			return &userv1.User{ObjectMeta: metav1.ObjectMeta{Name: clientConfig.BearerToken}}, nil

		case len(clientConfig.Username) > 0:
			return &userv1.User{ObjectMeta: metav1.ObjectMeta{Name: clientConfig.Username}}, nil

		}
	}

	if err != nil {
		return nil, err
	}

	return me, nil
}
