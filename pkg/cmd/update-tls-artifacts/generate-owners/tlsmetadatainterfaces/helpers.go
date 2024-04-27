package tlsmetadatainterfaces

import (
	"fmt"
	"reflect"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/origin/pkg/certs"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const UnknownOwner = "Unknown"

func AnnotationValue(whitelistedAnnotations []certgraphapi.AnnotationValue, key string) (string, bool) {
	for _, curr := range whitelistedAnnotations {
		if curr.Key == key {
			return curr.Value, true
		}
	}

	return "", false
}

func ProcessByLocation(rawData []*certgraphapi.PKIList) (*certs.PKIRegistryInfo, error) {
	errs := []error{}
	inClusterCertKeyPairs := certs.SecretInfoByNamespaceName{}
	onDiskCertKeyPairs := certs.CertKeyPairInfoByOnDiskLocation{}
	inClusterCABundles := certs.ConfigMapInfoByNamespaceName{}
	onDiskCABundles := certs.CABundleInfoByOnDiskLocation{}
	// TODO[vrutkovs]: fill them up

	for i := range rawData {
		currPKI := rawData[i]

		for i := range currPKI.InClusterResourceData.CertKeyPairs {
			currCert := currPKI.InClusterResourceData.CertKeyPairs[i]
			existing, ok := inClusterCertKeyPairs[currCert.SecretLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CertKeyInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v secret/%v", currCert.SecretLocation.Namespace, currCert.SecretLocation.Name))
				continue
			}

			inClusterCertKeyPairs[currCert.SecretLocation] = currCert.CertKeyInfo
		}
		for i := range currPKI.InClusterResourceData.CertificateAuthorityBundles {
			currCert := currPKI.InClusterResourceData.CertificateAuthorityBundles[i]
			existing, ok := inClusterCABundles[currCert.ConfigMapLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CABundleInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v configmap/%v", currCert.ConfigMapLocation.Namespace, currCert.ConfigMapLocation.Name))
				continue
			}

			inClusterCABundles[currCert.ConfigMapLocation] = currCert.CABundleInfo
		}
	}
	if len(errs) > 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	return certs.CertsToRegistryInfo(inClusterCertKeyPairs, onDiskCertKeyPairs, inClusterCABundles, onDiskCABundles), nil
}
