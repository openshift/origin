package meta

import "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"

// MultiRESTMapper is a wrapper for multiple RESTMappers.
type MultiRESTMapper []meta.RESTMapper

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
func (m MultiRESTMapper) RESTMapping(version, kind string) (mapping *meta.RESTMapping, err error) {
	for _, t := range m {
		mapping, err = t.RESTMapping(version, kind)
		if err == nil {
			return
		}
	}
	return
}
