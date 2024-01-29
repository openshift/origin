package tlsmetadatainterfaces

import (
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/api/annotations"
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
	onDiskCertsWithoutSecrets := certs.CertKeyPairInfoByOnDiskLocation{}
	inClusterCABundles := certs.ConfigMapInfoByNamespaceName{}
	onDiskCABundlesWithoutConfigMaps := certs.CABundleInfoByOnDiskLocation{}

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

		for _, tlsMetadata := range currPKI.OnDiskResourceData.TLSArtifact {
			found := false
			isAlsoInCluster := false
			for _, certKeyPair := range currPKI.CertKeyPairs.Items {
				for _, loc := range certKeyPair.Spec.OnDiskLocations {
					if tlsMetadata.OnDiskLocation.Path == loc.Cert.Path || tlsMetadata.OnDiskLocation.Path == loc.Key.Path {
						found = true
						isAlsoInCluster = len(certKeyPair.Spec.SecretLocations) > 0
						break
					}
				}
				if found {
					break
				}
			}
			if found {
				if !isAlsoInCluster {
					info, err := GetCertKeyPairInfoForOnDiskPath(tlsMetadata.OnDiskLocation)
					if err == nil {
						onDiskCertsWithoutSecrets[tlsMetadata] = info
					}
				}
				continue
			}

			for _, caBundle := range currPKI.CertificateAuthorityBundles.Items {
				for _, loc := range caBundle.Spec.OnDiskLocations {
					if tlsMetadata.OnDiskLocation.Path == loc.Path {
						found = true
						isAlsoInCluster = len(caBundle.Spec.ConfigMapLocations) > 0
						break
					}
				}
				if found {
					break
				}
			}
			if found {
				if !isAlsoInCluster {
					info, err := GetCertificateAuthorityInfoForOnDiskPath(tlsMetadata.OnDiskLocation)
					if err == nil {
						onDiskCABundlesWithoutConfigMaps[tlsMetadata] = info
					}
				}
				continue
			}

		}

	}
	if len(errs) > 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	return certs.CertsToRegistryInfo(inClusterCertKeyPairs, onDiskCertsWithoutSecrets, inClusterCABundles, onDiskCABundlesWithoutConfigMaps), nil
}

// TODO[vrutkovs]: move this to /api?
var (
	onDiskCertificateAuthorities = []certgraphapi.PKIRegistryOnDiskCABundle{
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/kubernetes/ca.crt",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-rw-r--r--",
				SELinuxOptions: "system_u:object_r:kubernetes_file_t:s0",
			},
			CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
				OwningJiraComponent: "Unknown",
			},
		},
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/trusted-ca-bundle/ca-bundle.crt",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-rw-------",
				SELinuxOptions: "system_u:object_r:kubernetes_file_t:s0",
			},
			CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
				OwningJiraComponent: "Unknown",
			},
		},
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/trusted-ca-bundle/ca-bundle.crt",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-rw-------",
				SELinuxOptions: "system_u:object_r:kubernetes_file_t:s0",
			},
			CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
				OwningJiraComponent: "Unknown",
			},
		},
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/pki/tls/cert.pem",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-r--r--r--",
				SELinuxOptions: "system_u:object_r:container_file_t:s0",
			},
			CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
				OwningJiraComponent: "Unknown",
			},
		},
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/pki/tls/certs/ca-bundle.crt",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-r--r--r--",
				SELinuxOptions: "system_u:object_r:container_file_t:s0",
			},
			CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
				OwningJiraComponent: "Unknown",
			},
		},
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/kubernetes/static-pod-resources/kube-controller-manager-certs/secrets/csr-signer/tls.crt",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-rw-------",
				SELinuxOptions: "system_u:object_r:kubernetes_file_t:s0",
			},
			CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
				OwningJiraComponent: "Unknown",
			},
		},
	}
	onDiskCertKeyPairs = []certgraphapi.PKIRegistryOnDiskCertKeyPair{
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/var/lib/ovn-ic/etc/ovnkube-node-certs/ovnkube-client-\u003ctimestamp\u003e.pem",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-rw-------",
				SELinuxOptions: "system_u:object_r:container_var_lib_t:s0",
			},
			CertKeyInfo: certgraphapi.PKIRegistryCertKeyPairInfo{
				OwningJiraComponent: "Unknown",
			},
		},
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/cni/multus/certs/multus-client-\u003ctimestamp\u003e.pem",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-rw-------",
				SELinuxOptions: "system_u:object_r:etc_t:s0",
			},
			CertKeyInfo: certgraphapi.PKIRegistryCertKeyPairInfo{
				OwningJiraComponent: "Unknown",
			},
		},
		{
			OnDiskLocation: certgraphapi.OnDiskLocationWithMetadata{
				OnDiskLocation: certgraphapi.OnDiskLocation{
					Path: "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/bound-service-account-signing-key/service-account.key",
				},
				User:           "root",
				Group:          "root",
				Permissions:    "-rw-------",
				SELinuxOptions: "system_u:object_r:kubernetes_file_t:s0",
			},
			CertKeyInfo: certgraphapi.PKIRegistryCertKeyPairInfo{
				OwningJiraComponent: "Unknown",
			},
		},
	}
)

func GetCertificateAuthorityInfoForOnDiskPath(loc certgraphapi.OnDiskLocation) (certgraphapi.PKIRegistryCertificateAuthorityInfo, error) {
	for _, caBundle := range onDiskCertificateAuthorities {
		if caBundle.OnDiskLocation.Path == loc.Path {
			return caBundle.CABundleInfo, nil
		}
	}
	return certgraphapi.PKIRegistryCertificateAuthorityInfo{}, fmt.Errorf("path %s not found in on disk CA list", loc.Path)
}

func GetExpectedFileMetadataForCABundleOnDisk(loc certgraphapi.OnDiskLocation) (certgraphapi.OnDiskLocationWithMetadata, error) {
	for _, caBundle := range onDiskCertificateAuthorities {
		if caBundle.OnDiskLocation.Path == loc.Path {
			return caBundle.OnDiskLocation, nil
		}
	}
	return certgraphapi.OnDiskLocationWithMetadata{}, fmt.Errorf("path %s not found in on disk CA list", loc.Path)
}

func GetCertKeyPairInfoForOnDiskPath(loc certgraphapi.OnDiskLocation) (certgraphapi.PKIRegistryCertKeyPairInfo, error) {
	for _, certKeyPair := range onDiskCertKeyPairs {
		if certKeyPair.OnDiskLocation.Path == loc.Path {
			return certKeyPair.CertKeyInfo, nil
		}
	}
	return certgraphapi.PKIRegistryCertKeyPairInfo{}, fmt.Errorf("path %s not found in on disk secret list", loc.Path)
}

func GetExpectedFileMetadataForCertKeyPairOnDisk(loc certgraphapi.OnDiskLocation) (certgraphapi.OnDiskLocationWithMetadata, error) {
	for _, certKeyPair := range onDiskCertKeyPairs {
		if certKeyPair.OnDiskLocation.Path == loc.Path {
			return certKeyPair.OnDiskLocation, nil
		}
	}
	return certgraphapi.OnDiskLocationWithMetadata{}, fmt.Errorf("path %s not found in on disk secret list", loc.Path)
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

func GetFileMetadataActualForTLSArtifact(onDisk certgraphapi.PerOnDiskResourceData, loc certgraphapi.OnDiskLocation) (certgraphapi.OnDiskLocationWithMetadata, error) {
	for _, artifact := range onDisk.TLSArtifact {
		if artifact.OnDiskLocation.Path == loc.Path {
			return artifact, nil
		}
	}
	return certgraphapi.OnDiskLocationWithMetadata{}, fmt.Errorf("path %s not found in on disk tls artifact list", loc.Path)
}

func CompareFilePermissions(actual, expected certgraphapi.OnDiskLocationWithMetadata) []string {
	messages := []string{}
	if diff := cmp.Diff(actual.Group, expected.Group); len(diff) > 0 {
		messages = append(messages, fmt.Sprintf("mismatching group for %s: %s", actual.Path, diff))
	}
	if diff := cmp.Diff(actual.Permissions, expected.Permissions); len(diff) > 0 {
		messages = append(messages, fmt.Sprintf("mismatching permissions for %s: %s", actual.Path, diff))
	}
	if diff := cmp.Diff(actual.SELinuxOptions, expected.SELinuxOptions); len(diff) > 0 {
		messages = append(messages, fmt.Sprintf("mismatching SELinux options for %s: %s", actual.Path, diff))
	}
	if diff := cmp.Diff(actual.User, expected.User); len(diff) > 0 {
		messages = append(messages, fmt.Sprintf("mismatching user for %s: %s", actual.Path, diff))
	}
	return messages
}
