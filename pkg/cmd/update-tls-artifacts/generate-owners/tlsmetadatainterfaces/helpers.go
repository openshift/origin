package tlsmetadatainterfaces

import (
	"fmt"
	"github.com/openshift/api/annotations"
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

func DescriptionFor(in []certgraphapi.AnnotationValue) string {
	ret, _ := AnnotationValue(in, annotations.OpenShiftDescription)
	return ret
}

func OwnerFor(in []certgraphapi.AnnotationValue) string {
	ret, ok := AnnotationValue(in, annotations.OpenShiftComponent)
	if !ok {
		return "Unknown"
	}
	return ret
}

func ProcessByLocation(rawData []*certgraphapi.PKIList) (*certgraphapi.PKIRegistryInfo, error) {
	errs := []error{}
	certKeyPairs := certs.SecretInfoByNamespaceName{}
	caBundles := certs.ConfigMapInfoByNamespaceName{}
	certificatesOnDiskByPath := map[string]certgraphapi.OnDiskLocationWithMetadata{}
	keysOnDiskByPath := map[string]certgraphapi.OnDiskLocationWithMetadata{}
	caBundlesOnDiskByPath := map[string]certgraphapi.OnDiskLocationWithMetadata{}

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

		for _, currCA := range currPKI.CertificateAuthorityBundles.Items {
			for _, currCALocation := range currCA.Spec.OnDiskLocations {
				found := false
				for i := range currPKI.OnDiskResourceData.TLSArtifact {
					currLocationMetadata := currPKI.OnDiskResourceData.TLSArtifact[i]
					if currCALocation.Path != currLocationMetadata.Path {
						continue
					}
					found = true

					_, ok := caBundlesOnDiskByPath[currLocationMetadata.Path]
					if !ok {
						caBundlesOnDiskByPath[currLocationMetadata.Path] = currLocationMetadata
						break
					}

					// TODO produce a diff that allow different selinux group thingies
					//if !reflect.DeepEqual(existing, currLocationMetadata) {
					//	errs = append(errs, fmt.Errorf("mismatch of pki artifact info for %q: %v", currCALocation.Path, cmp.Diff(existing, currLocationMetadata)))
					//}
				}
				if !found {
					errs = append(errs, fmt.Errorf("could not find metadata for %q", currCALocation.Path))
				}
			}
		}

		for _, currCertKeyPairs := range currPKI.CertKeyPairs.Items {
			// certs
			for _, currCertKeyPairLocation := range currCertKeyPairs.Spec.OnDiskLocations {
				if len(currCertKeyPairLocation.Cert.Path) == 0 {
					// not all on disk locations have both paths
					continue
				}
				found := false
				for i := range currPKI.OnDiskResourceData.TLSArtifact {
					currLocationMetadata := currPKI.OnDiskResourceData.TLSArtifact[i]
					if currCertKeyPairLocation.Cert.Path != currLocationMetadata.Path {
						continue
					}
					found = true

					_, ok := certificatesOnDiskByPath[currLocationMetadata.Path]
					if !ok {
						certificatesOnDiskByPath[currLocationMetadata.Path] = currLocationMetadata
						break
					}

					// TODO produce a diff that allow different selinux group thingies
					//if !reflect.DeepEqual(existing, currLocationMetadata) {
					//	errs = append(errs, fmt.Errorf("mismatch of pki artifact info for %q: %v", currCertKeyPairLocation.Cert.Path, cmp.Diff(existing, currLocationMetadata)))
					//}
				}
				if !found {
					errs = append(errs, fmt.Errorf("could not find metadata for %q", currCertKeyPairLocation.Cert.Path))
				}
			}

			// keys
			for _, currCertKeyPairLocation := range currCertKeyPairs.Spec.OnDiskLocations {
				if len(currCertKeyPairLocation.Key.Path) == 0 {
					// not all on disk locations have both paths
					continue
				}

				found := false
				for i := range currPKI.OnDiskResourceData.TLSArtifact {
					currLocationMetadata := currPKI.OnDiskResourceData.TLSArtifact[i]
					if currCertKeyPairLocation.Key.Path != currLocationMetadata.Path {
						continue
					}
					found = true

					_, ok := keysOnDiskByPath[currLocationMetadata.Path]
					if !ok {
						keysOnDiskByPath[currLocationMetadata.Path] = currLocationMetadata
						break
					}

					// TODO produce a diff that allow different selinux group thingies
					//if !reflect.DeepEqual(existing, currLocationMetadata) {
					//	errs = append(errs, fmt.Errorf("mismatch of pki artifact info for %q: %v", currCertKeyPairLocation.Key.Path, cmp.Diff(existing, currLocationMetadata)))
					//}
				}
				if !found {
					errs = append(errs, fmt.Errorf("could not find metadata for %q", currCertKeyPairLocation.Key.Path))
				}
			}
		}

	}
	if len(errs) > 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	return certs.CertsToRegistryInfo(certKeyPairs, caBundles, certificatesOnDiskByPath, keysOnDiskByPath, caBundlesOnDiskByPath), nil
}
