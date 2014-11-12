package latest

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	_ "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/v1beta1"
)

// Version is the string that represents the current external default version.
const Version = "v1beta1"

// OldestVersion is the string that represents the oldest server version supported,
// for client code that wants to hardcode the lowest common denominator.
const OldestVersion = "v1beta1"

// Versions is the list of versions that are recognized in code. The order provided
// may be assumed to be least feature rich to most feature rich, and clients may
// choose to prefer the latter items in the list over the former items when presented
// with a set of versions to choose.
var Versions = []string{"v1beta1"}

// Codec is the default codec for serializing output that should use
// the latest supported version.  Use this Codec when writing to
// disk, a data store that is not dynamically versioned, or in tests.
// This codec can decode any object that OpenShift is aware of.
var Codec = v1beta1.Codec

// accessor is the shared static metadata accessor for the API.
var accessor = meta.NewAccessor()

// ResourceVersioner describes a default versioner that can handle all types
// of versioning.
// TODO: when versioning changes, make this part of each API definition.
var ResourceVersioner runtime.ResourceVersioner = accessor

// SelfLinker can set or get the SelfLink field of all API types.
// TODO: when versioning changes, make this part of each API definition.
// TODO(lavalamp): Combine SelfLinker & ResourceVersioner interfaces, force all uses
// to go through the InterfacesFor method below.
var SelfLinker runtime.SelfLinker = accessor

// RESTMapper provides the default mapping between REST paths and the objects declared in api.Scheme and all known
// Kubernetes versions.
var RESTMapper meta.RESTMapper

// InterfacesFor returns the default Codec and ResourceVersioner for a given version
// string, or an error if the version is not known.
func InterfacesFor(version string) (*meta.VersionInterfaces, error) {
	switch version {
	case "v1beta1":
		return &meta.VersionInterfaces{
			Codec:            v1beta1.Codec,
			ObjectConvertor:  api.Scheme,
			MetadataAccessor: accessor,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage version: %s (valid: %s)", version, strings.Join(Versions, ", "))
	}
}

func init() {
	mapper := meta.NewDefaultRESTMapper(
		Versions,
		func(version string) (*meta.VersionInterfaces, bool) {
			interfaces, err := InterfacesFor(version)
			if err != nil {
				return nil, false
			}
			return interfaces, true
		},
	)
	mapper.Add(api.Scheme, true, Versions...)
	// TODO: when v1beta3 is added it will not use mixed case.
	RESTMapper = mapper
}
