package certgraphanalysis

import (
	"context"
	"fmt"
	"strings"

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
		rewriteConfigMap: func(configMap *corev1.ConfigMap, nodes map[string]int) (string, string) {
			return configMap.Name, configMap.Namespace
		},
		rewriteSecret: func(secret *corev1.Secret, nodes map[string]int) (string, string) {
			if name := rewriteEtcdServingMetricsCertificateName(secret, nodes); name != secret.Name {
				return name, secret.Namespace
			}
			if name := rewriteEtcdServingCertificateName(secret, nodes); name != secret.Name {
				return name, secret.Namespace
			}
			if name := rewriteEtcdPeerCertificateName(secret, nodes); name != secret.Name {
				return name, secret.Namespace
			}
			return secret.Name, secret.Namespace
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

func (l certGenerationOptionList) rewriteConfigMap(configMap *corev1.ConfigMap) (string, string) {
	var name, namespace string
	for _, option := range l.options {
		if option.rewriteConfigMap == nil {
			continue
		}
		name, namespace = option.rewriteConfigMap(configMap, l.nodes)
	}
	return name, namespace
}

func (l certGenerationOptionList) rewriteSecret(secret *corev1.Secret) (string, string) {
	var name, namespace string
	for _, option := range l.options {
		if option.rewriteSecret == nil {
			continue
		}
		name, namespace = option.rewriteSecret(secret, l.nodes)
	}
	return name, namespace
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

func rewriteEtcdServingCertificateName(secret *corev1.Secret, nodes map[string]int) string {
	if secret.Namespace != "openshift-etcd" {
		return secret.Name
	}
	if !strings.HasPrefix(secret.Name, "etcd-serving-") {
		return secret.Name
	}
	master := secret.Name[len("etcd-serving-"):]
	return fmt.Sprintf("etcd-serving-for-master-%d", nodes[master])
}

func rewriteEtcdServingMetricsCertificateName(secret *corev1.Secret, nodes map[string]int) string {
	if secret.Namespace != "openshift-etcd" {
		return secret.Name
	}
	if !strings.HasPrefix(secret.Name, "etcd-serving-metrics-") {
		return secret.Name
	}
	master := secret.Name[len("etcd-serving-metrics-"):]
	return fmt.Sprintf("etcd-metrics-for-master-%d", nodes[master])
}

func rewriteEtcdPeerCertificateName(secret *corev1.Secret, nodes map[string]int) string {
	if secret.Namespace != "openshift-etcd" {
		return secret.Name
	}
	if !strings.HasPrefix(secret.Name, "etcd-peer-") {
		return secret.Name
	}
	master := secret.Name[len("etcd-peer-"):]
	return fmt.Sprintf("etcd-peer-for-master-%d", nodes[master])
}
