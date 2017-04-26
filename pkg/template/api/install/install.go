package install

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apimachinery"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/v1"
)

const importPrefix = "github.com/openshift/origin/pkg/template/api"

var accessor = meta.NewAccessor()

// availableVersions lists all known external versions for this group from most preferred to least preferred
var availableVersions = []schema.GroupVersion{v1.LegacySchemeGroupVersion}

func init() {
	kapi.Registry.RegisterVersions(availableVersions)
	externalVersions := []schema.GroupVersion{}
	for _, v := range availableVersions {
		if kapi.Registry.IsAllowedVersion(v) {
			externalVersions = append(externalVersions, v)
		}
	}
	if len(externalVersions) == 0 {
		glog.Infof("No version is registered for group %v", api.LegacyGroupName)
		return
	}

	if err := kapi.Registry.EnableVersions(externalVersions...); err != nil {
		panic(err)
	}
	if err := enableVersions(externalVersions); err != nil {
		panic(err)
	}

	installApiGroup()
}

// TODO: enableVersions should be centralized rather than spread in each API
// group.
// We can combine kapi.Registry.RegisterVersions, kapi.Registry.EnableVersions and
// kapi.Registry.RegisterGroup once we have moved enableVersions there.
func enableVersions(externalVersions []schema.GroupVersion) error {
	addVersionsToScheme(externalVersions...)
	preferredExternalVersion := externalVersions[0]

	groupMeta := apimachinery.GroupMeta{
		GroupVersion:  preferredExternalVersion,
		GroupVersions: externalVersions,
		RESTMapper:    newRESTMapper(externalVersions),
		SelfLinker:    runtime.SelfLinker(accessor),
		InterfacesFor: interfacesFor,
	}

	if err := kapi.Registry.RegisterGroup(groupMeta); err != nil {
		return err
	}
	return nil
}

func addVersionsToScheme(externalVersions ...schema.GroupVersion) {
	// add the internal version to Scheme
	api.AddToSchemeInCoreGroup(kapi.Scheme)
	// add the enabled external versions to Scheme
	for _, v := range externalVersions {
		if !kapi.Registry.IsEnabledVersion(v) {
			glog.Errorf("Version %s is not enabled, so it will not be added to the Scheme.", v)
			continue
		}
		switch v {
		case v1.LegacySchemeGroupVersion:
			v1.AddToSchemeInCoreGroup(kapi.Scheme)

		default:
			glog.Errorf("Version %s is not known, so it will not be added to the Scheme.", v)
			continue
		}
	}
}

func newRESTMapper(externalVersions []schema.GroupVersion) meta.RESTMapper {
	worstToBestGroupVersions := []schema.GroupVersion{}
	for i := len(externalVersions) - 1; i >= 0; i-- {
		worstToBestGroupVersions = append(worstToBestGroupVersions, externalVersions[i])
	}
	rootScoped := sets.NewString("BrokerTemplateInstance")
	ignoredKinds := sets.NewString()
	return meta.NewDefaultRESTMapperFromScheme(externalVersions, interfacesFor, importPrefix, ignoredKinds, rootScoped, kapi.Scheme)
}

func interfacesFor(version schema.GroupVersion) (*meta.VersionInterfaces, error) {
	switch version {
	case v1.LegacySchemeGroupVersion:
		return &meta.VersionInterfaces{
			ObjectConvertor:  kapi.Scheme,
			MetadataAccessor: accessor,
		}, nil

	default:
		g, _ := kapi.Registry.Group(api.LegacyGroupName)
		return nil, fmt.Errorf("unsupported storage version: %s (valid: %v)", version, g.GroupVersions)
	}
}
