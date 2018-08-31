package app

import (
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	clientgoinformers "k8s.io/client-go/informers"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/master"
)

type KubeAPIServerConfigFunc func(config *genericapiserver.Config, sharedInformers informers.SharedInformerFactory, versionedInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer) (genericapiserver.DelegationTarget, error)

var OpenShiftKubeAPIServerConfigPatch KubeAPIServerConfigFunc = nil

type KubeAPIServerServerFunc func(server *master.Master) error

func PatchKubeAPIServerConfig(config *genericapiserver.Config, sharedInformers informers.SharedInformerFactory, versionedInformers clientgoinformers.SharedInformerFactory, pluginInitializers *[]admission.PluginInitializer) (genericapiserver.DelegationTarget, error) {
	if OpenShiftKubeAPIServerConfigPatch == nil {
		return genericapiserver.NewEmptyDelegate(), nil
	}

	return OpenShiftKubeAPIServerConfigPatch(config, sharedInformers, versionedInformers, pluginInitializers)
}

var OpenShiftKubeAPIServerServerPatch KubeAPIServerServerFunc = nil

func PatchKubeAPIServerServer(server *master.Master) error {
	if OpenShiftKubeAPIServerServerPatch == nil {
		return nil
	}

	return OpenShiftKubeAPIServerServerPatch(server)
}

var StartingDelegate genericapiserver.DelegationTarget = genericapiserver.NewEmptyDelegate()
