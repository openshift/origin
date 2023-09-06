package certgraphanalysis

import (
	"context"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
)

func GatherCertsFromAllNamespaces(ctx context.Context, kubeClient kubernetes.Interface) (*certgraphapi.PKIList, error) {
	return gatherFilteredCerts(ctx, kubeClient, allConfigMaps, allSecrets)
}

func GatherCertsFromPlatformNamespaces(ctx context.Context, kubeClient kubernetes.Interface) (*certgraphapi.PKIList, error) {
	return gatherFilteredCerts(ctx, kubeClient, platformConfigMaps, platformSecrets)
}

var wellKnownPlatformNamespaces = sets.NewString(
	"openshift",
	"default",
	"kube-system",
	"kube-public",
	"kubernetes",
	)

func isPlatformNamespace(nsName string) bool{
	if strings.HasPrefix(nsName, "openshift-"){
		return true
	}
	if strings.HasPrefix(nsName, "kubernetes-"){
		return true
	}
	return wellKnownPlatformNamespaces.Has(nsName)
}

type configMapFilterFunc func(configMap *corev1.ConfigMap) bool

func allConfigMaps(_ *corev1.ConfigMap) bool {
	return true
}
func platformConfigMaps(obj *corev1.ConfigMap) bool {
	return isPlatformNamespace(obj.Namespace)
}

type secretFilterFunc func(configMap *corev1.Secret) bool

func allSecrets(_ *corev1.Secret) bool {
	return true
}
func platformSecrets(obj *corev1.Secret) bool {
	return isPlatformNamespace(obj.Namespace)
}

func gatherFilteredCerts(ctx context.Context, kubeClient kubernetes.Interface, acceptConfigMap configMapFilterFunc, acceptSecret secretFilterFunc) (*certgraphapi.PKIList, error) {
	certs := []*certgraphapi.CertKeyPair{}
	caBundles := []*certgraphapi.CertificateAuthorityBundle{}
	errs := []error{}

	configMapList, err := kubeClient.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	switch {
	case err != nil:
		errs = append(errs, err)
	default:
		for _, configMap := range configMapList.Items {
			if !acceptConfigMap(&configMap){
				continue
			}
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
			if !acceptSecret(&secret){
				continue
			}
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
