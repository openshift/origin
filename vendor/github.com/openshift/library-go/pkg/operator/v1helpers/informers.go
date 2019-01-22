package v1helpers

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// KubeInformersForNamespaces is a simple way to combine several shared informers into a single struct with unified listing power
type KubeInformersForNamespaces interface {
	Start(stopCh <-chan struct{})
	InformersFor(namespace string) informers.SharedInformerFactory
	Namespaces() sets.String

	ConfigMapLister() corev1listers.ConfigMapLister
	SecretLister() corev1listers.SecretLister
}

func NewKubeInformersForNamespaces(kubeClient kubernetes.Interface, namespaces ...string) KubeInformersForNamespaces {
	ret := kubeInformersForNamespaces{}
	for _, namespace := range namespaces {
		if len(namespace) == 0 {
			ret[""] = informers.NewSharedInformerFactory(kubeClient, 10*time.Minute)
			continue
		}
		ret[namespace] = informers.NewSharedInformerFactoryWithOptions(kubeClient, 10*time.Minute, informers.WithNamespace(namespace))
	}

	return ret
}

type kubeInformersForNamespaces map[string]informers.SharedInformerFactory

func (i kubeInformersForNamespaces) Start(stopCh <-chan struct{}) {
	for _, informer := range i {
		informer.Start(stopCh)
	}
}

func (i kubeInformersForNamespaces) Namespaces() sets.String {
	return sets.StringKeySet(i)
}
func (i kubeInformersForNamespaces) InformersFor(namespace string) informers.SharedInformerFactory {
	return i[namespace]
}

func (i kubeInformersForNamespaces) HasInformersFor(namespace string) bool {
	return i.InformersFor(namespace) != nil
}

type configMapLister kubeInformersForNamespaces

func (i kubeInformersForNamespaces) ConfigMapLister() corev1listers.ConfigMapLister {
	return configMapLister(i)
}

func (l configMapLister) List(selector labels.Selector) (ret []*corev1.ConfigMap, err error) {
	globalInformer, ok := l[""]
	if !ok {
		return nil, fmt.Errorf("combinedLister does not support cross namespace list")
	}

	return globalInformer.Core().V1().ConfigMaps().Lister().List(selector)
}

func (l configMapLister) ConfigMaps(namespace string) corev1listers.ConfigMapNamespaceLister {
	informer, ok := l[namespace]
	if !ok {
		// coding error
		panic(fmt.Sprintf("namespace %q is missing", namespace))
	}

	return informer.Core().V1().ConfigMaps().Lister().ConfigMaps(namespace)
}

type secretLister kubeInformersForNamespaces

func (i kubeInformersForNamespaces) SecretLister() corev1listers.SecretLister {
	return secretLister(i)
}

func (l secretLister) List(selector labels.Selector) (ret []*corev1.Secret, err error) {
	globalInformer, ok := l[""]
	if !ok {
		return nil, fmt.Errorf("combinedLister does not support cross namespace list")
	}

	return globalInformer.Core().V1().Secrets().Lister().List(selector)
}

func (l secretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	informer, ok := l[namespace]
	if !ok {
		// coding error
		panic(fmt.Sprintf("namespace %q is missing", namespace))
	}

	return informer.Core().V1().Secrets().Lister().Secrets(namespace)
}
