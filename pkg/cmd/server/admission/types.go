package admission

import (
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
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

// WantsQuotaRegistry should be implemented by admission plugins that need a quota registry
type WantsOriginQuotaRegistry interface {
	SetOriginQuotaRegistry(quota.Registry)
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

// WantsJenkinsPipelineConfig gives access to the JenkinsPipelineConfig.  This is a historical oddity.
// It's likely that what we really wanted was this as an admission plugin config
type WantsJenkinsPipelineConfig interface {
	SetJenkinsPipelineConfig(jenkinsConfig configapi.JenkinsPipelineConfig)
}

// WantsRESTClientConfig gives access to a RESTClientConfig.  It's useful for doing unusual things with transports.
type WantsRESTClientConfig interface {
	SetRESTClientConfig(restclient.Config)
}
