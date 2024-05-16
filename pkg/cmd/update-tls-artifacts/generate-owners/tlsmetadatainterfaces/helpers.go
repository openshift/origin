package tlsmetadatainterfaces

import (
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/origin/pkg/certs"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const UnknownOwner = "Unknown"

var (
	onDiskCertKeyPairs = certs.CertKeyPairInfoByOnDiskLocation{
		{Path: "/var/lib/ovn-ic/etc/ovnkube-node-certs/ovnkube-client-\u003ctimestamp\u003e.pem"}:                                         {OwningJiraComponent: "Unknown"},
		{Path: "/etc/cni/multus/certs/multus-client-\u003ctimestamp\u003e.pem"}:                                                           {OwningJiraComponent: "Unknown"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/bound-service-account-signing-key/service-account.key"}: {OwningJiraComponent: "Unknown"},
	}
	onDiskCABundles = certs.CABundleInfoByOnDiskLocation{
		{Path: "/etc/kubernetes/ca.crt"}: {OwningJiraComponent: "Unknown"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/trusted-ca-bundle/ca-bundle.crt"}:          {OwningJiraComponent: "Unknown"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/trusted-ca-bundle/ca-bundle.crt"}: {OwningJiraComponent: "Unknown"},
		{Path: "/etc/pki/tls/cert.pem"}:            {OwningJiraComponent: "Unknown"},
		{Path: "/etc/pki/tls/certs/ca-bundle.crt"}: {OwningJiraComponent: "Unknown"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-controller-manager-certs/secrets/csr-signer/tls.crt"}: {OwningJiraComponent: "Unknown"},
	}
)

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
	inClusterCABundles := certs.ConfigMapInfoByNamespaceName{}

	for i := range rawData {
		currPKI := rawData[i]
		for i := range currPKI.InClusterResourceData.CertKeyPairs {
			currCert := currPKI.InClusterResourceData.CertKeyPairs[i]
			existing, ok := inClusterCertKeyPairs[currCert.SecretLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CertKeyInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v secret/%v:\n%v\n", currCert.SecretLocation.Namespace, currCert.SecretLocation.Name, cmp.Diff(existing, currCert.CertKeyInfo)))
				continue
			}

			inClusterCertKeyPairs[currCert.SecretLocation] = currCert.CertKeyInfo
		}

		for i := range currPKI.InClusterResourceData.CertificateAuthorityBundles {
			currCert := currPKI.InClusterResourceData.CertificateAuthorityBundles[i]
			existing, ok := inClusterCABundles[currCert.ConfigMapLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CABundleInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v configmap/%v:\n%v\n", currCert.ConfigMapLocation.Namespace, currCert.ConfigMapLocation.Name, cmp.Diff(existing, currCert.CABundleInfo)))
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
