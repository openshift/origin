package install

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/api/v1"

	_ "github.com/openshift/origin/pkg/build/controller/build/defaults/api/install"
	_ "github.com/openshift/origin/pkg/build/controller/build/overrides/api/install"
	_ "github.com/openshift/origin/pkg/image/admission/imagepolicy/api/install"
	_ "github.com/openshift/origin/pkg/ingress/admission/api/install"
	_ "github.com/openshift/origin/pkg/project/admission/requestlimit/api/install"
	_ "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api/install"
	_ "github.com/openshift/origin/pkg/quota/admission/runonceduration/api/install"
	_ "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api/install"
)

var accessor = meta.NewAccessor()

// availableVersions lists all known external versions for this group from most preferred to least preferred
var availableVersions = []schema.GroupVersion{configapiv1.SchemeGroupVersion}

func init() {
	configapi.AddToScheme(configapi.Scheme)
	configapiv1.AddToScheme(configapi.Scheme)
	// we additionally need to enable audit versions, since we embed the audit
	// policy file inside master-config.yaml
	audit.AddToScheme(configapi.Scheme)
	auditv1alpha1.AddToScheme(configapi.Scheme)
	apiserver.AddToScheme(configapi.Scheme)
	apiserverv1alpha1.AddToScheme(configapi.Scheme)
}

func interfacesFor(version schema.GroupVersion) (*meta.VersionInterfaces, error) {
	switch version {
	case configapiv1.SchemeGroupVersion:
		return &meta.VersionInterfaces{
			ObjectConvertor:  configapi.Scheme,
			MetadataAccessor: accessor,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported storage version: %s", version)
	}
}

func NewRESTMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper(availableVersions, interfacesFor)
	// enumerate all supported versions, get the kinds, and register with the mapper how to address
	// our resources.
	for _, gv := range availableVersions {
		for kind := range configapi.Scheme.KnownTypes(gv) {
			mapper.Add(gv.WithKind(kind), meta.RESTScopeRoot)
		}
	}

	return mapper
}
