package certs

import (
	"encoding/json"
	"sort"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

// PKIRegistryInfo holds information about TLS artifacts stored in etcd. This includes object location and metadata based on object annotations
type PKIRegistryInfo struct {
	// +mapType:=atomic
	CertificateAuthorityBundles []certgraphapi.PKIRegistryCABundle `json:"certificateAuthorityBundles"`
	// +mapType:=atomic
	CertKeyPairs []certgraphapi.PKIRegistryCertKeyPair `json:"certKeyPairs"`
}

func GetPKIInfoFromEmbeddedOwnership(ownershipFile []byte) (*PKIRegistryInfo, error) {
	inClusterCerts := SecretInfoByNamespaceName{}
	onDiskCerts := CertKeyPairInfoByOnDiskLocation{}
	inClusterCABundles := ConfigMapInfoByNamespaceName{}
	onDiskCABundles := CABundleInfoByOnDiskLocation{}
	inMemoryCerts := PodInfoByNamespaceName{}

	currPKI := &PKIRegistryInfo{}
	err := json.Unmarshal(ownershipFile, currPKI)
	if err != nil {
		return nil, err
	}

	for _, currCert := range currPKI.CertKeyPairs {
		if currCert.InClusterLocation != nil {
			inClusterCerts[currCert.InClusterLocation.SecretLocation] = currCert.InClusterLocation.CertKeyInfo
		}
		if currCert.OnDiskLocation != nil {
			onDiskCerts[currCert.OnDiskLocation.OnDiskLocation] = currCert.OnDiskLocation.CertKeyInfo
		}
		if currCert.InMemoryPodLocation != nil {
			inMemoryCerts[currCert.InMemoryPodLocation.PodLocation] = currCert.InMemoryPodLocation.CertKeyInfo
		}
	}
	for _, currCABundle := range currPKI.CertificateAuthorityBundles {
		if currCABundle.InClusterLocation != nil {
			inClusterCABundles[currCABundle.InClusterLocation.ConfigMapLocation] = currCABundle.InClusterLocation.CABundleInfo
		}
		if currCABundle.OnDiskLocation != nil {
			onDiskCABundles[currCABundle.OnDiskLocation.OnDiskLocation] = currCABundle.OnDiskLocation.CABundleInfo
		}
	}
	return CertsToRegistryInfo(inClusterCerts, onDiskCerts, inClusterCABundles, onDiskCABundles, inMemoryCerts), nil
}

func CertsToRegistryInfo(
	certs SecretInfoByNamespaceName,
	onDiskCerts CertKeyPairInfoByOnDiskLocation,
	caBundles ConfigMapInfoByNamespaceName,
	onDiskCABundles CABundleInfoByOnDiskLocation,
	inMemoryCerts PodInfoByNamespaceName,
) *PKIRegistryInfo {
	result := &PKIRegistryInfo{}

	inClusterCertKeys := sets.KeySet[certgraphapi.InClusterSecretLocation, certgraphapi.PKIRegistryCertKeyPairInfo](certs).UnsortedList()
	sort.Sort(SecretRefByNamespaceName(inClusterCertKeys))
	for _, key := range inClusterCertKeys {
		result.CertKeyPairs = append(result.CertKeyPairs, certgraphapi.PKIRegistryCertKeyPair{
			InClusterLocation: &certgraphapi.PKIRegistryInClusterCertKeyPair{
				SecretLocation: key,
				CertKeyInfo:    certs[key],
			},
		})
	}
	onDiskCertKeys := sets.KeySet[certgraphapi.OnDiskLocation, certgraphapi.PKIRegistryCertKeyPairInfo](onDiskCerts).UnsortedList()
	sort.Sort(OnDiskLocationByPath(onDiskCertKeys))
	for _, key := range onDiskCertKeys {
		result.CertKeyPairs = append(result.CertKeyPairs, certgraphapi.PKIRegistryCertKeyPair{
			OnDiskLocation: &certgraphapi.PKIRegistryOnDiskCertKeyPair{
				OnDiskLocation: key,
				CertKeyInfo:    onDiskCerts[key],
			},
		})
	}
	inMemoryCertKeys := sets.KeySet[certgraphapi.InClusterPodLocation, certgraphapi.PKIRegistryCertKeyPairInfo](inMemoryCerts).UnsortedList()
	sort.Sort(PodRefByNamespaceName(inMemoryCertKeys))
	for _, key := range inMemoryCertKeys {
		result.CertKeyPairs = append(result.CertKeyPairs, certgraphapi.PKIRegistryCertKeyPair{
			InMemoryPodLocation: &certgraphapi.PKIRegistryInMemoryCertKeyPair{
				PodLocation: key,
				CertKeyInfo: inMemoryCerts[key],
			},
		})
	}

	inClusterCAKeys := sets.KeySet[certgraphapi.InClusterConfigMapLocation, certgraphapi.PKIRegistryCertificateAuthorityInfo](caBundles).UnsortedList()
	sort.Sort(ConfigMapRefByNamespaceName(inClusterCAKeys))
	for _, key := range inClusterCAKeys {
		result.CertificateAuthorityBundles = append(result.CertificateAuthorityBundles, certgraphapi.PKIRegistryCABundle{
			InClusterLocation: &certgraphapi.PKIRegistryInClusterCABundle{
				ConfigMapLocation: key,
				CABundleInfo:      caBundles[key],
			},
		})
	}
	onDiskCAKeys := sets.KeySet[certgraphapi.OnDiskLocation, certgraphapi.PKIRegistryCertificateAuthorityInfo](onDiskCABundles).UnsortedList()
	sort.Sort(OnDiskLocationByPath(onDiskCAKeys))
	for _, key := range onDiskCAKeys {
		result.CertificateAuthorityBundles = append(result.CertificateAuthorityBundles, certgraphapi.PKIRegistryCABundle{
			OnDiskLocation: &certgraphapi.PKIRegistryOnDiskCABundle{
				OnDiskLocation: key,
				CABundleInfo:   onDiskCABundles[key],
			},
		})
	}
	return result
}
