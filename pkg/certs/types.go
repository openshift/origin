package certs

import (
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

// PKIRegistryInfo holds information about TLS artifacts stored in etcd. This includes object location and metadata based on object annotations
type PKIRegistryInfo struct {
	// +mapType:=atomic
	CertificateAuthorityBundles []certgraphapi.PKIRegistryCABundle `json:"certificateAuthorityBundles"`
	// +mapType:=atomic
	CertKeyPairs []certgraphapi.PKIRegistryCertKeyPair `json:"certKeyPairs"`
}

type ConfigMapRefByNamespaceName []certgraphapi.InClusterConfigMapLocation
type SecretRefByNamespaceName []certgraphapi.InClusterSecretLocation
type SecretInfoByNamespaceName map[certgraphapi.InClusterSecretLocation]certgraphapi.PKIRegistryCertKeyPairInfo
type ConfigMapInfoByNamespaceName map[certgraphapi.InClusterConfigMapLocation]certgraphapi.PKIRegistryCertificateAuthorityInfo
type OnDiskLocationByPath []certgraphapi.OnDiskLocationWithMetadata
type CertKeyPairInfoByOnDiskLocation map[certgraphapi.OnDiskLocationWithMetadata]certgraphapi.PKIRegistryCertKeyPairInfo
type CABundleInfoByOnDiskLocation map[certgraphapi.OnDiskLocationWithMetadata]certgraphapi.PKIRegistryCertificateAuthorityInfo

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

func (n OnDiskLocationByPath) Len() int {
	return len(n)
}
func (n OnDiskLocationByPath) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n OnDiskLocationByPath) Less(i, j int) bool {
	return strings.Compare(n[i].Path, n[j].Path) < 0
}
