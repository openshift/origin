package admission

import (
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/project/cache"
)

// WantsOpenshiftClient should be implemented by admission plugins that need
// an Openshift client
type WantsOpenshiftClient interface {
	SetOpenshiftClient(client.Interface)
}

// WantsProjectCache should be implemented by admission plugins that need a
// project cache
type WantsProjectCache interface {
	SetProjectCache(*cache.ProjectCache)
}

// Validator should be implemented by admission plugins that can validate themselves
// after initialization has happened.
type Validator interface {
	Validate() error
}

// WantsAuthorizer should be implemented by admission plugins that
// need access to the Authorizer interface
type WantsAuthorizer interface {
	SetAuthorizer(authorizer.Authorizer)
}
