package certgraphanalysis

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
)

const rewritePrefix = "rewritten.cert-info.openshift.io/"

type configMapRewriteFunc func(configMap *corev1.ConfigMap)
type secretRewriteFunc func(secret *corev1.Secret)
type caBundleRewriteFunc func(metadata metav1.ObjectMeta, caBundle *certgraphapi.CertificateAuthorityBundle)
type certKeyPairRewriteFunc func(metadata metav1.ObjectMeta, certKeyPair *certgraphapi.CertKeyPair)

type metadataOptions struct {
	rewriteCABundleFn    caBundleRewriteFunc
	rewriteCertKeyPairFn certKeyPairRewriteFunc
	rewriteConfigMapFn   configMapRewriteFunc
	rewriteSecretFn      secretRewriteFunc
}

var (
	_ configMapRewriter           = &metadataOptions{}
	_ secretRewriter              = &metadataOptions{}
	_ caBundleMetadataRewriter    = &metadataOptions{}
	_ certKeypairMetadataRewriter = &metadataOptions{}
)

func (*metadataOptions) approved() {}

func (o *metadataOptions) rewriteCABundle(metadata metav1.ObjectMeta, caBundle *certgraphapi.CertificateAuthorityBundle) {
	if o.rewriteCABundleFn == nil {
		return
	}
	o.rewriteCABundleFn(metadata, caBundle)
}

func (o *metadataOptions) rewriteCertKeyPair(metadata metav1.ObjectMeta, certKeyPair *certgraphapi.CertKeyPair) {
	if o.rewriteCertKeyPairFn == nil {
		return
	}
	o.rewriteCertKeyPairFn(metadata, certKeyPair)
}

func (o *metadataOptions) rewriteConfigMap(configMap *corev1.ConfigMap) {
	if o.rewriteConfigMapFn == nil {
		return
	}
	o.rewriteConfigMapFn(configMap)
}

func (o *metadataOptions) rewriteSecret(secret *corev1.Secret) {
	if o.rewriteSecretFn == nil {
		return
	}
	o.rewriteSecretFn(secret)
}

var (
	ElideProxyCADetails = &metadataOptions{
		rewriteCABundleFn: func(metadata metav1.ObjectMeta, caBundle *certgraphapi.CertificateAuthorityBundle) {
			isProxyCA := false
			if metadata.Namespace == "openshift-config-managed" && metadata.Name == "trusted-ca-bundle" {
				isProxyCA = true
			}
			// this plugin does a direct copy
			if metadata.Namespace == "openshift-cloud-controller-manager" && metadata.Name == "ccm-trusted-ca" {
				isProxyCA = true
			}
			// this namespace appears to hash (notice trailing dash) the content and lose labels
			if metadata.Namespace == "openshift-monitoring" && strings.Contains(metadata.Name, "-trusted-ca-bundle-") {
				isProxyCA = true
			}
			if len(metadata.Labels["config.openshift.io/inject-trusted-cabundle"]) > 0 {
				isProxyCA = true
			}

			if !isProxyCA {
				return
			}
			if len(caBundle.Spec.CertificateMetadata) < 10 {
				return
			}
			caBundle.Name = "proxy-ca"
			caBundle.LogicalName = "proxy-ca"
			caBundle.Spec.CertificateMetadata = []certgraphapi.CertKeyMetadata{
				{
					CertIdentifier: certgraphapi.CertIdentifier{
						CommonName:   "synthetic-proxy-ca",
						SerialNumber: "0",
						Issuer:       nil,
					},
				},
			}
		},
	}
)

func RewriteNodeIPs(nodeList []*corev1.Node) *metadataOptions {
	nodes := map[string]int{}
	for i, node := range nodeList {
		nodes[node.Name] = i
	}
	return &metadataOptions{
		rewriteSecretFn: func(secret *corev1.Secret) {
			for nodeName, masterID := range nodes {
				name := strings.ReplaceAll(secret.Name, nodeName, fmt.Sprintf("<master-%d>", masterID))
				if secret.Name != name {
					secret.Name = name
					if len(secret.Annotations) == 0 {
						secret.Annotations = map[string]string{}
					}
					secret.Annotations[rewritePrefix+"RewriteNodeIPs"] = nodeName
				}
			}
		},
	}
}
