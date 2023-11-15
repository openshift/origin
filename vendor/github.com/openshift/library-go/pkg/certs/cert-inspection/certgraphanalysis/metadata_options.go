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

func RewriteNodeIPs(nodeList []corev1.Node) *metadataOptions {
	nodes := map[string]int{}
	for i, node := range nodeList {
		nodes[node.Name] = i
	}
	return &metadataOptions{
		rewriteSecret: func(secret *corev1.Secret) {
			for nodeName, masterID := range nodes {
				name := strings.ReplaceAll(secret.Name, nodeName, fmt.Sprintf("<master-%d>", masterID))
				if secret.Name != name {
					secret.Name = name
					if len(secret.Annotations) == 0 {
						secret.Annotations = map[string]string{}
					}
					secret.Annotations["openshift.io/last-rewritten-by"] = "RewriteNodeIPs"
				}
			}
		},
	}
}
