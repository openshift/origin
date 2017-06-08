/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"k8s.io/apiserver/pkg/admission"

	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"

	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/internalversion"
)

// WantsInternalServiceCatalogClientSet defines a function which sets ClientSet for admission plugins that need it
type WantsInternalServiceCatalogClientSet interface {
	SetInternalServiceCatalogClientSet(internalclientset.Interface)
	admission.Validator
}

// WantsInternalServiceCatalogInformerFactory defines a function which sets InformerFactory for admission plugins that need it
type WantsInternalServiceCatalogInformerFactory interface {
	SetInternalServiceCatalogInformerFactory(informers.SharedInformerFactory)
	admission.Validator
}

// WantsKubeClientSet defines a function which sets ClientSet for admission plugins that need it
type WantsKubeClientSet interface {
	SetKubeClientSet(kubeclientset.Interface)
	admission.Validator
}

// WantsKubeInformerFactory defines a function which sets InformerFactory for admission plugins that need it
type WantsKubeInformerFactory interface {
	SetKubeInformerFactory(kubeinformers.SharedInformerFactory)
	admission.Validator
}

type pluginInitializer struct {
	internalClient internalclientset.Interface
	informers      informers.SharedInformerFactory

	kubeClient    kubeclientset.Interface
	kubeInformers kubeinformers.SharedInformerFactory
}

var _ admission.PluginInitializer = pluginInitializer{}

// NewPluginInitializer constructs new instance of PluginInitializer
func NewPluginInitializer(internalClient internalclientset.Interface, sharedInformers informers.SharedInformerFactory,
	kubeClient kubeclientset.Interface, kubeInformers kubeinformers.SharedInformerFactory) admission.PluginInitializer {
	return pluginInitializer{
		internalClient: internalClient,
		informers:      sharedInformers,
		kubeClient:     kubeClient,
		kubeInformers:  kubeInformers,
	}
}

// Initialize checks the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i pluginInitializer) Initialize(plugin admission.Interface) {
	if wants, ok := plugin.(WantsInternalServiceCatalogClientSet); ok {
		wants.SetInternalServiceCatalogClientSet(i.internalClient)
	}

	if wants, ok := plugin.(WantsInternalServiceCatalogInformerFactory); ok {
		wants.SetInternalServiceCatalogInformerFactory(i.informers)
	}

	if wants, ok := plugin.(WantsKubeClientSet); ok {
		wants.SetKubeClientSet(i.kubeClient)
	}

	if wants, ok := plugin.(WantsKubeInformerFactory); ok {
		wants.SetKubeInformerFactory(i.kubeInformers)
	}
}
