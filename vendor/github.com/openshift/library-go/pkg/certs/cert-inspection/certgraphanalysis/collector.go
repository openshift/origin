package certgraphanalysis

import (
	"context"
	"strings"

	"github.com/openshift/api/annotations"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
)

func GatherCertsFromAllNamespaces(ctx context.Context, kubeClient kubernetes.Interface, options ...certGenerationOptions) (*certgraphapi.PKIList, error) {
	return gatherFilteredCerts(ctx, kubeClient, allConfigMaps, allSecrets, options)
}

func GatherCertsFromPlatformNamespaces(ctx context.Context, kubeClient kubernetes.Interface, options ...certGenerationOptions) (*certgraphapi.PKIList, error) {
	return gatherFilteredCerts(ctx, kubeClient, platformConfigMaps, platformSecrets, options)
}

func GatherCertsFromDisk(ctx context.Context, kubeClient kubernetes.Interface, dir string, options ...certGenerationOptions) (*certgraphapi.PKIList, error) {
	errs := []error{}

	certs, certMetadata, err := gatherSecretsFromDisk(ctx, dir, options)
	if err != nil {
		errs = append(errs, err)
	}
	caBundles, caBundlesMetadata, err := gatherCABundlesFromDisk(ctx, dir, options)
	if err != nil {
		errs = append(errs, err)
	}
	pkiList := PKIListFromParts(ctx, nil, certs, caBundles)

	fileMetadata := []certgraphapi.OnDiskLocationWithMetadata{}
	for _, metadata := range certMetadata {
		fileMetadata = append(fileMetadata, *metadata)
	}
	for _, metadata := range caBundlesMetadata {
		fileMetadata = append(fileMetadata, *metadata)
	}
	pkiList.OnDiskResourceData = deduplicateOnDiskMetadata(certgraphapi.PerOnDiskResourceData{
		TLSArtifact: fileMetadata,
	})
	return pkiList, utilerrors.NewAggregate(errs)
}

var wellKnownPlatformNamespaces = sets.New(
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

func allConfigMaps(_ *corev1.ConfigMap) bool {
	return true
}

func platformConfigMaps(obj *corev1.ConfigMap) bool {
	return isPlatformNamespace(obj.Namespace)
}

func allSecrets(_ *corev1.Secret) bool {
	return true
}
func platformSecrets(obj *corev1.Secret) bool {
	return isPlatformNamespace(obj.Namespace)
}

func gatherFilteredCerts(ctx context.Context, kubeClient kubernetes.Interface, acceptConfigMap configMapFilterFunc, acceptSecret secretFilterFunc, options certGenerationOptionList) (*certgraphapi.PKIList, error) {
	inClusterResourceData := &certgraphapi.PerInClusterResourceData{}
	certs := []*certgraphapi.CertKeyPair{}
	caBundles := []*certgraphapi.CertificateAuthorityBundle{}
	errs := []error{}

	// TODO here is the point where need to collect data like node names and IPs that need to be replaced
	//  this will be something like options.Discovery(kubeClient, configClient).

	annotationsToCollect := options.annotationsToCollect()

	configMapList, err := kubeClient.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	switch {
	case err != nil:
		errs = append(errs, err)
	default:
		for _, configMap := range configMapList.Items {
			options.rewriteConfigMap(&configMap)
			if !acceptConfigMap(&configMap) {
				continue
			}
			if options.rejectConfigMap(&configMap) {
				continue
			}
			details, err := InspectConfigMap(&configMap)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if details == nil {
				continue
			}
			options.rewriteCABundle(configMap.ObjectMeta, details)
			caBundles = append(caBundles, details)

			inClusterResourceData.CertificateAuthorityBundles = append(inClusterResourceData.CertificateAuthorityBundles,
				certgraphapi.PKIRegistryInClusterCABundle{
					ConfigMapLocation: certgraphapi.InClusterConfigMapLocation{
						Namespace: configMap.Namespace,
						Name:      configMap.Name,
					},
					CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
						SelectedCertMetadataAnnotations: recordedAnnotationsFrom(configMap.ObjectMeta, annotationsToCollect),
						OwningJiraComponent:             configMap.Annotations[annotations.OpenShiftComponent],
						Description:                     configMap.Annotations[annotations.OpenShiftDescription],
					},
				})
		}
	}

	secretList, err := kubeClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	switch {
	case err != nil:
		errs = append(errs, err)
	default:
		for _, secret := range secretList.Items {
			options.rewriteSecret(&secret)
			if !acceptSecret(&secret) {
				continue
			}
			if options.rejectSecret(&secret) {
				continue
			}
			details, err := InspectSecret(&secret)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if len(details) == 0 {
				continue
			}
			for i := range details {
				options.rewriteCertKeyPair(secret.ObjectMeta, details[i])
			}

			certs = append(certs, details...)
			inClusterResourceData.CertKeyPairs = append(inClusterResourceData.CertKeyPairs,
				certgraphapi.PKIRegistryInClusterCertKeyPair{
					SecretLocation: certgraphapi.InClusterSecretLocation{
						Namespace: secret.Namespace,
						Name:      secret.Name,
					},
					CertKeyInfo: certgraphapi.PKIRegistryCertKeyPairInfo{
						SelectedCertMetadataAnnotations: recordedAnnotationsFrom(secret.ObjectMeta, annotationsToCollect),
						OwningJiraComponent:             secret.Annotations[annotations.OpenShiftComponent],
						Description:                     secret.Annotations[annotations.OpenShiftDescription],
					},
				})
		}
	}

	pkiList := PKIListFromParts(ctx, inClusterResourceData, certs, caBundles)
	return pkiList, utilerrors.NewAggregate(errs)
}

func recordedAnnotationsFrom(metadata metav1.ObjectMeta, annotationsToCollect []string) []certgraphapi.AnnotationValue {
	ret := []certgraphapi.AnnotationValue{}
	for _, key := range annotationsToCollect {
		val, ok := metadata.Annotations[key]
		if !ok {
			continue
		}
		ret = append(ret, certgraphapi.AnnotationValue{
			Key:   key,
			Value: val,
		})
	}

	return ret
}

func PKIListFromParts(ctx context.Context, inClusterResourceData *certgraphapi.PerInClusterResourceData, certs []*certgraphapi.CertKeyPair, caBundles []*certgraphapi.CertificateAuthorityBundle) *certgraphapi.PKIList {
	certs = deduplicateCertKeyPairs(certs)
	onDiskResourceData := certgraphapi.PerOnDiskResourceData{}
	certList := &certgraphapi.CertKeyPairList{}
	for i := range certs {
		certList.Items = append(certList.Items, *certs[i])
	}

	caBundles = deduplicateCABundles(caBundles)
	caBundleList := &certgraphapi.CertificateAuthorityBundleList{}
	for i := range caBundles {
		caBundleList.Items = append(caBundleList.Items, *caBundles[i])
	}

	onDiskResourceData = deduplicateOnDiskMetadata(onDiskResourceData)

	ret := &certgraphapi.PKIList{
		CertificateAuthorityBundles: *caBundleList,
		CertKeyPairs:                *certList,
		OnDiskResourceData:          onDiskResourceData,
	}

	if inClusterResourceData != nil {
		ret.InClusterResourceData = *inClusterResourceData
	}

	return ret
}

func MergePKILists(ctx context.Context, first, second *certgraphapi.PKIList) *certgraphapi.PKIList {

	if first == nil {
		first = &certgraphapi.PKIList{}
	}

	if second == nil {
		second = &certgraphapi.PKIList{}
	}

	certList := &certgraphapi.CertKeyPairList{
		Items: append(first.CertKeyPairs.Items, second.CertKeyPairs.Items...),
	}
	certList = deduplicateCertKeyPairList(certList)

	caBundlesList := &certgraphapi.CertificateAuthorityBundleList{
		Items: append(first.CertificateAuthorityBundles.Items, second.CertificateAuthorityBundles.Items...),
	}
	caBundlesList = deduplicateCABundlesList(caBundlesList)

	inClusterData := certgraphapi.PerInClusterResourceData{}
	inClusterData.CertKeyPairs = append(inClusterData.CertKeyPairs, first.InClusterResourceData.CertKeyPairs...)
	inClusterData.CertKeyPairs = append(inClusterData.CertKeyPairs, second.InClusterResourceData.CertKeyPairs...)
	inClusterData.CertificateAuthorityBundles = append(inClusterData.CertificateAuthorityBundles, first.InClusterResourceData.CertificateAuthorityBundles...)
	inClusterData.CertificateAuthorityBundles = append(inClusterData.CertificateAuthorityBundles, second.InClusterResourceData.CertificateAuthorityBundles...)

	onDiskResourceData := certgraphapi.PerOnDiskResourceData{
		TLSArtifact: append(first.OnDiskResourceData.TLSArtifact, second.OnDiskResourceData.TLSArtifact...),
	}
	onDiskResourceData = deduplicateOnDiskMetadata(onDiskResourceData)

	return &certgraphapi.PKIList{
		CertificateAuthorityBundles: *caBundlesList,
		CertKeyPairs:                *certList,
		InClusterResourceData:       inClusterData,
		OnDiskResourceData:          onDiskResourceData,
	}
}

// GetBootstrapIPAndHostname finds bootstrap IP and hostname in openshift-etcd namespace
// configmaps and secrets
// Either IP or hostname may be empty
func GetBootstrapIPAndHostname(ctx context.Context, kubeClient kubernetes.Interface) (string, string, error) {
	bootstrapIP := ""
	etcdConfigMaps, err := kubeClient.CoreV1().ConfigMaps("openshift-etcd").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}
	for _, cm := range etcdConfigMaps.Items {
		annotation, ok := cm.Annotations["alpha.installer.openshift.io/etcd-bootstrap"]
		if ok {
			bootstrapIP = annotation
			break
		}
	}
	// Return empty hostname if bootstrap IP is not found
	if len(bootstrapIP) == 0 {
		return "", "", nil
	}

	bootstrapHostname := ""
	secretList, err := kubeClient.CoreV1().Secrets("openshift-etcd").List(ctx, metav1.ListOptions{})
	if err != nil {
		return bootstrapIP, "", err
	}
	for _, secret := range secretList.Items {
		certHostNames, ok := secret.Annotations["auth.openshift.io/certificate-hostnames"]
		if !ok || !strings.Contains(certHostNames, bootstrapIP) {
			continue
		}
		// Extract bootstrap name from etcd secret name
		if result, found := strings.CutPrefix(secret.Name, "etcd-peer-"); found {
			bootstrapHostname = result
			break
		}
	}

	return bootstrapIP, bootstrapHostname, nil
}
