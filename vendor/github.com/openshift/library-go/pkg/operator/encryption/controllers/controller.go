package controllers

import (
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	"github.com/openshift/library-go/pkg/operator/management"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

func shouldRunEncryptionController(operatorClient operatorv1helpers.OperatorClient) (bool, error) {
	operatorSpec, _, _, err := operatorClient.GetOperatorState()
	if err != nil {
		return false, err
	}

	return management.IsOperatorManaged(operatorSpec.ManagementState), nil
}

func setUpInformers(
	deployer statemachine.Deployer,
	operatorClient operatorv1helpers.OperatorClient,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	eventHandler cache.ResourceEventHandler,
) []cache.InformerSynced {
	operatorInformer := operatorClient.Informer()
	operatorInformer.AddEventHandler(eventHandler)

	managedSecretsInformer := kubeInformersForNamespaces.InformersFor("openshift-config-managed").Core().V1().Secrets().Informer()
	managedSecretsInformer.AddEventHandler(eventHandler)

	return append([]cache.InformerSynced{
		operatorInformer.HasSynced,
		managedSecretsInformer.HasSynced,
	}, deployer.AddEventHandler(eventHandler)...)
}
