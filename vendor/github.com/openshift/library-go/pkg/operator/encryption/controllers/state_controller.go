package controllers

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

const stateWorkKey = "key"

// stateController is responsible for creating a single secret in
// openshift-config-managed with the name destName.  This single secret
// contains the complete EncryptionConfiguration that is consumed by the API
// server that is performing the encryption.  Thus this secret represents
// the current state of all resources in encryptedGRs.  Every encryption key
// that matches encryptionSecretSelector is included in this final secret.
// This secret is synced into targetNamespace at a static location.  This
// indirection allows the cluster to recover from the deletion of targetNamespace.
// See getResourceConfigs for details on how the raw state of all keys
// is converted into a single encryption config.  The logic for determining
// the current write key is of special interest.
type stateController struct {
	component                string
	name                     string
	encryptionSecretSelector metav1.ListOptions

	operatorClient operatorv1helpers.OperatorClient
	secretClient   corev1client.SecretsGetter
	deployer       statemachine.Deployer
	provider       Provider
}

func NewStateController(
	component string,
	provider Provider,
	deployer statemachine.Deployer,
	operatorClient operatorv1helpers.OperatorClient,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	secretClient corev1client.SecretsGetter,
	encryptionSecretSelector metav1.ListOptions,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &stateController{
		operatorClient: operatorClient,
		name:           "EncryptionStateController",

		component: component,

		encryptionSecretSelector: encryptionSecretSelector,
		secretClient:             secretClient,
		deployer:                 deployer,
		provider:                 provider,
	}

	return factory.New().ResyncEvery(time.Second).WithSync(c.sync).WithInformers(
		operatorClient.Informer(),
		kubeInformersForNamespaces.InformersFor("openshift-config-managed").Core().V1().Secrets().Informer(),
		deployer,
	).ToController(c.name, eventRecorder.WithComponentSuffix("encryption-state-controller"))

}

func (c *stateController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	if ready, err := shouldRunEncryptionController(c.operatorClient, c.provider.ShouldRunEncryptionControllers); err != nil || !ready {
		return err // we will get re-kicked when the operator status updates
	}

	configError := c.generateAndApplyCurrentEncryptionConfigSecret(syncCtx.Queue(), syncCtx.Recorder(), c.provider.EncryptedGRs())

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   "EncryptionStateControllerDegraded",
		Status: operatorv1.ConditionFalse,
	}
	if configError != nil {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = configError.Error()
	}
	if _, _, updateError := operatorv1helpers.UpdateStatus(c.operatorClient, operatorv1helpers.UpdateConditionFn(cond)); updateError != nil {
		return updateError
	}

	return configError
}

type eventWithReason struct {
	reason  string
	message string
}

func (c *stateController) generateAndApplyCurrentEncryptionConfigSecret(queue workqueue.RateLimitingInterface, recorder events.Recorder, encryptedGRs []schema.GroupResource) error {
	currentConfig, desiredEncryptionState, encryptionSecrets, transitioningReason, err := statemachine.GetEncryptionConfigAndState(c.deployer, c.secretClient, c.encryptionSecretSelector, encryptedGRs)
	if err != nil {
		return err
	}
	if len(transitioningReason) > 0 {
		queue.AddAfter(stateWorkKey, 2*time.Minute)
		return nil
	}

	if currentConfig == nil && len(encryptionSecrets) == 0 {
		// we depend on the key controller to create the first key to bootstrap encryption.
		// Later-on either the config exists or there are keys, even in the case of disabled
		// encryption via the apiserver config.
		return nil
	}

	desiredEncryptionConfig := encryptionconfig.FromEncryptionState(desiredEncryptionState)
	changed, err := c.applyEncryptionConfigSecret(desiredEncryptionConfig, recorder)
	if err != nil {
		return err
	}

	if changed {
		currentEncryptionConfig, _ := encryptionconfig.ToEncryptionState(currentConfig, encryptionSecrets)
		if actionEvents := eventsFromEncryptionConfigChanges(currentEncryptionConfig, desiredEncryptionState); len(actionEvents) > 0 {
			for _, event := range actionEvents {
				recorder.Eventf(event.reason, event.message)
			}
		}
	}
	return nil
}

func (c *stateController) applyEncryptionConfigSecret(encryptionConfig *apiserverconfigv1.EncryptionConfiguration, recorder events.Recorder) (bool, error) {
	s, err := encryptionconfig.ToSecret("openshift-config-managed", fmt.Sprintf("%s-%s", encryptionconfig.EncryptionConfSecretName, c.component), encryptionConfig)
	if err != nil {
		return false, err
	}

	_, changed, applyErr := resourceapply.ApplySecret(c.secretClient, recorder, s)
	return changed, applyErr
}

// eventsFromEncryptionConfigChanges return slice of event reasons with messages corresponding to a difference between current and desired encryption state.
func eventsFromEncryptionConfigChanges(current, desired map[schema.GroupResource]state.GroupResourceState) []eventWithReason {
	var result []eventWithReason
	// handle removals from current first
	for currentGroupResource := range current {
		if _, exists := desired[currentGroupResource]; !exists {
			result = append(result, eventWithReason{
				reason:  "EncryptionResourceRemoved",
				message: fmt.Sprintf("Resource %q was removed from encryption config", currentGroupResource),
			})
		}
	}
	for desiredGroupResource, desiredGroupResourceState := range desired {
		currentGroupResource, exists := current[desiredGroupResource]
		if !exists {
			keyMessage := "without write key"
			if desiredGroupResourceState.HasWriteKey() {
				keyMessage = fmt.Sprintf("with write key %q", desiredGroupResourceState.WriteKey.Key.Name)
			}
			result = append(result, eventWithReason{
				reason:  "EncryptionResourceAdded",
				message: fmt.Sprintf("Resource %q was added to encryption config %s", desiredGroupResource, keyMessage),
			})
			continue
		}
		if !currentGroupResource.HasWriteKey() && desiredGroupResourceState.HasWriteKey() {
			result = append(result, eventWithReason{
				reason:  "EncryptionKeyPromoted",
				message: fmt.Sprintf("Promoting key %q for resource %q to write key", desiredGroupResourceState.WriteKey.Key.Name, desiredGroupResource),
			})
		}
		if currentGroupResource.HasWriteKey() && !desiredGroupResourceState.HasWriteKey() {
			result = append(result, eventWithReason{
				reason:  "EncryptionKeyRemoved",
				message: fmt.Sprintf("Removing key %q for resource %q to write key", currentGroupResource.WriteKey.Key.Name, desiredGroupResource),
			})
		}
		if currentGroupResource.HasWriteKey() && desiredGroupResourceState.HasWriteKey() {
			if currentGroupResource.WriteKey.ExternalReason != desiredGroupResourceState.WriteKey.ExternalReason {
				result = append(result, eventWithReason{
					reason:  "EncryptionWriteKeyTriggeredExternal",
					message: fmt.Sprintf("Triggered key %q for resource %q because %s", currentGroupResource.WriteKey.Key.Name, desiredGroupResource, desiredGroupResourceState.WriteKey.ExternalReason),
				})
			}
			if currentGroupResource.WriteKey.InternalReason != desiredGroupResourceState.WriteKey.InternalReason {
				result = append(result, eventWithReason{
					reason:  "EncryptionWriteKeyTriggeredInternal",
					message: fmt.Sprintf("Triggered key %q for resource %q because %s", currentGroupResource.WriteKey.Key.Name, desiredGroupResource, desiredGroupResourceState.WriteKey.InternalReason),
				})
			}
			if !state.EqualKeyAndEqualID(&currentGroupResource.WriteKey, &desiredGroupResourceState.WriteKey) {
				result = append(result, eventWithReason{
					reason:  "EncryptionWriteKeyChanged",
					message: fmt.Sprintf("Write key %q for resource %q changed", currentGroupResource.WriteKey.Key.Name, desiredGroupResource),
				})
			}
		}
		if len(currentGroupResource.ReadKeys) != len(desiredGroupResourceState.ReadKeys) {
			result = append(result, eventWithReason{
				reason:  "EncryptionReadKeysChanged",
				message: fmt.Sprintf("Number of read keys for resource %q changed from %d to %d", desiredGroupResource, len(currentGroupResource.ReadKeys), len(desiredGroupResourceState.ReadKeys)),
			})
		}
	}
	return result
}
