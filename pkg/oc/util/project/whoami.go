package project

import (
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
)

func WhoAmI(clientConfig *restclient.Config) (*userapi.User, error) {
	client, err := userclient.NewForConfig(clientConfig)

	me, err := client.User().Users().Get("~", metav1.GetOptions{})

	// if we're talking to kube (or likely talking to kube),
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		switch {
		case len(clientConfig.BearerToken) > 0:
			// the user has already been willing to provide the token on the CLI, so they probably
			// don't mind using it again if they switch to and from this user
			return &userapi.User{ObjectMeta: metav1.ObjectMeta{Name: clientConfig.BearerToken}}, nil

		case len(clientConfig.Username) > 0:
			return &userapi.User{ObjectMeta: metav1.ObjectMeta{Name: clientConfig.Username}}, nil

		}
	}

	if err != nil {
		return nil, err
	}

	return me, nil
}
