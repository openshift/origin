package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1alpha1api "github.com/openshift/api/operator/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceServingCertSignerConfig provides information to configure a serving serving cert signing controller
type ServiceServingCertSignerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ServingInfo is the HTTP serving information for the controller's endpoints
	ServingInfo configv1.HTTPServingInfo `json:"servingInfo,omitempty" protobuf:"bytes,1,opt,name=servingInfo"`

	// authentication allows configuration of authentication for the endpoints
	Authentication DelegatedAuthentication `json:"authentication,omitempty" protobuf:"bytes,2,opt,name=authentication"`
	// authorization allows configuration of authentication for the endpoints
	Authorization DelegatedAuthorization `json:"authorization,omitempty" protobuf:"bytes,3,opt,name=authorization"`

	// Signer holds the signing information used to automatically sign serving certificates.
	Signer configv1.CertInfo `json:"signer" protobuf:"bytes,4,opt,name=signer"`
}

// DelegatedAuthentication allows authentication to be disabled.
type DelegatedAuthentication struct {
	// disabled indicates that authentication should be disabled.  By default it will use delegated authentication.
	Disabled bool `json:"disabled,omitempty" protobuf:"varint,1,opt,name=disabled"`
}

// DelegatedAuthorization allows authorization to be disabled.
type DelegatedAuthorization struct {
	// disabled indicates that authorization should be disabled.  By default it will use delegated authorization.
	Disabled bool `json:"disabled,omitempty" protobuf:"varint,1,opt,name=disabled"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCertSignerOperatorConfig provides information to configure an operator to manage the service cert signing controllers
type ServiceCertSignerOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ServiceCertSignerOperatorConfigSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status ServiceCertSignerOperatorConfigStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

type ServiceCertSignerOperatorConfigSpec struct {
	operatorsv1alpha1api.OperatorSpec `json:",inline" protobuf:"bytes,1,opt,name=operatorSpec"`

	// serviceServingCertSignerConfig holds a sparse config that the user wants for this component.  It only needs to be the overrides from the defaults
	// it will end up overlaying in the following order:
	// 1. hardcoded default
	// 2. this config
	ServiceServingCertSignerConfig runtime.RawExtension `json:"serviceServingCertSignerConfig" protobuf:"bytes,2,opt,name=serviceServingCertSignerConfig"`
}

type ServiceCertSignerOperatorConfigStatus struct {
	operatorsv1alpha1api.OperatorStatus `json:",inline" protobuf:"bytes,1,opt,name=operatorStatus"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCertSignerOperatorConfigList is a collection of items
type ServiceCertSignerOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items contains the items
	Items []ServiceCertSignerOperatorConfig `json:"items" protobuf:"bytes,2,rep,name=items"`
}
