package install

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apimachinery"
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
)

const importPrefix = "github.com/openshift/origin/pkg/security/apis/security"

var accessor = meta.NewAccessor()

// availableVersions lists all known external versions for this group from most preferred to least preferred
var availableVersions = []schema.GroupVersion{securityapiv1.LegacySchemeGroupVersion}

func init() {
	kapi.Registry.RegisterVersions(availableVersions)
	externalVersions := []schema.GroupVersion{}
	for _, v := range availableVersions {
		if kapi.Registry.IsAllowedVersion(v) {
			externalVersions = append(externalVersions, v)
		}
	}
	if len(externalVersions) == 0 {
		glog.Infof("No version is registered for group %v", securityapi.LegacyGroupName)
		return
	}

	if err := kapi.Registry.EnableVersions(externalVersions...); err != nil {
		panic(err)
	}
	if err := enableVersions(kapi.Registry, kapi.Scheme, externalVersions); err != nil {
		panic(err)
	}

	installApiGroup()
}

func InstallIntoDeprecatedV1(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	enableVersions(registry, scheme, []schema.GroupVersion{securityapiv1.LegacySchemeGroupVersion})
}

// TODO: enableVersions should be centralized rather than spread in each API
// group.
// We can combine kapi.Registry.RegisterVersions, kapi.Registry.EnableVersions and
// kapi.Registry.RegisterGroup once we have moved enableVersions there.
func enableVersions(registry *registered.APIRegistrationManager, scheme *runtime.Scheme, externalVersions []schema.GroupVersion) error {
	addVersionsToScheme(registry, scheme, externalVersions...)
	preferredExternalVersion := externalVersions[0]

	groupMeta := apimachinery.GroupMeta{
		GroupVersion:  preferredExternalVersion,
		GroupVersions: externalVersions,
		RESTMapper:    newRESTMapper(registry, scheme, externalVersions),
		SelfLinker:    runtime.SelfLinker(accessor),
		InterfacesFor: interfacesFor(registry, scheme),
	}

	if err := registry.RegisterGroup(groupMeta); err != nil {
		return err
	}
	return nil
}

func addVersionsToScheme(registry *registered.APIRegistrationManager, scheme *runtime.Scheme, externalVersions ...schema.GroupVersion) {
	// add the internal version to Scheme
	securityapi.AddToSchemeInCoreGroup(scheme)
	// add the enabled external versions to Scheme
	for _, v := range externalVersions {
		if !registry.IsEnabledVersion(v) {
			glog.Errorf("Version %s is not enabled, so it will not be added to the Scheme.", v)
			continue
		}
		switch v {
		case securityapiv1.LegacySchemeGroupVersion:
			securityapiv1.AddToSchemeInCoreGroup(scheme)
		default:
			glog.Errorf("Version %s is not known, so it will not be added to the Scheme.", v)
			continue
		}
	}
}

func newRESTMapper(registry *registered.APIRegistrationManager, scheme *runtime.Scheme, externalVersions []schema.GroupVersion) meta.RESTMapper {
	rootScoped := sets.NewString("SecurityContextConstraints")
	ignoredKinds := sets.NewString()
	return meta.NewDefaultRESTMapperFromScheme(externalVersions, interfacesFor(registry, scheme), importPrefix, ignoredKinds, rootScoped, scheme)
}

func interfacesFor(registry *registered.APIRegistrationManager, scheme *runtime.Scheme) func(version schema.GroupVersion) (*meta.VersionInterfaces, error) {
	return func(version schema.GroupVersion) (*meta.VersionInterfaces, error) {
		switch version {
		case securityapiv1.LegacySchemeGroupVersion:
			return &meta.VersionInterfaces{
				ObjectConvertor:  scheme,
				MetadataAccessor: accessor,
			}, nil

		default:
			g, _ := registry.Group(securityapi.LegacyGroupName)
			return nil, fmt.Errorf("unsupported storage version: %s (valid: %v)", version, g.GroupVersions)
		}
	}
}
