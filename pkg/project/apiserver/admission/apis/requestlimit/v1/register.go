package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit"
)

const (
	GroupName           = "requestlimit.project.openshift.io"
	DeprecatedGroupName = ""
)

var (
	SchemeGroupVersion           = schema.GroupVersion{Group: GroupName, Version: "v1"}
	DeprecatedSchemeGroupVersion = schema.GroupVersion{Group: DeprecatedGroupName, Version: "v1"}

	DeprecatedSchemeBuilder = runtime.NewSchemeBuilder(
		addDeprecatedKnownTypes,
		requestlimit.InstallLegacy,
	)
	DeprecatedInstall = DeprecatedSchemeBuilder.AddToScheme

	SchemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
		requestlimit.Install,
	)
	Install = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addDeprecatedKnownTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&ProjectRequestLimitConfig{},
	}
	scheme.AddKnownTypes(DeprecatedSchemeGroupVersion, types...)
	return nil
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ProjectRequestLimitConfig{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
