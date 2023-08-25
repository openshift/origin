package certgraphapi

import (
	"encoding/json"
)

type PKIList struct {
	// LogicalName is an inexact representation of what this is for.  It may be empty.  It will usually be some hardcoded
	// heuristic trying to find it.
	LogicalName string

	Description string

	CertificateAuthorityBundles CertificateAuthorityBundleList
	CertKeyPairs                CertKeyPairList
}

type CertificateAuthorityBundleList struct {
	Items []CertificateAuthorityBundle
}

type CertificateAuthorityBundle struct {
	// LogicalName is an inexact representation of what this is for.  It may be empty.  It will usually be some hardcoded
	// heuristic trying to determine it.
	LogicalName string

	Description string

	// Name is CommonName::SerialNumber
	Name string

	Spec   CertificateAuthorityBundleSpec
	Status CertificateAuthorityBundleStatus
}

type CertificateAuthorityBundleStatus struct {
	Errors []string
}

type CertificateAuthorityBundleSpec struct {
	ConfigMapLocations []InClusterConfigMapLocation
	OnDiskLocations    []OnDiskLocation

	CertificateMetadata []CertKeyMetadata
}

type CertKeyPairList struct {
	Items []CertKeyPair
}

type CertKeyPair struct {
	// LogicalName is an inexact representation of what this is for.  It may be empty.  It will usually be some hardcoded
	// heuristic trying to determine it.
	LogicalName string

	Description string

	// Name is CommonName::SerialNumber
	Name string

	Spec   CertKeyPairSpec
	Status CertKeyPairStatus
}

type CertKeyPairStatus struct {
	Errors []string
}

type CertKeyPairSpec struct {
	SecretLocations []InClusterSecretLocation
	OnDiskLocations []OnDiskCertKeyPairLocation

	CertMetadata CertKeyMetadata
	Details      CertKeyPairDetails
}

type InClusterSecretLocation struct {
	Namespace string
	Name      string
}

type InClusterConfigMapLocation struct {
	Namespace string
	Name      string
}

type OnDiskCertKeyPairLocation struct {
	Cert OnDiskLocation
	Key  OnDiskLocation
}

type OnDiskLocation struct {
	Path           string
	User           string
	Group          string
	Permissions    string
	SELinuxOptions string
}

type CertKeyPairDetails struct {
	CertType string

	SignerDetails      *SignerCertDetails
	ServingCertDetails *ServingCertDetails
	ClientCertDetails  *ClientCertDetails
}

type SignerCertDetails struct {
}

type ServingCertDetails struct {
	DNSNames    []string
	IPAddresses []string
}

type ClientCertDetails struct {
	Organizations []string
}

type CertIdentifier struct {
	CommonName   string
	SerialNumber string

	Issuer *CertIdentifier
}

type CertKeyMetadata struct {
	CertIdentifier     CertIdentifier
	SignatureAlgorithm string
	PublicKeyAlgorithm string
	PublicKeyBitSize   string
	ValidityDuration   string
	Usages             []string
	ExtendedUsages     []string
}

// do better
func (t *CertKeyPair) DeepCopy() *CertKeyPair {
	jsonBytes, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}

	ret := &CertKeyPair{}
	if err := json.Unmarshal(jsonBytes, ret); err != nil {
		panic(err)
	}

	return ret
}

// do better
func (t *CertificateAuthorityBundle) DeepCopy() *CertificateAuthorityBundle {
	jsonBytes, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}

	ret := &CertificateAuthorityBundle{}
	if err := json.Unmarshal(jsonBytes, ret); err != nil {
		panic(err)
	}

	return ret
}
