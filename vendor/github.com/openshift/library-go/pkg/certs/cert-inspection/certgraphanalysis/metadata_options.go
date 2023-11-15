package certgraphanalysis

import (
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
)

type configMapRewriteFunc func(configMap *corev1.ConfigMap)
type secretRewriteFunc func(secret *corev1.Secret)
type caBundleRewriteFunc func(caBundle *certgraphapi.CertificateAuthorityBundle)
type certKeyPairRewriteFunc func(certKeyPair *certgraphapi.CertKeyPair)

type metadataOptions struct {
	rewriteCABundle    caBundleRewriteFunc
	rewriteCertKeyPair certKeyPairRewriteFunc
	rewriteConfigMap   configMapRewriteFunc
	rewriteSecret      secretRewriteFunc
}

func (metadataOptions) approved() {}

func (l certGenerationOptionList) rewriteCABundle(caBundle *certgraphapi.CertificateAuthorityBundle) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteCABundle == nil {
			continue
		}
		option.rewriteCABundle(caBundle)
	}
}

func (l certGenerationOptionList) rewriteCertKeyPair(certKeyPair *certgraphapi.CertKeyPair) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteCertKeyPair == nil {
			continue
		}
		option.rewriteCertKeyPair(certKeyPair)
	}
}

func (l certGenerationOptionList) rewriteConfigMap(configMap *corev1.ConfigMap) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteConfigMap == nil {
			continue
		}
		option.rewriteConfigMap(configMap)
	}
}

func (l certGenerationOptionList) rewriteSecret(secret *corev1.Secret) {
	for _, curr := range l {
		option, ok := curr.(*metadataOptions)
		if !ok {
			continue
		}
		if option.rewriteSecret == nil {
			continue
		}
		option.rewriteSecret(secret)
	}
}

var (
	ElideProxyCADetails = &metadataOptions{
		rewriteCABundle: func(caBundle *certgraphapi.CertificateAuthorityBundle) {
			isProxyCA := false
			for _, location := range caBundle.Spec.ConfigMapLocations {
				if location.Namespace == "openshift-config-managed" && location.Name == "trusted-ca-bundle" {
					isProxyCA = true
				}
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

func RewriteNodeIPs(nodeList *corev1.NodeList) *metadataOptions {
	nodes := map[string]int{}
	for i, node := range nodeList.Items {
		nodes[node.Name] = i
	}
	return &metadataOptions{
		rewriteSecret: func(secret *corev1.Secret) {
			rewriteIPInEtcdServingMetricsSecret(secret, nodes)
			rewriteIPInEtcdServingSecret(secret, nodes)
			rewriteIPInEtcdPeerSecret(secret, nodes)
		},
	}
}

func rewriteIPInEtcdServingSecret(secret *corev1.Secret, nodes map[string]int) {
	if secret.Namespace == "openshift-etcd" && strings.HasPrefix(secret.Name, "etcd-serving-") {
		master := secret.Name[len("etcd-serving-"):]
		secret.Name = fmt.Sprintf("etcd-serving-for-master-%d", nodes[master])
	}
}

func rewriteIPInEtcdServingMetricsSecret(secret *corev1.Secret, nodes map[string]int) {
	if secret.Namespace == "openshift-etcd" && strings.HasPrefix(secret.Name, "etcd-serving-metrics-") {
		master := secret.Name[len("etcd-serving-metrics-"):]
		secret.Name = fmt.Sprintf("etcd-metrics-for-master-%d", nodes[master])
	}
}

func rewriteIPInEtcdPeerSecret(secret *corev1.Secret, nodes map[string]int) {
	if secret.Namespace == "openshift-etcd" && strings.HasPrefix(secret.Name, "etcd-peer-") {
		master := secret.Name[len("etcd-peer-"):]
		secret.Name = fmt.Sprintf("etcd-peer-for-master-%d", nodes[master])
	}
}
