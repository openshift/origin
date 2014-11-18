package meta

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

// MultiRESTMapper is a wrapper for multiple RESTMappers.
type MultiRESTMapper struct {
	Mappers []meta.RESTMapper
}

// MultiObjectTyper is a wrapper for multiple ObjectTypers.
type MultiObjectTyper struct {
	Typers []runtime.ObjectTyper
}

// DataVersionAndKind provides the Version and Kind from a raw JSON/YAML string
// based on the current Schema. This implementation searches multiple schemas
// and return the first match.
func (m MultiObjectTyper) DataVersionAndKind(data []byte) (version, kind string, err error) {
	for _, t := range m.Typers {
		version, kind, err = t.DataVersionAndKind(data)
		if err != nil && len(kind) > 0 {
			return
		}
	}
	return
}

// ObjectVersionAndKind provides the Version and Kind from a runtime Object
// based on the current Schema. This implementation searches multiple schemas
// and return the first match.
func (m MultiObjectTyper) ObjectVersionAndKind(obj runtime.Object) (version, kind string, err error) {
	for _, t := range m.Typers {
		version, kind, err = t.ObjectVersionAndKind(obj)
		if err != nil && len(kind) > 0 {
			return
		}
	}
	return
}

// VersionAndKindForResource provides the Version and Kind  mappings for the
// REST resources. This implementation supports multiple REST schemas and return
// the first match.
func (m MultiRESTMapper) VersionAndKindForResource(resource string) (defaultVersion, kind string, err error) {
	for _, t := range m.Mappers {
		defaultVersion, kind, err = t.VersionAndKindForResource(resource)
		if err != nil && len(kind) > 0 {
			return
		}
	}
	return
}

// RESTMapping provides the REST mapping for the resource based on the resource
// kind and version. This implementation supports multiple REST schemas and
// return the first match.
func (m MultiRESTMapper) RESTMapping(version, kind string) (mapping *meta.RESTMapping, err error) {
	for _, t := range m.Mappers {
		mapping, err = t.RESTMapping(version, kind)
		if err != nil && mapping != nil {
			return
		}
	}
	return
}
