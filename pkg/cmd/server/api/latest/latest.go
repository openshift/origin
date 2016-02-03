package latest

import (
	// kapi "k8s.io/kubernetes/pkg/api"
	// "k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime/serializer"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// HACK TO ELIMINATE CYCLE UNTIL WE KILL THIS PACKAGE

// Version is the string that represents the current external default version.
var Version = unversioned.GroupVersion{Group: "", Version: "v1"}

// OldestVersion is the string that represents the oldest server version supported,
// for client code that wants to hardcode the lowest common denominator.
var OldestVersion = unversioned.GroupVersion{Group: "", Version: "v1"}

// Versions is the list of versions that are recognized in code. The order provided
// may be assumed to be least feature rich to most feature rich, and clients may
// choose to prefer the latter items in the list over the former items when presented
// with a set of versions to choose.
var Versions = []unversioned.GroupVersion{unversioned.GroupVersion{Group: "", Version: "v1"}}

var Codec = serializer.NewCodecFactory(configapi.Scheme).LegacyCodec(unversioned.GroupVersion{Group: "", Version: "v1"})

// func interfacesFor(version unversioned.GroupVersion) (*meta.VersionInterfaces, error) {
// 	switch version {
// 	case unversioned.GroupVersion{Group: "", Version: "v1"}:
// 		return &meta.VersionInterfaces{
// 			ObjectConvertor:  kapi.Scheme,
// 			MetadataAccessor: accessor,
// 		}, nil

// 	default:
// 		return nil, fmt.Errorf("unsupported storage version: %s", version)
// 	}
// }

// func NewRESTMapper(externalVersions []unversioned.GroupVersion) meta.RESTMapper {
// 	rootScoped := sets.NewString()
// 	ignoredKinds := sets.NewString()

// 	mapper := meta.NewDefaultRESTMapper(defaultGroupVersions, interfacesFunc)
// 	// enumerate all supported versions, get the kinds, and register with the mapper how to address
// 	// our resources.
// 	for _, gv := range defaultGroupVersions {
// 		for kind, oType := range Scheme.KnownTypes(gv) {
// 			gvk := gv.WithKind(kind)
// 			// TODO: Remove import path prefix check.
// 			// We check the import path prefix because we currently stuff both "api" and "extensions" objects
// 			// into the same group within Scheme since Scheme has no notion of groups yet.
// 			if !strings.HasPrefix(oType.PkgPath(), importPathPrefix) || ignoredKinds.Has(kind) {
// 				continue
// 			}
// 			scope := meta.RESTScopeNamespace
// 			if rootScoped.Has(kind) {
// 				scope = meta.RESTScopeRoot
// 			}
// 			mapper.Add(gvk, scope, false)
// 		}
// 	}

// 	return kapi.NewDefaultRESTMapper(Versions, interfacesFor, importPrefix, ignoredKinds, rootScoped)
// }
