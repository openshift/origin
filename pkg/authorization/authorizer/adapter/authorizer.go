package adapter

import (
	"errors"

	"github.com/golang/glog"

	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"

	oauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
)

type AdapterAuthorizer struct {
	originAuthorizer oauthorizer.Authorizer
}

// NewAuthorizer adapts an Origin Authorizer interface to a Kubernetes Authorizer interface
func NewAuthorizer(originAuthorizer oauthorizer.Authorizer) (kauthorizer.Authorizer, error) {
	return &AdapterAuthorizer{originAuthorizer}, nil
}

func (z *AdapterAuthorizer) Authorize(kattrs kauthorizer.Attributes) error {
	allowed, reason, err := z.originAuthorizer.Authorize(OriginAuthorizerAttributes(kattrs))

	if err != nil {
		glog.V(5).Infof("evaluation error: %v", err)
		return err
	}

	glog.V(5).Infof("allowed=%v, reason=%s", allowed, reason)
	if allowed {
		return nil
	}

	// Turn the reason into an error so we can reject with the most information possible
	return errors.New(reason)
}
