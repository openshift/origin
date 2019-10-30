package encryption

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/library-go/pkg/operator/encryption/controllers"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
)

type runner interface {
	Run(stopCh <-chan struct{})
}

func NewControllers(
	component string,
	deployer statemachine.Deployer,
	operatorClient operatorv1helpers.OperatorClient,
	apiServerClient configv1client.APIServerInterface,
	apiServerInformer configv1informers.APIServerInformer,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	secretsClient corev1.SecretsGetter,
	discovery discovery.DiscoveryInterface,
	eventRecorder events.Recorder,
	dynamicClient dynamic.Interface, // temporary hack for in-process storage migration
	encryptedGRs ...schema.GroupResource,
) (*Controllers, error) {
	// avoid using the CachedSecretGetter as we need strong guarantees that our encryptionSecretSelector works
	// otherwise we could see secrets from a different component (which will break our keyID invariants)
	// this is fine in terms of performance since these controllers will be idle most of the time
	// TODO: update the eventHandlers used by the controllers to ignore components that do not match their own
	encryptionSecretSelector := metav1.ListOptions{LabelSelector: secrets.EncryptionKeySecretsLabel + "=" + component}

	return &Controllers{
		controllers: []runner{
			controllers.NewKeyController(
				component,
				deployer,
				operatorClient,
				apiServerClient,
				apiServerInformer,
				kubeInformersForNamespaces,
				secretsClient,
				encryptionSecretSelector,
				eventRecorder,
				encryptedGRs,
			),
			controllers.NewStateController(
				component,
				deployer,
				operatorClient,
				kubeInformersForNamespaces,
				secretsClient,
				encryptionSecretSelector,
				eventRecorder,
				encryptedGRs,
			),
			controllers.NewPruneController(
				deployer,
				operatorClient,
				kubeInformersForNamespaces,
				secretsClient,
				encryptionSecretSelector,
				eventRecorder,
				encryptedGRs,
			),
			controllers.NewMigrationController(
				deployer,
				operatorClient,
				kubeInformersForNamespaces,
				secretsClient,
				encryptionSecretSelector,
				eventRecorder,
				encryptedGRs,
				dynamicClient,
				discovery,
			),
		},
	}, nil
}

type Controllers struct {
	controllers []runner
}

func (c *Controllers) Run(stopCh <-chan struct{}) {
	for _, controller := range c.controllers {
		con := controller // capture range variable
		go con.Run(stopCh)
	}
	<-stopCh
}
