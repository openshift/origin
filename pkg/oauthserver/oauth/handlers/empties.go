package handlers

import (
	"net/http"

	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"k8s.io/apiserver/pkg/authentication/user"
)

type EmptyAuth struct{}

func (EmptyAuth) AuthenticationNeeded(client authapi.Client, w http.ResponseWriter, req *http.Request) (bool, error) {
	return false, nil
}

type EmptySuccess struct{}

func (EmptySuccess) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.V(4).Infof("AuthenticationSucceeded: %v (state=%s)", user, state)
	return false, nil
}

type EmptyError struct{}

func (EmptyError) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.Errorf("AuthenticationError: %v", err)
	return false, err
}

func (EmptyError) GrantError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.Errorf("GrantError: %v", err)
	return false, err
}
