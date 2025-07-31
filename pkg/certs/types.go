package certs

import (
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type ConfigMapRefByNamespaceName []certgraphapi.InClusterConfigMapLocation
type SecretRefByNamespaceName []certgraphapi.InClusterSecretLocation
type SecretInfoByNamespaceName map[certgraphapi.InClusterSecretLocation]certgraphapi.PKIRegistryCertKeyPairInfo
type ConfigMapInfoByNamespaceName map[certgraphapi.InClusterConfigMapLocation]certgraphapi.PKIRegistryCertificateAuthorityInfo
type PodRefByNamespaceName []certgraphapi.InClusterPodLocation
type PodInfoByNamespaceName map[certgraphapi.InClusterPodLocation]certgraphapi.PKIRegistryCertKeyPairInfo
type OnDiskLocationByPath []certgraphapi.OnDiskLocation
type CertKeyPairInfoByOnDiskLocation map[certgraphapi.OnDiskLocation]certgraphapi.PKIRegistryCertKeyPairInfo
type CABundleInfoByOnDiskLocation map[certgraphapi.OnDiskLocation]certgraphapi.PKIRegistryCertificateAuthorityInfo

type CertKeyPairByLocation []certgraphapi.PKIRegistryCertKeyPair
type CertificateAuthorityBundleByLocation []certgraphapi.PKIRegistryCABundle

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

func (n PodRefByNamespaceName) Len() int {
	return len(n)
}
func (n PodRefByNamespaceName) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n PodRefByNamespaceName) Less(i, j int) bool {
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

func BuildCertKeyPath(curr certgraphapi.PKIRegistryCertKeyPair) string {
	if curr.InClusterLocation != nil {
		return fmt.Sprintf("ns/%v secret/%v", curr.InClusterLocation.SecretLocation.Namespace, curr.InClusterLocation.SecretLocation.Name)
	}
	if curr.InMemoryPodLocation != nil {
		return fmt.Sprintf("ns/%v pod/%v (in-memory)", curr.InMemoryPodLocation.PodLocation.Namespace, curr.InMemoryPodLocation.PodLocation.Name)
	}
	if curr.OnDiskLocation != nil {
		return fmt.Sprintf("file %v", curr.OnDiskLocation.OnDiskLocation.Path)
	}
	return ""
}

func BuildCABundlePath(curr certgraphapi.PKIRegistryCABundle) string {
	if curr.InClusterLocation != nil {
		return fmt.Sprintf("ns/%v configmap/%v", curr.InClusterLocation.ConfigMapLocation.Namespace, curr.InClusterLocation.ConfigMapLocation.Name)
	}
	if curr.OnDiskLocation != nil {
		return fmt.Sprintf("file %v", curr.OnDiskLocation.OnDiskLocation.Path)
	}
	return ""
}

func (n CertKeyPairByLocation) Len() int {
	return len(n)
}
func (n CertKeyPairByLocation) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n CertKeyPairByLocation) Less(i, j int) bool {
	return strings.Compare(BuildCertKeyPath(n[i]), BuildCertKeyPath(n[j])) < 0
}

func (n CertificateAuthorityBundleByLocation) Len() int {
	return len(n)
}
func (n CertificateAuthorityBundleByLocation) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n CertificateAuthorityBundleByLocation) Less(i, j int) bool {
	return strings.Compare(BuildCABundlePath(n[i]), BuildCABundlePath(n[j])) < 0
}
