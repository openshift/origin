package v1

import (
	"github.com/openshift/origin/pkg/network/admission/apis/externalipranger"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (obj *ExternalIPRangerAdmissionConfig) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }

var GroupVersion = schema.GroupVersion{Group: "network.openshift.io", Version: "v1"}

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		externalipranger.Install,
	)
	Install = schemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&ExternalIPRangerAdmissionConfig{},
	)
	return nil
}
