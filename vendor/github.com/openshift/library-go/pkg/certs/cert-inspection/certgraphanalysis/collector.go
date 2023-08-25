package certgraphanalysis

import (
	"context"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
)

func GatherCertsFromAllNamespaces(ctx context.Context, kubeClient kubernetes.Interface) (*certgraphapi.PKIList, error) {
	certs := []*certgraphapi.CertKeyPair{}
	caBundles := []*certgraphapi.CertificateAuthorityBundle{}
	errs := []error{}

	configMapList, err := kubeClient.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	switch {
	case err != nil:
		errs = append(errs, err)
	default:
		for _, configMap := range configMapList.Items {
			details, err := InspectConfigMap(&configMap)
			if details != nil {
				caBundles = append(caBundles, details)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	secretList, err := kubeClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	switch {
	case err != nil:
		errs = append(errs, err)
	default:
		for _, secret := range secretList.Items {
			details, err := InspectSecret(&secret)
			if details != nil {
				certs = append(certs, details)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	pkiList := PKIListFromParts(ctx, certs, caBundles)
	return pkiList, errors.NewAggregate(errs)
}

func PKIListFromParts(ctx context.Context, certs []*certgraphapi.CertKeyPair, caBundles []*certgraphapi.CertificateAuthorityBundle) *certgraphapi.PKIList {
	certs = deduplicateCertKeyPairs(certs)
	certList := &certgraphapi.CertKeyPairList{}
	for i := range certs {
		certList.Items = append(certList.Items, *certs[i])
	}
	guessLogicalNamesForCertKeyPairList(certList)

	caBundles = deduplicateCABundles(caBundles)
	caBundleList := &certgraphapi.CertificateAuthorityBundleList{}
	for i := range caBundles {
		caBundleList.Items = append(caBundleList.Items, *caBundles[i])
	}
	guessLogicalNamesForCABundleList(caBundleList)

	return &certgraphapi.PKIList{
		CertificateAuthorityBundles: *caBundleList,
		CertKeyPairs:                *certList,
	}
}
