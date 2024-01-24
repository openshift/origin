package certs

import (
	"encoding/json"
	"sort"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetPKIInfoFromEmbeddedOwnership(ownershipFile []byte) (*certgraphapi.PKIRegistryInfo, error) {
	currPKI := &certgraphapi.PKIRegistryInfo{}
	err := json.Unmarshal(ownershipFile, currPKI)
	if err != nil {
		return nil, err
	}

	return currPKI, nil
}

func CertsToRegistryInfo(
	certs SecretInfoByNamespaceName,
	caBundles ConfigMapInfoByNamespaceName,
	certificatesOnDiskByPath map[string]certgraphapi.OnDiskLocationWithMetadata,
	keysOnDiskByPath map[string]certgraphapi.OnDiskLocationWithMetadata,
	caBundlesOnDiskByPath map[string]certgraphapi.OnDiskLocationWithMetadata,
) *certgraphapi.PKIRegistryInfo {
	result := &certgraphapi.PKIRegistryInfo{}

	certKeys := sets.KeySet[certgraphapi.InClusterSecretLocation, certgraphapi.PKIRegistryCertKeyPairInfo](certs).UnsortedList()
	sort.Sort(SecretRefByNamespaceName(certKeys))
	for _, key := range certKeys {
		result.CertKeyPairs = append(result.CertKeyPairs, certgraphapi.PKIRegistryInClusterCertKeyPair{
			SecretLocation: key,
			CertKeyInfo:    certs[key],
		})
	}

	caKeys := sets.KeySet[certgraphapi.InClusterConfigMapLocation, certgraphapi.PKIRegistryCertificateAuthorityInfo](caBundles).UnsortedList()
	sort.Sort(ConfigMapRefByNamespaceName(caKeys))
	for _, key := range caKeys {
		result.CertificateAuthorityBundles = append(result.CertificateAuthorityBundles, certgraphapi.PKIRegistryInClusterCABundle{
			ConfigMapLocation: key,
			CABundleInfo:      caBundles[key],
		})
	}

	for _, path := range sets.List(sets.KeySet(certificatesOnDiskByPath)) {
		result.CertificatesOnDisk = append(result.CertificatesOnDisk, certificatesOnDiskByPath[path])
	}
	for _, path := range sets.List(sets.KeySet(keysOnDiskByPath)) {
		result.KeysOnDisk = append(result.KeysOnDisk, keysOnDiskByPath[path])
	}
	for _, path := range sets.List(sets.KeySet(caBundlesOnDiskByPath)) {
		result.CertificateAuthorityBundlesOnDisk = append(result.CertificateAuthorityBundlesOnDisk, caBundlesOnDiskByPath[path])
	}

	return result
}
