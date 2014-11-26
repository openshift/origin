package meta

import kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"

// MultiRESTMapper is a wrapper for multiple RESTMappers.
type MultiRESTMapper []kmeta.RESTMapper

const (
	OriginAPI     = "origin"
	KubernetesAPI = "kubernetes"
)

// TODO: This list have to be maintained manually if a new API resource is added
// into the Origin API.
var OriginTypes = []string{
	"Build", "BuildConfig",
	"Deployment", "DeploymentConfig",
	"Image", "ImageRepository", "ImageRepositoryMapping",
	"Route",
	"Project",
	"User",
	"OAuth",
}

// VersionAndKindForResource provides the Version and Kind  mappings for the
// REST resources. This implementation supports multiple REST schemas and return
// the first match.
func (m MultiRESTMapper) VersionAndKindForResource(resource string) (defaultVersion, kind string, err error) {
	for _, t := range m {
		defaultVersion, kind, err = t.VersionAndKindForResource(resource)
		if err == nil {
			return
		}
	}
	return
}

// RESTMapping provides the REST mapping for the resource based on the resource
// kind and version. This implementation supports multiple REST schemas and
// return the first match.
func (m MultiRESTMapper) RESTMapping(version, kind string) (mapping *kmeta.RESTMapping, err error) {
	for _, t := range m {
		mapping, err = t.RESTMapping(version, kind)
		if err == nil {
			return
		}
	}
	return
}

// APINameForResource provides information about the API name that manages the
// given resource and version.
func (m MultiRESTMapper) APINameForResource(version, kind string) string {
	for _, t := range OriginTypes {
		if t == kind {
			return OriginAPI
		}
	}
	return KubernetesAPI
}
