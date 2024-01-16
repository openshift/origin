package certs

import (
	"encoding/json"
	"fmt"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetPKIInfoFromEmbeddedOwnership(ownershipFile []byte) (*certgraphapi.PKIRegistryInfo, error) {
	certs := certgraphapi.SecretInfoByNamespaceName{}
	caBundles := certgraphapi.ConfigMapInfoByNamespaceName{}

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

func CertsToRegistryInfo(certs certgraphapi.SecretInfoByNamespaceName, caBundles certgraphapi.ConfigMapInfoByNamespaceName) *certgraphapi.PKIRegistryInfo {
	result := &certgraphapi.PKIRegistryInfo{}

	certKeys := sets.KeySet[certgraphapi.InClusterSecretLocation, certgraphapi.PKIRegistryCertKeyPairInfo](certs).UnsortedList()
	sort.Sort(certgraphapi.SecretRefByNamespaceName(certKeys))
	for _, key := range certKeys {
		result.CertKeyPairs = append(result.CertKeyPairs, certgraphapi.PKIRegistryInClusterCertKeyPair{
			SecretLocation: key,
			CertKeyInfo:    certs[key],
		})
	}

	caKeys := sets.KeySet[certgraphapi.InClusterConfigMapLocation, certgraphapi.PKIRegistryCertificateAuthorityInfo](caBundles).UnsortedList()
	sort.Sort(certgraphapi.ConfigMapRefByNamespaceName(caKeys))
	for _, key := range caKeys {
		result.CertificateAuthorityBundles = append(result.CertificateAuthorityBundles, certgraphapi.PKIRegistryInClusterCABundle{
			ConfigMapLocation: key,
			CABundleInfo:      caBundles[key],
		})
	}
	return result
}

func GetRawDataFromDir(rawDataDir string) ([]*certgraphapi.PKIList, error) {
	ret := []*certgraphapi.PKIList{}

	err := filepath.WalkDir(rawDataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		filename := filepath.Join(rawDataDir, d.Name())
		currBytes, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		currPKI := &certgraphapi.PKIList{}
		err = json.Unmarshal(currBytes, currPKI)
		if err != nil {
			return err
		}
		ret = append(ret, currPKI)

		return nil
	})
	if err != nil {
		return nil, err
	}

	// verification that our raw data is consistent
	if _, err := ProcessByLocation(ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func ProcessByLocation(rawData []*certgraphapi.PKIList) (*certgraphapi.PKIRegistryInfo, error) {
	errs := []error{}
	certKeyPairs := certgraphapi.SecretInfoByNamespaceName{}
	caBundles := certgraphapi.ConfigMapInfoByNamespaceName{}

	for i := range rawData {
		currPKI := rawData[i]

		for i := range currPKI.InClusterResourceData.CertKeyPairs {
			currCert := currPKI.InClusterResourceData.CertKeyPairs[i]
			existing, ok := certKeyPairs[currCert.SecretLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CertKeyInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v secret/%v", currCert.SecretLocation.Namespace, currCert.SecretLocation.Name))
				continue
			}

			certKeyPairs[currCert.SecretLocation] = currCert.CertKeyInfo
		}
		for i := range currPKI.InClusterResourceData.CertificateAuthorityBundles {
			currCert := currPKI.InClusterResourceData.CertificateAuthorityBundles[i]
			existing, ok := caBundles[currCert.ConfigMapLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CABundleInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v configmap/%v", currCert.ConfigMapLocation.Namespace, currCert.ConfigMapLocation.Name))
				continue
			}

			caBundles[currCert.ConfigMapLocation] = currCert.CABundleInfo
		}
	}
	if len(errs) > 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	return CertsToRegistryInfo(certKeyPairs, caBundles), nil
}
