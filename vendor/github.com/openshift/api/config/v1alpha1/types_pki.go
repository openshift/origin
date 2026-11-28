package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PKI configures cryptographic parameters for certificates generated
// internally by OpenShift components.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
//
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=pkis,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2645
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=config-operator,operatorOrdering=01
// +openshift:enable:FeatureGate=ConfigurablePKI
// +openshift:compatibility-gen:level=4
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="pki is a singleton, .metadata.name must be 'cluster'"
type PKI struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +required
	Spec PKISpec `json:"spec,omitzero"`
}

// PKISpec holds the specification for PKI configuration.
// Exactly one of certificates or certificateManagement must be set.
//
// +kubebuilder:validation:ExactlyOneOf=certificates;certificateManagement
type PKISpec struct {
	// certificates specifies the cluster's certificate policy for internally-generated
	// certificates, resolved most-specific-wins: a named certificate override, then a
	// category override (CA, server, client), then the required defaults.
	// This field is optional and mutually exclusive with the deprecated
	// certificateManagement field; exactly one of the two must be set.
	// This field names a configuration policy; it is unrelated to the X.509
	// certificate-policies extension.
	//
	// +optional
	Certificates PKICertificatePolicy `json:"certificates,omitzero"`

	// keys optionally configures keys generated outside of any certificate, such as
	// the bound service-account token signing key. Entries are named and opt-in with no
	// inherited default. When omitted, no keys are configured by this resource.
	// When set, at least one named key must be specified; an empty object is rejected.
	// Unlike certificates, keys have no resolution chain: a key is either configured
	// here or it is not, and a component whose key is not configured decides for
	// itself (an internal default, or an error if it has none).
	//
	// +optional
	Keys PKIKeyPolicy `json:"keys,omitempty,omitzero"`

	// certificateManagement is the former shape of certificates, a mode union
	// (Unmanaged/Default/Custom). It is retained only so that clients written against
	// it keep compiling and so that objects using it keep roundtripping; it is mutually
	// exclusive with certificates and exactly one of the two must be set.
	// Note that key validation is tighter than it was when this field was current:
	// RSA key sizes above 4096 bits, which were previously accepted, are now rejected.
	//
	// Deprecated: use certificates instead.
	//
	// +optional
	CertificateManagement PKICertificateManagement `json:"certificateManagement,omitzero"`
}

// PKICertificatePolicy is the cluster's policy for internally-generated certificates:
// a required defaults, optional per-category overrides, and optional overrides for
// specific named certificates. The most-specific setting wins.
type PKICertificatePolicy struct {
	// defaults specifies the default certificate configuration applied to any certificate
	// that has no more-specific (category or named) override. This field is required so
	// that every certificate has a well-defined configuration.
	//
	// +required
	Defaults PKIDefaultCertificateConfig `json:"defaults,omitzero"`

	// caCertificates optionally overrides certificate parameters for certificate
	// authority (CA) certificates that sign other certificates. When set, these
	// parameters take precedence over defaults for all CA certificates. When omitted,
	// defaults are used for CA certificates.
	//
	// +optional
	CACertificates PKICertificateConfig `json:"caCertificates,omitempty,omitzero"`

	// serverCertificates optionally overrides certificate parameters for TLS server
	// certificates. When set, these parameters take precedence over defaults for all
	// server certificates. When omitted, defaults are used for server certificates.
	//
	// +optional
	ServerCertificates PKICertificateConfig `json:"serverCertificates,omitempty,omitzero"`

	// clientCertificates optionally overrides certificate parameters for client
	// authentication certificates. When set, these parameters take precedence over
	// defaults for all client certificates. When omitted, defaults are used for client
	// certificates.
	//
	// +optional
	ClientCertificates PKICertificateConfig `json:"clientCertificates,omitempty,omitzero"`

	// namedCertificates optionally overrides certificate parameters for specific
	// certificates identified by their well-known name. These overrides have the highest
	// precedence (named, then category, then defaults). Entries are keyed by name, so a
	// given name may appear at most once. When omitted, no named overrides are applied.
	// When present, the list must contain at least one entry. At most 512 named
	// certificate overrides may be specified.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=512
	// +optional
	NamedCertificates []PKINamedCertificateConfig `json:"namedCertificates,omitempty"`
}

// PKINamedCertificateConfig overrides certificate parameters for a specific certificate
// identified by its well-known name.
type PKINamedCertificateConfig struct {
	// name is the well-known override name of the targeted certificate. It is a DNS
	// subdomain prefix, a "/", then a name made of alphanumerics, "-", "_" and "."
	// (for example "openshift.io/service-ca.service-serving-signer"). The prefix is at
	// most 253 characters and the name after the "/" at most 63, so the full name
	// must be between 1 and 317 characters. The name is validated
	// for format only; there is no registry of valid names, because certificates can be
	// registered dynamically. Core-platform certificates use the bare "openshift.io/"
	// prefix; names outside the core platform use an owner subdomain prefix (for example
	// "hypershift.openshift.io/").
	// This field is required so that every named certificate override identifies the
	// certificate it applies to.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=317
	// +kubebuilder:validation:XValidation:rule="self.contains('/') && !format.qualifiedName().validate(self).hasValue()",message="name must be a prefixed qualified name, for example openshift.io/etcd.signer"
	Name string `json:"name,omitempty"`

	// PKICertificateConfig holds the overridden parameters for this certificate.
	PKICertificateConfig `json:",inline"`
}

// PKIKeyPolicy configures keys generated outside of any certificate, by well-known name.
// When set, at least one named key must be specified; an empty object is rejected.
type PKIKeyPolicy struct {
	// namedKeys configures keys generated outside of any certificate, such as the bound
	// service-account token signing key, identified by their well-known name. Entries are
	// opt-in with no inherited default and are keyed by name, so a given name may appear at
	// most once. This field is required so that a keys object is never empty.
	// At most 16 named keys may be specified.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +required
	NamedKeys []PKINamedKeyConfig `json:"namedKeys,omitempty,omitzero"`
}

// PKINamedKeyConfig configures the cryptographic parameters for a single key generated
// outside of any certificate, identified by its well-known name.
type PKINamedKeyConfig struct {
	// name is the well-known name of the key. It is a DNS subdomain prefix, a
	// "/", then a name made of alphanumerics, "-", "_" and "." (for example
	// "openshift.io/kube-apiserver.bound-service-account-token-signer"). The prefix is at
	// most 253 characters and the name after the "/" at most 63, so the full name
	// must be between 1 and 317 characters. The name is validated
	// for format only, using the same rules as a named certificate override.
	// This field is required so that every named key entry identifies the key it configures.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=317
	// +kubebuilder:validation:XValidation:rule="self.contains('/') && !format.qualifiedName().validate(self).hasValue()",message="name must be a prefixed qualified name, for example openshift.io/etcd.signer"
	Name string `json:"name,omitempty"`

	// key specifies the cryptographic parameters for this key. It is required
	// because keys configured here have no inherited default.
	//
	// +required
	Key PKIKeyConfig `json:"key,omitzero"`
}

// PKIDefaultCertificateConfig specifies the default certificate configuration
// parameters. All fields are required to ensure that defaults are fully
// specified for all certificates.
type PKIDefaultCertificateConfig struct {
	// key specifies the cryptographic parameters for the certificate's key pair.
	// This field is required in defaults to ensure all certificates have a
	// well-defined key configuration.
	// +required
	Key PKIKeyConfig `json:"key,omitzero"`
}

// PKICertificateConfig specifies configuration parameters for certificates.
type PKICertificateConfig struct {
	// key specifies the cryptographic parameters for the certificate's key pair.
	// +required
	Key PKIKeyConfig `json:"key,omitzero"`
}

// PKIKeyConfig specifies cryptographic parameters for key generation.
//
// +kubebuilder:validation:XValidation:rule="has(self.algorithm) && self.algorithm == 'RSA' ?  has(self.rsa) : !has(self.rsa)",message="rsa is required when algorithm is RSA, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="has(self.algorithm) && self.algorithm == 'ECDSA' ?  has(self.ecdsa) : !has(self.ecdsa)",message="ecdsa is required when algorithm is ECDSA, and forbidden otherwise"
// +union
type PKIKeyConfig struct {
	// algorithm specifies the key generation algorithm.
	// Valid values are "RSA" and "ECDSA".
	//
	// When set to RSA, the rsa field must be specified and the generated key
	// will be an RSA key with the configured key size.
	//
	// When set to ECDSA, the ecdsa field must be specified and the generated key
	// will be an ECDSA key using the configured elliptic curve.
	//
	// +required
	// +unionDiscriminator
	Algorithm PKIKeyAlgorithm `json:"algorithm,omitempty"`

	// rsa specifies RSA key parameters.
	// Required when algorithm is RSA, and forbidden otherwise.
	// +optional
	// +unionMember
	RSA PKIRSAKeyConfig `json:"rsa,omitzero"`

	// ecdsa specifies ECDSA key parameters.
	// Required when algorithm is ECDSA, and forbidden otherwise.
	// +optional
	// +unionMember
	ECDSA PKIECDSAKeyConfig `json:"ecdsa,omitzero"`
}

// PKIRSAKeyConfig specifies parameters for RSA key generation.
type PKIRSAKeyConfig struct {
	// keySize specifies the size of RSA keys in bits.
	// Valid values are multiples of 1024 from 2048 to 4096.
	// +required
	// +kubebuilder:validation:Minimum=2048
	// +kubebuilder:validation:Maximum=4096
	// +kubebuilder:validation:MultipleOf=1024
	KeySize int32 `json:"keySize,omitempty"`
}

// PKIECDSAKeyConfig specifies parameters for ECDSA key generation.
type PKIECDSAKeyConfig struct {
	// curve specifies the NIST elliptic curve for ECDSA keys.
	// Valid values are "P256", "P384", and "P521".
	//
	// When set to P256, the NIST P-256 curve (also known as secp256r1) is used,
	// providing 128-bit security.
	//
	// When set to P384, the NIST P-384 curve (also known as secp384r1) is used,
	// providing 192-bit security.
	//
	// When set to P521, the NIST P-521 curve (also known as secp521r1) is used,
	// providing 256-bit security.
	//
	// +required
	Curve PKIECDSACurve `json:"curve,omitempty"`
}

// PKIKeyAlgorithm specifies the cryptographic algorithm used for key generation.
//
// +kubebuilder:validation:Enum=RSA;ECDSA
type PKIKeyAlgorithm string

const (
	// PKIKeyAlgorithmRSA specifies the RSA (Rivest-Shamir-Adleman) algorithm for key generation.
	PKIKeyAlgorithmRSA PKIKeyAlgorithm = "RSA"

	// PKIKeyAlgorithmECDSA specifies the ECDSA (Elliptic Curve Digital Signature Algorithm) for key generation.
	PKIKeyAlgorithmECDSA PKIKeyAlgorithm = "ECDSA"
)

// PKIECDSACurve specifies the elliptic curve used for ECDSA key generation.
//
// +kubebuilder:validation:Enum=P256;P384;P521
type PKIECDSACurve string

const (
	// PKIECDSACurveP256 specifies the NIST P-256 curve (also known as secp256r1), providing 128-bit security.
	PKIECDSACurveP256 PKIECDSACurve = "P256"

	// PKIECDSACurveP384 specifies the NIST P-384 curve (also known as secp384r1), providing 192-bit security.
	PKIECDSACurveP384 PKIECDSACurve = "P384"

	// PKIECDSACurveP521 specifies the NIST P-521 curve (also known as secp521r1), providing 256-bit security.
	PKIECDSACurveP521 PKIECDSACurve = "P521"
)

// PKICertificateManagement is the former container for certificate configuration,
// a mode union determining whether components use hardcoded defaults (Unmanaged),
// follow OpenShift best practices (Default), or use administrator-specified
// cryptographic parameters (Custom).
//
// Deprecated: use PKICertificatePolicy instead.
//
// +kubebuilder:validation:XValidation:rule="self.mode == 'Custom' ? has(self.custom) : !has(self.custom)",message="custom is required when mode is Custom, and forbidden otherwise"
// +union
type PKICertificateManagement struct {
	// mode determines how PKI configuration is managed.
	// Valid values are "Unmanaged", "Default", and "Custom".
	//
	// When set to Unmanaged, components use their existing hardcoded certificate
	// generation behavior, exactly as if this feature did not exist. Each component
	// generates certificates using whatever parameters it was using before this
	// feature. While most components use RSA 2048, some may use different
	// parameters. Use of this mode might prevent upgrading to the next major
	// OpenShift release.
	//
	// When set to Default, OpenShift-recommended best practices for certificate
	// generation are applied. The specific parameters may evolve across OpenShift
	// releases to adopt improved cryptographic standards. In the initial release,
	// this matches Unmanaged behavior for each component. In future releases, this
	// may adopt ECDSA or larger RSA keys based on industry best practices.
	// Recommended for most customers who want to benefit from security improvements
	// automatically.
	//
	// When set to Custom, the certificate management parameters can be set
	// explicitly. Use the custom field to specify certificate generation parameters.
	//
	// Deprecated: configure PKICertificatePolicy via the certificates field instead.
	//
	// +required
	// +unionDiscriminator
	Mode PKICertificateManagementMode `json:"mode,omitempty"`

	// custom contains administrator-specified cryptographic configuration.
	// Use the defaults and category override fields of the embedded PKIProfile
	// to specify certificate generation parameters.
	// Required when mode is Custom, and forbidden otherwise.
	// When omitted, mode governs behavior on its own.
	//
	// Deprecated: use the fields of PKICertificatePolicy instead.
	//
	// +optional
	// +unionMember
	Custom CustomPKIPolicy `json:"custom,omitzero"`
}

// CustomPKIPolicy contains administrator-specified cryptographic configuration,
// wrapping a PKIProfile. When set, at least one property must be specified.
//
// Deprecated: the wrapped fields were flattened into PKICertificatePolicy.
//
// +kubebuilder:validation:MinProperties=1
type CustomPKIPolicy struct {
	// PKIProfile holds the administrator-specified certificate generation parameters.
	//
	// Deprecated: use PKICertificatePolicy instead.
	PKIProfile `json:",inline"`
}

// PKICertificateManagementMode specifies the mode for PKI certificate management.
// Valid values are "Unmanaged", "Default", and "Custom".
//
// Deprecated: the mode union was replaced by the flat PKICertificatePolicy.
//
// +kubebuilder:validation:Enum=Unmanaged;Default;Custom
type PKICertificateManagementMode string

const (
	// PKICertificateManagementModeUnmanaged uses each component's existing hardcoded defaults.
	// Most components currently use RSA 2048, but parameters may differ by component.
	//
	// Deprecated: the mode union was replaced by the flat PKICertificatePolicy.
	PKICertificateManagementModeUnmanaged PKICertificateManagementMode = "Unmanaged"

	// PKICertificateManagementModeDefault uses OpenShift-recommended best practices.
	// Specific parameters may evolve across OpenShift releases.
	//
	// Deprecated: the mode union was replaced by the flat PKICertificatePolicy.
	PKICertificateManagementModeDefault PKICertificateManagementMode = "Default"

	// PKICertificateManagementModeCustom uses administrator-specified configuration.
	//
	// Deprecated: the mode union was replaced by the flat PKICertificatePolicy.
	PKICertificateManagementModeCustom PKICertificateManagementMode = "Custom"
)

// PKIProfile defines the certificate generation parameters that OpenShift
// components use to create certificates. Category overrides take precedence
// over defaults.
//
// Deprecated: use PKICertificatePolicy instead.
type PKIProfile struct {
	// defaults specifies the default certificate configuration that applies
	// to all certificates unless overridden by a category override.
	// This field is required so that every certificate has a well-defined configuration.
	//
	// Deprecated: use PKICertificatePolicy.Defaults instead.
	//
	// +required
	Defaults PKIDefaultCertificateConfig `json:"defaults,omitzero"`

	// signerCertificates optionally overrides certificate parameters for
	// certificate authority (CA) certificates that sign other certificates.
	// When set, these parameters take precedence over defaults for all signer certificates.
	// When omitted, the defaults are used for signer certificates.
	// The key field within it is now marked required (previously enforced
	// via minProperties: 1).
	//
	// Deprecated: use PKICertificatePolicy.CACertificates instead.
	//
	// +optional
	SignerCertificates PKICertificateConfig `json:"signerCertificates,omitempty,omitzero"`

	// servingCertificates optionally overrides certificate parameters for
	// TLS server certificates used to serve HTTPS endpoints.
	// When set, these parameters take precedence over defaults for all serving certificates.
	// When omitted, the defaults are used for serving certificates.
	// The key field within it is now marked required (previously enforced
	// via minProperties: 1).
	//
	// Deprecated: use PKICertificatePolicy.ServerCertificates instead.
	//
	// +optional
	ServingCertificates PKICertificateConfig `json:"servingCertificates,omitempty,omitzero"`

	// clientCertificates optionally overrides certificate parameters for
	// client authentication certificates used to authenticate to servers.
	// When set, these parameters take precedence over defaults for all client certificates.
	// When omitted, the defaults are used for client certificates.
	// The key field within it is now marked required (previously enforced
	// via minProperties: 1).
	//
	// Deprecated: use PKICertificatePolicy.ClientCertificates instead.
	//
	// +optional
	ClientCertificates PKICertificateConfig `json:"clientCertificates,omitempty,omitzero"`
}

// The following types and constants are deprecated aliases of their PKI-prefixed
// replacements, retained so that clients written against the former names keep
// compiling. Each carries a //go:fix inline directive so that gopls offers to
// rewrite usages to the replacement automatically.

// DefaultCertificateConfig specifies the default certificate configuration parameters.
//
// Deprecated: use PKIDefaultCertificateConfig instead.
//
//go:fix inline
type DefaultCertificateConfig = PKIDefaultCertificateConfig

// CertificateConfig specifies configuration parameters for certificates.
// Its key field is now marked required (previously enforced via
// minProperties: 1).
//
// Deprecated: use PKICertificateConfig instead.
//
//go:fix inline
type CertificateConfig = PKICertificateConfig

// KeyConfig specifies cryptographic parameters for key generation.
//
// Deprecated: use PKIKeyConfig instead.
//
//go:fix inline
type KeyConfig = PKIKeyConfig

// RSAKeyConfig specifies parameters for RSA key generation.
// Unlike when this type was current, key sizes are capped at 4096 bits.
//
// Deprecated: use PKIRSAKeyConfig instead.
//
//go:fix inline
type RSAKeyConfig = PKIRSAKeyConfig

// ECDSAKeyConfig specifies parameters for ECDSA key generation.
//
// Deprecated: use PKIECDSAKeyConfig instead.
//
//go:fix inline
type ECDSAKeyConfig = PKIECDSAKeyConfig

// KeyAlgorithm specifies the cryptographic algorithm used for key generation.
//
// Deprecated: use PKIKeyAlgorithm instead.
//
//go:fix inline
type KeyAlgorithm = PKIKeyAlgorithm

// ECDSACurve specifies the elliptic curve used for ECDSA key generation.
//
// Deprecated: use PKIECDSACurve instead.
//
//go:fix inline
type ECDSACurve = PKIECDSACurve

// KeyAlgorithmRSA specifies the RSA (Rivest-Shamir-Adleman) algorithm for key generation.
//
// Deprecated: use PKIKeyAlgorithmRSA instead.
//
//go:fix inline
const KeyAlgorithmRSA = PKIKeyAlgorithmRSA

// KeyAlgorithmECDSA specifies the ECDSA (Elliptic Curve Digital Signature Algorithm) for key generation.
//
// Deprecated: use PKIKeyAlgorithmECDSA instead.
//
//go:fix inline
const KeyAlgorithmECDSA = PKIKeyAlgorithmECDSA

// ECDSACurveP256 specifies the NIST P-256 curve (also known as secp256r1), providing 128-bit security.
//
// Deprecated: use PKIECDSACurveP256 instead.
//
//go:fix inline
const ECDSACurveP256 = PKIECDSACurveP256

// ECDSACurveP384 specifies the NIST P-384 curve (also known as secp384r1), providing 192-bit security.
//
// Deprecated: use PKIECDSACurveP384 instead.
//
//go:fix inline
const ECDSACurveP384 = PKIECDSACurveP384

// ECDSACurveP521 specifies the NIST P-521 curve (also known as secp521r1), providing 256-bit security.
//
// Deprecated: use PKIECDSACurveP521 instead.
//
//go:fix inline
const ECDSACurveP521 = PKIECDSACurveP521

// PKIList is a collection of PKI resources.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +openshift:compatibility-gen:level=4
type PKIList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of PKI resources
	Items []PKI `json:"items"`
}
