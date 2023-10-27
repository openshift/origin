package certgraphapi

import (
	"encoding/json"
)

type PKIList struct {
	// LogicalName is an inexact representation of what this is for.  It may be empty.  It will usually be some hardcoded
	// heuristic trying to find it.
	LogicalName string

	Description string

	InClusterResourceData PerInClusterResourceData

	CertificateAuthorityBundles CertificateAuthorityBundleList
	CertKeyPairs                CertKeyPairList
}

// PerInClusterResourceData tracks metadata that corresponds to specific secrets and configmaps.
// This data should not duplicate the analysis of the certkeypair lists, but is pulled from annotations on the resources.
// It will be stitched together by a generator after the fact.
type PerInClusterResourceData struct {
	// +mapType:=atomic
	CertificateAuthorityBundles []RawPKIRegistryInClusterCABundle `json:"certificateAuthorityBundles"`
	// +mapType:=atomic
	CertKeyPairs []RawPKIRegistryInClusterCertKeyPair `json:"certKeyPairs"`
}

// RawPKIRegistryInClusterCertKeyPair identifies certificate key pair and stores its metadata
type RawPKIRegistryInClusterCertKeyPair struct {
	// SecretLocation points to the secret location
	SecretLocation InClusterSecretLocation `json:"secretLocation"`
	// CertKeyInfo stores metadata for certificate key pair
	CertKeyInfo RawPKIRegistryCertKeyPairInfo `json:"certKeyInfo"`
}

// RawPKIRegistryCertKeyPairInfo holds information about certificate key pair
type RawPKIRegistryCertKeyPairInfo struct {
	// OwningJiraComponent is a component name when a new OCP issue is filed in Jira
	OwningJiraComponent string `json:"owningJiraComponent"`
	// Description is a one sentence description of the certificate pair purpose
	Description string `json:"description"`

	// revisionedSource indicates which secret this one is revisioned from.
	// If it is nil, then this secret is not revisioned.
	// Revisioned secrets are the "foo-%d" secrets you see.
	RevisionedSource *InClusterSecretLocation `json:"revisionedSource"`

	//CertificateData PKIRegistryCertKeyMetadata
}

// RawPKIRegistryInClusterCABundle holds information about certificate authority bundle
type RawPKIRegistryInClusterCABundle struct {
	// ConfigMapLocation points to the configmap location
	ConfigMapLocation InClusterConfigMapLocation `json:"configMapLocation"`
	// CABundleInfo stores metadata for the certificate authority bundle
	CABundleInfo RawPKIRegistryCertificateAuthorityInfo `json:"certificateAuthorityBundleInfo"`
}

// RawPKIRegistryCertificateAuthorityInfo holds information about certificate authority bundle
type RawPKIRegistryCertificateAuthorityInfo struct {
	// OwningJiraComponent is a component name when a new OCP issue is filed in Jira
	OwningJiraComponent string `json:"owningJiraComponent"`
	// Description is a one sentence description of the certificate pair purpose
	Description string `json:"description"`

	// revisionedSource indicates which configmap this one is revisioned from.
	// If it is nil, then this configmap is not revisioned.
	// Revisioned secrets are the "foo-%d" secrets you see.
	RevisionedSource *InClusterConfigMapLocation `json:"revisionedSource"`

	//CertificateData []PKIRegistryCertKeyMetadata
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
