package certgraphapi

import (
	"encoding/json"
	"strings"
)

type PKIList struct {
	// LogicalName is an inexact representation of what this is for.  It may be empty.  It will usually be some hardcoded
	// heuristic trying to find it.
	LogicalName string

	Description string

	InClusterResourceData PerInClusterResourceData
	OnDiskResourceData    PerOnDiskResourceData
	InMemoryResourceData  PerInMemoryResourceData

	CertificateAuthorityBundles CertificateAuthorityBundleList
	CertKeyPairs                CertKeyPairList
}

// PerInClusterResourceData tracks metadata that corresponds to specific secrets and configmaps.
// This data should not duplicate the analysis of the certkeypair lists, but is pulled from annotations on the resources.
// It will be stitched together by a generator after the fact.
type PerInClusterResourceData struct {
	// +mapType:=atomic
	CertificateAuthorityBundles []PKIRegistryInClusterCABundle `json:"certificateAuthorityBundles"`
	// +mapType:=atomic
	CertKeyPairs []PKIRegistryInClusterCertKeyPair `json:"certKeyPairs"`
}

// PerInMemoryResourceData tracks metadata that corresponds to specific certificates stored in pod memory.
type PerInMemoryResourceData struct {
	// +mapType:=atomic
	CertKeyPairs []PKIRegistryInMemoryCertKeyPair `json:"certKeyPairs"`
}

// PerOnDiskResourceData tracks metadata that corresponds to specific files on disk.
// This data should not duplicate the analysis of the certkeypair lists, but is pulled from files on disk.
// It will be stitched together by a generator after the fact.
type PerOnDiskResourceData struct {
	// +mapType:=atomic
	TLSArtifact []OnDiskLocationWithMetadata `json:"tlsArtifact"`
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
	SecretLocations   []InClusterSecretLocation
	OnDiskLocations   []OnDiskCertKeyPairLocation
	InMemoryLocations []InClusterPodLocation

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

type InClusterPodLocation struct {
	Namespace string
	Name      string
}

type OnDiskCertKeyPairLocation struct {
	Cert OnDiskLocation
	Key  OnDiskLocation
}

type OnDiskLocation struct {
	Path string
}

type OnDiskLocationWithMetadata struct {
	OnDiskLocation

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
	CommonName    string
	SerialNumber  string
	PubkeyModulus string

	Issuer *CertIdentifier
}

type CertKeyMetadata struct {
	CertIdentifier     CertIdentifier
	SignatureAlgorithm string
	PublicKeyAlgorithm string
	PublicKeyBitSize   string
	NotBefore          string `json:"-"`
	NotAfter           string `json:"-"`
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

type ConfigMapRefByNamespaceName []InClusterConfigMapLocation
type SecretRefByNamespaceName []InClusterSecretLocation
type SecretInfoByNamespaceName map[InClusterSecretLocation]PKIRegistryCertKeyPairInfo
type ConfigMapInfoByNamespaceName map[InClusterConfigMapLocation]PKIRegistryCertificateAuthorityInfo

func (n SecretRefByNamespaceName) Len() int {
	return len(n)
}
func (n SecretRefByNamespaceName) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n SecretRefByNamespaceName) Less(i, j int) bool {
	diff := strings.Compare(n[i].Namespace, n[j].Namespace)
	switch {
	case diff < 0:
		return true
	case diff > 0:
		return false
	}

	return strings.Compare(n[i].Name, n[j].Name) < 0
}

func (n ConfigMapRefByNamespaceName) Len() int {
	return len(n)
}
func (n ConfigMapRefByNamespaceName) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n ConfigMapRefByNamespaceName) Less(i, j int) bool {
	diff := strings.Compare(n[i].Namespace, n[j].Namespace)
	switch {
	case diff < 0:
		return true
	case diff > 0:
		return false
	}

	return strings.Compare(n[i].Name, n[j].Name) < 0
}
