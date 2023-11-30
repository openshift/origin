package certs

import (
	"encoding/json"
	"sort"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetPKIInfoFromEmbeddedOwnership(ownershipFile []byte) (*certgraphapi.PKIRegistryInfo, error) {
	certs := SecretInfoByNamespaceName{}
	caBundles := ConfigMapInfoByNamespaceName{}

	currPKI := &certgraphapi.PKIRegistryInfo{}
	err := json.Unmarshal(ownershipFile, currPKI)
	if err != nil {
		return nil, err
	}

	for _, currCert := range currPKI.CertKeyPairs {
		certs[currCert.SecretLocation] = currCert.CertKeyInfo
	}
	for _, currCABundle := range currPKI.CertificateAuthorityBundles {
		caBundles[currCABundle.ConfigMapLocation] = currCABundle.CABundleInfo
	}
	return CertsToRegistryInfo(certs, caBundles), nil
}

func CertsToRegistryInfo(certs SecretInfoByNamespaceName, caBundles ConfigMapInfoByNamespaceName) *certgraphapi.PKIRegistryInfo {
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
	return result
}
