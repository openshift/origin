package v1helpers

import "k8s.io/client-go/informers"

func NewFakeKubeInformersForNamespaces(informers map[string]informers.SharedInformerFactory) KubeInformersForNamespaces {
	return kubeInformersForNamespaces(informers)
}
