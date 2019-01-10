package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceServingCertSignerConfig provides information to configure a serving serving cert signing controller
type ServiceServingCertSignerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// This configuration is not meant to be edited by humans as
	// it is normally managed by the service cert signer operator.
	// ServiceCertSignerOperatorConfig's spec.serviceServingCertSignerConfig
	// can be used to override the defaults for this configuration.
	configv1.GenericControllerConfig `json:",inline"`

	// signer holds the signing information used to automatically sign serving certificates.
	Signer configv1.CertInfo `json:"signer"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// APIServiceCABundleInjectorConfig provides information to configure an APIService CA Bundle Injector controller
type APIServiceCABundleInjectorConfig struct {
	metav1.TypeMeta `json:",inline"`

	// This configuration is not meant to be edited by humans as
	// it is normally managed by the service cert signer operator.
	// ServiceCertSignerOperatorConfig's spec.apiServiceCABundleInjectorConfig
	// can be used to override the defaults for this configuration.
	configv1.GenericControllerConfig `json:",inline"`

	// caBundleFile holds the ca bundle to apply to APIServices.
	CABundleFile string `json:"caBundleFile"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigMapCABundleInjectorConfig provides information to configure a ConfigMap CA Bundle Injector controller
type ConfigMapCABundleInjectorConfig struct {
	metav1.TypeMeta `json:",inline"`

	// This configuration is not meant to be edited by humans as
	// it is normally managed by the service cert signer operator.
	// ServiceCertSignerOperatorConfig's spec.configMapCABundleInjectorConfig
	// can be used to override the defaults for this configuration.
	configv1.GenericControllerConfig `json:",inline"`

	// caBundleFile holds the ca bundle to apply to ConfigMaps.
	CABundleFile string `json:"caBundleFile"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCertSignerOperatorConfig provides information to configure an operator to manage the service cert signing controllers
type ServiceCertSignerOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   ServiceCertSignerOperatorConfigSpec   `json:"spec"`
	Status ServiceCertSignerOperatorConfigStatus `json:"status"`
}

type ServiceCertSignerOperatorConfigSpec struct {
	operatorv1.OperatorSpec `json:",inline"`

	// serviceServingCertSignerConfig holds a sparse config that the user wants for this component.  It only needs to be the overrides from the defaults
	// it will end up overlaying in the following order:
	// 1. hardcoded default
	// 2. this config
	ServiceServingCertSignerConfig runtime.RawExtension `json:"serviceServingCertSignerConfig"`

	// apiServiceCABundleInjectorConfig holds a sparse config that the user wants for this component.  It only needs to be the overrides from the defaults
	// it will end up overlaying in the following order:
	// 1. hardcoded default
	// 2. this config
	APIServiceCABundleInjectorConfig runtime.RawExtension `json:"apiServiceCABundleInjectorConfig"`

	// configMapCABundleInjectorConfig holds a sparse config that the user wants for this component.  It only needs to be the overrides from the defaults
	// it will end up overlaying in the following order:
	// 1. hardcoded default
	// 2. this config
	ConfigMapCABundleInjectorConfig runtime.RawExtension `json:"configMapCABundleInjectorConfig"`
}

type ServiceCertSignerOperatorConfigStatus struct {
	operatorv1.OperatorStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceCertSignerOperatorConfigList is a collection of items
type ServiceCertSignerOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items contains the items
	Items []ServiceCertSignerOperatorConfig `json:"items"`
}
