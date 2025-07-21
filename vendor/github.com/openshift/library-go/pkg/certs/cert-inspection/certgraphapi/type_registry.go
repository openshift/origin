package certgraphapi

type PKIRegistryCertKeyPair struct {
	InClusterLocation   *PKIRegistryInClusterCertKeyPair
	OnDiskLocation      *PKIRegistryOnDiskCertKeyPair
	InMemoryPodLocation *PKIRegistryInMemoryCertKeyPair
}

// PKIRegistryOnDiskCertKeyPair identifies certificate key pair on disk and stores its metadata
type PKIRegistryOnDiskCertKeyPair struct {
	// OnDiskLocation points to the certkeypair location on disk
	OnDiskLocation OnDiskLocation `json:"onDiskLocation"`
	// CertKeyInfo stores metadata for certificate key pair
	CertKeyInfo PKIRegistryCertKeyPairInfo `json:"certKeyInfo"`
}

// PKIRegistryInMemoryCertKeyPair identifies certificate key pair and stores its metadata
type PKIRegistryInMemoryCertKeyPair struct {
	// PodLocation points to the pod location
	PodLocation InClusterPodLocation `json:"podLocation"`
	// CertKeyInfo stores metadata for certificate key pair
	CertKeyInfo PKIRegistryCertKeyPairInfo `json:"certKeyInfo"`
}

// PKIRegistryInClusterCertKeyPair identifies certificate key pair and stores its metadata
type PKIRegistryInClusterCertKeyPair struct {
	// SecretLocation points to the secret location
	SecretLocation InClusterSecretLocation `json:"secretLocation"`
	// CertKeyInfo stores metadata for certificate key pair
	CertKeyInfo PKIRegistryCertKeyPairInfo `json:"certKeyInfo"`
}

// PKIRegistryCertKeyPairInfo holds information about certificate key pair
type PKIRegistryCertKeyPairInfo struct {
	// SelectedCertMetadataAnnotations is a specified subset of annotations. NOT all annotations.
	// The caller will specify which annotations he wants.
	SelectedCertMetadataAnnotations []AnnotationValue `json:"selectedCertMetadataAnnotations,omitempty"`

	// OwningJiraComponent is a component name when a new OCP issue is filed in Jira
	// Deprecated
	OwningJiraComponent string `json:"owningJiraComponent"`
	// Description is a one sentence description of the certificate pair purpose
	// Deprecated
	Description string `json:"description"`

	//CertificateData PKIRegistryCertKeyMetadata
}

// PKIRegistryInClusterCABundle holds information about certificate authority bundle
type PKIRegistryInClusterCABundle struct {
	// ConfigMapLocation points to the configmap location
	ConfigMapLocation InClusterConfigMapLocation `json:"configMapLocation"`
	// CABundleInfo stores metadata for the certificate authority bundle
	CABundleInfo PKIRegistryCertificateAuthorityInfo `json:"certificateAuthorityBundleInfo"`
}

type PKIRegistryCABundle struct {
	InClusterLocation *PKIRegistryInClusterCABundle
	OnDiskLocation    *PKIRegistryOnDiskCABundle
}

// PKIRegistryOnDiskCABundle identifies certificate key pair on disk and stores its metadata
type PKIRegistryOnDiskCABundle struct {
	// OnDiskLocation points to the ca bundle location on disk
	OnDiskLocation OnDiskLocation `json:"onDiskLocation"`
	// CABundleInfo stores metadata for the certificate authority bundle
	CABundleInfo PKIRegistryCertificateAuthorityInfo `json:"certificateAuthorityBundleInfo"`
}

// PKIRegistryCertificateAuthorityInfo holds information about certificate authority bundle
type PKIRegistryCertificateAuthorityInfo struct {
	// SelectedCertMetadataAnnotations is a specified subset of annotations. NOT all annotations.
	// The caller will specify which annotations he wants.
	SelectedCertMetadataAnnotations []AnnotationValue `json:"selectedCertMetadataAnnotations,omitempty"`

	// OwningJiraComponent is a component name when a new OCP issue is filed in Jira
	// Deprecated
	OwningJiraComponent string `json:"owningJiraComponent"`
	// Description is a one sentence description of the certificate pair purpose
	// Deprecated
	Description string `json:"description"`
}

type AnnotationValue struct {
	// Key is the annotation key from the resource
	Key string `json:"key"`
	// Value is the annotation value from the resource
	Value string `json:"value"`
}
