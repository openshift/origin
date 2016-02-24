package admission

import (
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/project/cache"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// WantsOpenshiftClient should be implemented by admission plugins that need
// an Openshift client
type WantsOpenshiftClient interface {
	SetOpenshiftClient(client.Interface)
}

// WantsInternalRegistryClientFactory should be implemented by admission plugins
// that need to query internal registry
type WantsInternalRegistryClientFactory interface {
	SetInternalRegistryClientFactory(quotautil.InternalRegistryClientFactory)
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
