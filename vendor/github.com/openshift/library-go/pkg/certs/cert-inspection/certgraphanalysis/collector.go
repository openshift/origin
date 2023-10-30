package certgraphanalysis

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/openshift/api/annotations"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
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

func isPlatformNamespace(nsName string) bool {
	if strings.HasPrefix(nsName, "openshift-") {
		return true
	}
	if strings.HasPrefix(nsName, "kubernetes-") {
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
	inClusterResourceData := &certgraphapi.PerInClusterResourceData{}
	certs := []*certgraphapi.CertKeyPair{}
	caBundles := []*certgraphapi.CertificateAuthorityBundle{}
	errs := []error{}
	nodes := map[string]int{}
	nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster nodes: %v", err)
	}
	for i, node := range nodeList.Items {
		nodes[node.Name] = i
	}

	configMapList, err := kubeClient.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	switch {
	case err != nil:
		errs = append(errs, err)
	default:
		for _, configMap := range configMapList.Items {
			if !acceptConfigMap(&configMap) {
				continue
			}

			details, err := InspectConfigMap(&configMap)
			if details != nil {
				caBundles = append(caBundles, details)

				var revisionSource *certgraphapi.InClusterConfigMapLocation
				if metadata, err := meta.Accessor(&configMap); err != nil {
					errs = append(errs, err)
				} else {
					if revisionNamespace, revisionName, revisioned := isRevisioned(metadata); revisioned {
						revisionSource = &certgraphapi.InClusterConfigMapLocation{
							Namespace: revisionNamespace,
							Name:      revisionName,
						}
					}
				}

				inClusterResourceData.CertificateAuthorityBundles = append(inClusterResourceData.CertificateAuthorityBundles,
					certgraphapi.RawPKIRegistryInClusterCABundle{
						ConfigMapLocation: certgraphapi.InClusterConfigMapLocation{
							Namespace: configMap.Namespace,
							Name:      configMap.Name,
						},
						CABundleInfo: certgraphapi.RawPKIRegistryCertificateAuthorityInfo{
							OwningJiraComponent: configMap.Annotations[annotations.OpenShiftComponent],
							Description:         configMap.Annotations[annotations.OpenShiftDescription],
							RevisionedSource:    revisionSource,
						},
					})
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
			if !acceptSecret(&secret) {
				continue
			}
			details, err := InspectSecret(&secret)
			if details != nil {
				certs = append(certs, details)

				var revisionSource *certgraphapi.InClusterSecretLocation
				if metadata, err := meta.Accessor(&secret); err != nil {
					errs = append(errs, err)
				} else {
					if revisionNamespace, revisionName, revisioned := isRevisioned(metadata); revisioned {
						revisionSource = &certgraphapi.InClusterSecretLocation{
							Namespace: revisionNamespace,
							Name:      revisionName,
						}
					}
				}

				inClusterResourceData.CertKeyPairs = append(inClusterResourceData.CertKeyPairs,
					certgraphapi.RawPKIRegistryInClusterCertKeyPair{
						SecretLocation: certgraphapi.InClusterSecretLocation{
							Namespace: secret.Namespace,
							Name:      secret.Name,
						},
						CertKeyInfo: certgraphapi.RawPKIRegistryCertKeyPairInfo{
							OwningJiraComponent: secret.Annotations[annotations.OpenShiftComponent],
							Description:         secret.Annotations[annotations.OpenShiftDescription],
							RevisionedSource:    revisionSource,
						},
					})
			}
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	pkiList := PKIListFromParts(ctx, inClusterResourceData, certs, caBundles, nodes)
	return pkiList, errors.NewAggregate(errs)
}

// returns namespace, name, isRevisioned
func isRevisioned(metadata metav1.Object) (string, string, bool) {
	revisioned := false
	for _, curr := range metadata.GetOwnerReferences() {
		if strings.HasPrefix(curr.Name, "revision-status-") {
			revisioned = true
			break
		}
	}
	if !revisioned {
		return "", "", false
	}
	suffixIndex := strings.LastIndex(metadata.GetName(), "-")
	if suffixIndex < 1 {
		return "", "", false
	}

	return metadata.GetNamespace(), metadata.GetName()[:suffixIndex-1], true
}

func PKIListFromParts(ctx context.Context, inClusterResourceData *certgraphapi.PerInClusterResourceData, certs []*certgraphapi.CertKeyPair, caBundles []*certgraphapi.CertificateAuthorityBundle, nodes map[string]int) *certgraphapi.PKIList {
	certs = deduplicateCertKeyPairs(certs)
	certList := &certgraphapi.CertKeyPairList{}
	for i := range certs {
		certList.Items = append(certList.Items, *certs[i])
	}
	guessLogicalNamesForCertKeyPairList(certList, nodes)

	caBundles = deduplicateCABundles(caBundles)
	caBundleList := &certgraphapi.CertificateAuthorityBundleList{}
	for i := range caBundles {
		caBundleList.Items = append(caBundleList.Items, *caBundles[i])
	}
	guessLogicalNamesForCABundleList(caBundleList)

	ret := &certgraphapi.PKIList{
		CertificateAuthorityBundles: *caBundleList,
		CertKeyPairs:                *certList,
	}
	if inClusterResourceData != nil {
		ret.InClusterResourceData = *inClusterResourceData
	}

	return ret
}
