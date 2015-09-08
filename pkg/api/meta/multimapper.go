package meta

import (
	kmeta "k8s.io/kubernetes/pkg/api/meta"
)

// MultiRESTMapper is a wrapper for multiple RESTMappers.
type MultiRESTMapper []kmeta.RESTMapper

// VersionAndKindForResource provides the Version and Kind mappings for the
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
func (m MultiRESTMapper) RESTMapping(kind string, versions ...string) (mapping *kmeta.RESTMapping, err error) {
	for _, t := range m {
		mapping, err = t.RESTMapping(kind, versions...)
		if err == nil {
			return
		}
	}
	return
}

// AliasesForResource returns whether a resource has an alias or not. This implementation
// supports multiple REST schemas and return the first match.
func (m MultiRESTMapper) AliasesForResource(resource string) (aliases []string, has bool) {
	for _, t := range m {
		aliases, has = t.AliasesForResource(resource)
		if has {
			return
		}
	}
	return
}
