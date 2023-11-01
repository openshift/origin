package certgraphanalysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type certGenerationOptions struct {
	rejectConfigMap  configMapFilterFunc
	rejectSecret     secretFilterFunc
	rewriteConfigMap configMapRewriteFunc
	rewriteSecret    secretRewriteFunc
}

var (
	SkipRevisioned = &certGenerationOptions{
		rejectConfigMap: func(configMap *corev1.ConfigMap) bool {
			return isRevisioned(configMap.OwnerReferences)
		},
		rejectSecret: func(secret *corev1.Secret) bool {
			return isRevisioned(secret.OwnerReferences)
		},
	}
	RewriteNames = &certGenerationOptions{
		rewriteConfigMap: func(caBundle *certgraphapi.CertificateAuthorityBundle, nodes map[string]int) {},
		rewriteSecret: func(keyPair *certgraphapi.CertKeyPair, nodes map[string]int) {
			for i := range keyPair.Spec.SecretLocations {
				location := keyPair.Spec.SecretLocations[i]
				rewriteEtcdServingMetricsCertificateName(&location, nodes)
				rewriteEtcdServingCertificateName(&location, nodes)
				rewriteEtcdPeerCertificateName(&location, nodes)
				keyPair.Spec.SecretLocations[i] = location
			}
		},
	}
	SkipHashed = &certGenerationOptions{
		rejectConfigMap: func(configMap *corev1.ConfigMap) bool {
			return hasMonitoringHashLabel(configMap.Labels)
		},
		rejectSecret: func(secret *corev1.Secret) bool {
			return hasMonitoringHashLabel(secret.Labels)
		},
	}
)

type certGenerationOptionList struct {
	options []*certGenerationOptions
	nodes   map[string]int
}

func NewCertGenerationOptionList(ctx context.Context, kubeClient kubernetes.Interface, options []*certGenerationOptions) (*certGenerationOptionList, error) {
	nodes := map[string]int{}
	nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane"})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster nodes: %v", err)
	}
	for i, node := range nodeList.Items {
		nodes[node.Name] = i
	}
	return &certGenerationOptionList{
		options: options,
		nodes:   nodes,
	}, nil
}

func (l certGenerationOptionList) rejectConfigMap(configMap *corev1.ConfigMap) bool {
	for _, option := range l.options {
		if option.rejectConfigMap == nil {
			continue
		}
		if option.rejectConfigMap(configMap) {
			return true
		}
	}
	return false
}

func (l certGenerationOptionList) rejectSecret(secret *corev1.Secret) bool {
	for _, option := range l.options {
		if option.rejectSecret == nil {
			continue
		}
		if option.rejectSecret(secret) {
			return true
		}
	}
	return false
}

func (l certGenerationOptionList) rewriteConfigMap(caBundle *certgraphapi.CertificateAuthorityBundle) {
	for _, option := range l.options {
		if option.rewriteConfigMap == nil {
			continue
		}
		option.rewriteConfigMap(caBundle, l.nodes)
	}
}

func (l certGenerationOptionList) rewriteSecret(keyPair *certgraphapi.CertKeyPair) {
	for _, option := range l.options {
		if option.rewriteSecret == nil {
			continue
		}
		option.rewriteSecret(keyPair, l.nodes)
	}
}

func isRevisioned(ownerReferences []metav1.OwnerReference) bool {
	for _, curr := range ownerReferences {
		if strings.HasPrefix(curr.Name, "revision-status-") {
			return true
		}
	}

	return false
}

func hasMonitoringHashLabel(labels map[string]string) bool {
	_, ok := labels["monitoring.openshift.io/hash"]
	return ok
}

func rewriteEtcdServingMetricsCertificateName(location *certgraphapi.InClusterSecretLocation, nodes map[string]int) {
	if location.Namespace != "openshift-etcd" {
		return
	}
	if !strings.HasPrefix(location.Name, "etcd-serving-") {
		return
	}
	master := location.Name[len("etcd-serving-"):]
	location.Name = fmt.Sprintf("etcd-serving-for-master-%d", nodes[master])
}

func rewriteEtcdServingCertificateName(location *certgraphapi.InClusterSecretLocation, nodes map[string]int) {
	if location.Namespace != "openshift-etcd" {
		return
	}
	if !strings.HasPrefix(location.Name, "etcd-serving-metrics-") {
		return
	}
	master := location.Name[len("etcd-serving-metrics-"):]
	location.Name = fmt.Sprintf("etcd-metrics-for-master-%d", nodes[master])
}

func rewriteEtcdPeerCertificateName(location *certgraphapi.InClusterSecretLocation, nodes map[string]int) {
	if location.Namespace != "openshift-etcd" {
		return
	}
	if !strings.HasPrefix(location.Name, "etcd-peer-") {
		return
	}
	master := location.Name[len("etcd-peer-"):]
	location.Name = fmt.Sprintf("etcd-peer-for-master-%d", nodes[master])
}
