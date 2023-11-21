package certs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetPKIInfoFromRawData(rawTLSInfoDir string) (*certgraphapi.PKIRegistryInfo, error) {
	certs := SecretInfoByNamespaceName{}
	caBundles := ConfigMapInfoByNamespaceName{}

	err := filepath.WalkDir(rawTLSInfoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		filename := filepath.Join(rawTLSInfoDir, d.Name())
		currBytes, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		currPKI := &certgraphapi.PKIList{}
		err = json.Unmarshal(currBytes, currPKI)
		if err != nil {
			return err
		}

		for i := range currPKI.InClusterResourceData.CertKeyPairs {
			currCert := currPKI.InClusterResourceData.CertKeyPairs[i]
			existing, ok := certs[currCert.SecretLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CertKeyInfo) {
				return fmt.Errorf("mismatch of certificate info, expected\n%v\n to be equal\n%v", existing, currCert.CertKeyInfo)
			}

			certs[currCert.SecretLocation] = currCert.CertKeyInfo
		}
		for i := range currPKI.InClusterResourceData.CertificateAuthorityBundles {
			currCert := currPKI.InClusterResourceData.CertificateAuthorityBundles[i]
			existing, ok := caBundles[currCert.ConfigMapLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CABundleInfo) {
				return fmt.Errorf("mismatch of ca bundle info")
			}

			caBundles[currCert.ConfigMapLocation] = currCert.CABundleInfo
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return certsToRegistryInfo(certs, caBundles), nil
}

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
	return certsToRegistryInfo(certs, caBundles), nil
}

func certsToRegistryInfo(certs SecretInfoByNamespaceName, caBundles ConfigMapInfoByNamespaceName) *certgraphapi.PKIRegistryInfo {
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
