package install

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"

	_ "github.com/openshift/origin/pkg/build/controller/build/apis/defaults/install"
	_ "github.com/openshift/origin/pkg/build/controller/build/apis/overrides/install"
	_ "github.com/openshift/origin/pkg/image/admission/apis/imagepolicy/install"
	_ "github.com/openshift/origin/pkg/ingress/admission/apis/ingressadmission/install"
	_ "github.com/openshift/origin/pkg/project/admission/apis/requestlimit/install"
	_ "github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride/install"
	_ "github.com/openshift/origin/pkg/quota/admission/apis/runonceduration/install"
	_ "github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints/install"
)

var accessor = meta.NewAccessor()

// availableVersions lists all known external versions for this group from most preferred to least preferred
var availableVersions = []schema.GroupVersion{configapiv1.SchemeGroupVersion}

func init() {
	AddToScheme(configapi.Scheme)
}

func AddToScheme(scheme *runtime.Scheme) {
	configapi.AddToScheme(scheme)
	configapiv1.AddToScheme(scheme)
	// we additionally need to enable audit versions, since we embed the audit
	// policy file inside master-config.yaml
	audit.AddToScheme(scheme)
	auditv1alpha1.AddToScheme(scheme)
	auditv1beta1.AddToScheme(scheme)
	apiserver.AddToScheme(scheme)
	apiserverv1alpha1.AddToScheme(scheme)
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
