package legacy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apimachinery"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

var (
	accessor = meta.NewAccessor()
	coreV1   = schema.GroupVersion{Group: "", Version: "v1"}
)

func LegacyInstallAll(scheme *runtime.Scheme, registry *registered.APIRegistrationManager) {
	InstallLegacyApps(scheme, registry)
	InstallLegacyAuthorization(scheme, registry)
	InstallLegacyBuild(scheme, registry)
	InstallLegacyImage(scheme, registry)
	InstallLegacyNetwork(scheme, registry)
	InstallLegacyOAuth(scheme, registry)
	InstallLegacyProject(scheme, registry)
	InstallLegacyQuota(scheme, registry)
	InstallLegacyRoute(scheme, registry)
	InstallLegacySecurity(scheme, registry)
	InstallLegacyTemplate(scheme, registry)
	InstallLegacyUser(scheme, registry)
}

func InstallLegacy(group string, addToCore, addToCoreV1 func(*runtime.Scheme) error, rootScopedKinds sets.String, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	interfacesFor := interfacesForGroup(group)

	// install core V1 types temporarily into a local scheme to enumerate them
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{coreV1}, interfacesFor)
	localScheme := runtime.NewScheme()
	if err := addToCoreV1(localScheme); err != nil {
		panic(err)
	}
	for kind := range localScheme.KnownTypes(coreV1) {
		scope := meta.RESTScopeNamespace
		if rootScopedKinds.Has(kind) {
			scope = meta.RESTScopeRoot
		}
		mapper.Add(coreV1.WithKind(kind), scope)
	}

	// register core v1 version. Should be done by kube (if the import dependencies are right).
	registry.RegisterVersions([]schema.GroupVersion{coreV1})
	if err := registry.EnableVersions(coreV1); err != nil {
		panic(err)
	}

	// register types as core v1
	if err := addToCore(scheme); err != nil {
		panic(err)
	}
	if err := addToCoreV1(scheme); err != nil {
		panic(err)
	}

	// add to group
	legacyGroupMeta := apimachinery.GroupMeta{
		GroupVersion:  coreV1,
		GroupVersions: []schema.GroupVersion{coreV1},
		RESTMapper:    mapper,
		SelfLinker:    runtime.SelfLinker(accessor),
		InterfacesFor: interfacesFor,
	}
	if err := registry.RegisterGroup(legacyGroupMeta); err != nil {
		panic(err)
	}
}

func interfacesForGroup(group string) func(version schema.GroupVersion) (*meta.VersionInterfaces, error) {
	return func(version schema.GroupVersion) (*meta.VersionInterfaces, error) {
		switch version {
		case coreV1:
			return &meta.VersionInterfaces{
				ObjectConvertor:  legacyscheme.Scheme,
				MetadataAccessor: accessor,
			}, nil

		default:
			g, _ := legacyscheme.Registry.Group(group)
			return nil, fmt.Errorf("unsupported storage version: %s (valid: %v)", version, g.GroupVersions)
		}
	}
}
