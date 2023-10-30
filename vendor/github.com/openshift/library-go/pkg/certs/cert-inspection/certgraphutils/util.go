package certgraphutils

import (
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type RegistrySecretByNamespaceName []certgraphapi.PKIRegistryInClusterCertKeyPair

func (n RegistrySecretByNamespaceName) Len() int      { return len(n) }
func (n RegistrySecretByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n RegistrySecretByNamespaceName) Less(i, j int) bool {
	if n[i].SecretLocation.Namespace != n[j].SecretLocation.Namespace {
		return n[i].SecretLocation.Namespace < n[j].SecretLocation.Namespace
	}
	return n[i].SecretLocation.Name < n[j].SecretLocation.Name
}

type RegistryConfigMapByNamespaceName []certgraphapi.PKIRegistryInClusterCABundle

func (n RegistryConfigMapByNamespaceName) Len() int      { return len(n) }
func (n RegistryConfigMapByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n RegistryConfigMapByNamespaceName) Less(i, j int) bool {
	if n[i].ConfigMapLocation.Namespace != n[j].ConfigMapLocation.Namespace {
		return n[i].ConfigMapLocation.Namespace < n[j].ConfigMapLocation.Namespace
	}
	return n[i].ConfigMapLocation.Name < n[j].ConfigMapLocation.Name
}

type SecretLocationByNamespaceName []certgraphapi.InClusterSecretLocation

func (n SecretLocationByNamespaceName) Len() int      { return len(n) }
func (n SecretLocationByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n SecretLocationByNamespaceName) Less(i, j int) bool {
	if n[i].Namespace != n[j].Namespace {
		return n[i].Namespace < n[j].Namespace
	}
	return n[i].Name < n[j].Name
}

type ConfigMapLocationByNamespaceName []certgraphapi.InClusterConfigMapLocation

func (n ConfigMapLocationByNamespaceName) Len() int      { return len(n) }
func (n ConfigMapLocationByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n ConfigMapLocationByNamespaceName) Less(i, j int) bool {
	if n[i].Namespace != n[j].Namespace {
		return n[i].Namespace < n[j].Namespace
	}
	return n[i].Name < n[j].Name
}

func LocateCertKeyPair(targetLocation certgraphapi.InClusterSecretLocation, expectedCertKeyPairs []certgraphapi.PKIRegistryInClusterCertKeyPair) (*certgraphapi.PKIRegistryInClusterCertKeyPair, error) {
	for i, expected := range expectedCertKeyPairs {
		if targetLocation == expected.SecretLocation {
			return &expectedCertKeyPairs[i], nil
		}
	}

	return nil, fmt.Errorf("not found: %#v", targetLocation)
}

func LocateCertificateAuthorityBundle(targetLocation certgraphapi.InClusterConfigMapLocation, expectedCABundles []certgraphapi.PKIRegistryInClusterCABundle) (*certgraphapi.PKIRegistryInClusterCABundle, error) {
	for i, expected := range expectedCABundles {
		if targetLocation == expected.ConfigMapLocation {
			return &expectedCABundles[i], nil
		}
	}

	return nil, fmt.Errorf("not found: %#v", targetLocation)
}
