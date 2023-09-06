package certgraphapi


type PKIRegistryInfo struct {
	CertificateAuthorityBundles []PKIRegistryInClusterCABundle `json:"certificateAuthorityBundles"`
	CertKeyPairs                []PKIRegistryInClusterCertKeyPair `json:"certKeyPairs"`
}

type PKIRegistryInClusterCertKeyPair struct{
	SecretLocation InClusterSecretLocation  `json:"secretLocation"`

	CertKeyInfo PKIRegistryCertKeyPairInfo  `json:"certKeyInfo"`
}

type PKIRegistryCertKeyPairInfo struct{
	OwningJiraComponent string `json:"owningJiraComponent"`
	HumanName string  `json:"humanName"`
	Description string  `json:"description"`

	//CertificateData PKIRegistryCertKeyMetadata
}

type PKIRegistryInClusterCABundle struct{
	ConfigMapLocation InClusterConfigMapLocation `json:"configMapLocation"`

	CABundleInfo PKIRegistryCertificateAuthorityInfo `json:"certificateAuthorityBundleInfo"`
}

type PKIRegistryCertificateAuthorityInfo struct{
	OwningJiraComponent string `json:"owningJiraComponent"`
	HumanName string  `json:"humanName"`
	Description string  `json:"description"`

	//CertificateData []PKIRegistryCertKeyMetadata
}

// TODO this could be added if we plan to enforce it.
//type PKIRegistryCertKeyMetadata struct {
//	SignatureAlgorithm string `json:"signatureAlgorithm,omitempty"`
//	PublicKeyAlgorithm string `json:"publicKeyAlgorithm,omitempty"`
//	PublicKeyBitSize   string `json:"publicKeyBitSize,omitempty"`
//	ValidityDuration   string `json:"validityDuration,omitempty"`
//	Usages             []string `json:"usages,omitempty"`
//	ExtendedUsages     []string `json:"extendedUsages,omitempty"`
//}