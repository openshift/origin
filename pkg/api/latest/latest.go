package latest

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/golang/glog"

	_ "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/api/v1beta3"
)

// Version is the string that represents the current external default version.
const Version = "v1beta3"

// OldestVersion is the string that represents the oldest server version supported,
// for client code that wants to hardcode the lowest common denominator.
const OldestVersion = "v1beta1"

// Versions is the list of versions that are recognized in code. The order provided
// may be assumed to be least feature rich to most feature rich, and clients may
// choose to prefer the latter items in the list over the former items when presented
// with a set of versions to choose.
var Versions = []string{"v1beta1", "v1beta3", "v1"}

// Codec is the default codec for serializing output that should use
// the latest supported version.  Use this Codec when writing to
// disk, a data store that is not dynamically versioned, or in tests.
// This codec can decode any object that OpenShift is aware of.
var Codec = v1beta3.Codec

// accessor is the shared static metadata accessor for the API.
var accessor = kmeta.NewAccessor()

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
var RESTMapper kmeta.RESTMapper

// InterfacesFor returns the default Codec and ResourceVersioner for a given version
// string, or an error if the version is not known.
func InterfacesFor(version string) (*kmeta.VersionInterfaces, error) {
	switch version {
	case "v1beta1":
		return &kmeta.VersionInterfaces{
			Codec:            v1beta1.Codec,
			ObjectConvertor:  api.Scheme,
			MetadataAccessor: accessor,
		}, nil
	case "v1beta3":
		return &kmeta.VersionInterfaces{
			Codec:            v1beta3.Codec,
			ObjectConvertor:  api.Scheme,
			MetadataAccessor: accessor,
		}, nil
	case "v1":
		return &kmeta.VersionInterfaces{
			Codec:            v1.Codec,
			ObjectConvertor:  api.Scheme,
			MetadataAccessor: accessor,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported storage version: %s (valid: %s)", version, strings.Join(Versions, ", "))
	}
}

// originTypes are the hardcoded types defined by the OpenShift API.
var originTypes = util.StringSet{}

// UserResources are the resource names that apply to the primary, user facing resources used by
// client tools. They are in deletion-first order - dependent resources should be last.
var UserResources = []string{
	"buildConfigs", "builds",
	"imageStreams",
	"deploymentConfigs", "replicationControllers",
	"routes", "services",
	"pods",
}

// OriginKind returns true if OpenShift owns the kind described in a given apiVersion.
func OriginKind(kind, apiVersion string) bool {
	return originTypes.Has(kind)
}

func init() {
	kubeMapper := klatest.RESTMapper

	// list of versions we support on the server, in preferred order
	versions := []string{"v1beta3", "v1beta1", "v1"}

	originMapper := kmeta.NewDefaultRESTMapper(
		versions,
		func(version string) (*kmeta.VersionInterfaces, bool) {
			interfaces, err := InterfacesFor(version)
			if err != nil {
				return nil, false
			}
			return interfaces, true
		},
	)

	// versions that used mixed case URL formats
	versionMixedCase := map[string]bool{
		"v1beta1": true,
	}

	// backwards compatibility, prior to v1beta3, we identified the namespace as a query parameter
	versionToNamespaceScope := map[string]kmeta.RESTScope{
		"v1beta1": kmeta.RESTScopeNamespaceLegacy,
		"v1beta3": kmeta.RESTScopeNamespace,
		"v1":      kmeta.RESTScopeNamespace,
	}

	// the list of kinds that are scoped at the root of the api hierarchy
	// if a kind is not enumerated here, it is assumed to have a namespace scope
	kindToRootScope := map[string]bool{
		"Status": true,

		"Project":        true,
		"ProjectRequest": true,

		"Image": true,

		"User":                true,
		"Identity":            true,
		"UserIdentityMapping": true,

		"OAuthAccessToken":         true,
		"OAuthAuthorizeToken":      true,
		"OAuthClient":              true,
		"OAuthClientAuthorization": true,

		"ClusterRole":          true,
		"ClusterRoleBinding":   true,
		"ClusterPolicy":        true,
		"ClusterPolicyBinding": true,

		"ClusterNetwork": true,
		"HostSubnet":     true,
	}

	// enumerate all supported versions, get the kinds, and register with the mapper how to address our resources
	for _, version := range versions {
		for kind, t := range api.Scheme.KnownTypes(version) {
			if !strings.Contains(t.PkgPath(), "openshift/origin") {
				if _, ok := kindToRootScope[kind]; !ok {
					continue
				}
			}
			originTypes.Insert(kind)
			mixedCase, found := versionMixedCase[version]
			if !found {
				mixedCase = false
			}
			scope, found := versionToNamespaceScope[version]
			if !found {
				panic(fmt.Sprintf("no scope defined for %s", version))
			}
			_, found = kindToRootScope[kind]
			if found || (strings.HasSuffix(kind, "List") && kindToRootScope[strings.TrimSuffix(kind, "List")]) {
				scope = kmeta.RESTScopeRoot
			}
			glog.V(6).Infof("Registering %s %s %s", kind, version, scope.Name())
			originMapper.Add(scope, kind, version, mixedCase)
		}
	}

	// For Origin we use MultiRESTMapper that handles both Origin and Kubernetes
	// objects
	RESTMapper = meta.MultiRESTMapper{originMapper, kubeMapper}
}
