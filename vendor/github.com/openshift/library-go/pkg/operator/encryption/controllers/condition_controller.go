package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

// conditionController maintains the Encrypted condition. It sets it to true iff there is a
// fully migrated read-key in the current config, and no later key is of identity type.
type conditionController struct {
	operatorClient operatorv1helpers.OperatorClient

	encryptedGRs []schema.GroupResource

	encryptionSecretSelector metav1.ListOptions

	deployer     statemachine.Deployer
	secretClient corev1client.SecretsGetter
}

func NewConditionController(
	deployer statemachine.Deployer,
	operatorClient operatorv1helpers.OperatorClient,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	secretClient corev1client.SecretsGetter,
	encryptionSecretSelector metav1.ListOptions,
	eventRecorder events.Recorder,
	encryptedGRs []schema.GroupResource,
) factory.Controller {
	c := &conditionController{
		operatorClient: operatorClient,

		encryptedGRs: encryptedGRs,

		encryptionSecretSelector: encryptionSecretSelector,
		deployer:                 deployer,
		secretClient:             secretClient,
	}

	return factory.New().WithInformers(
		kubeInformersForNamespaces.InformersFor("openshift-config-managed").Core().V1().Secrets().Informer(),
		operatorClient.Informer(),
		deployer,
	).ResyncEvery(time.Second).WithSync(c.sync).ToController("EncryptionConditionController", eventRecorder.WithComponentSuffix("encryption-condition-controller"))
}

func (c *conditionController) sync(ctx context.Context, syncContext factory.SyncContext) error {
	if ready, err := shouldRunEncryptionController(c.operatorClient); err != nil || !ready {
		return err // we will get re-kicked when the operator status updates
	}

	currentConfig, desiredState, foundSecrets, transitioningReason, err := statemachine.GetEncryptionConfigAndState(c.deployer, c.secretClient, c.encryptionSecretSelector, c.encryptedGRs)
	if err != nil || len(transitioningReason) > 0 {
		return err
	}

	cond := operatorv1.OperatorCondition{
		Type:    "Encrypted",
		Status:  operatorv1.ConditionTrue,
		Reason:  "EncryptionCompleted",
		Message: fmt.Sprintf("All resources encrypted: %s", grString(c.encryptedGRs)),
	}
	currentState, _ := encryptionconfig.ToEncryptionState(currentConfig, foundSecrets)

	if len(foundSecrets) == 0 {
		cond.Status = operatorv1.ConditionFalse
		cond.Reason = "EncryptionDisabled"
		cond.Message = "Encryption is not enabled"
	} else {
		// check for identity key in desired state first. This will make us catch upcoming decryption early before
		// it settles into the current config.
		for _, s := range desiredState {
			if s.WriteKey.Mode != state.Identity {
				continue
			}

			if allMigrated(c.encryptedGRs, s.WriteKey.Migrated.Resources) {
				cond.Status = operatorv1.ConditionFalse
				cond.Reason = "DecryptionCompleted"
				cond.Message = "Encryption mode set to identity and everything is decrypted"
			} else {
				cond.Status = operatorv1.ConditionFalse
				cond.Reason = "DecryptionInProgress"
				cond.Message = "Encryption mode set to identity and decryption is not finished"
			}
			break
		}
	}
	if cond.Status == operatorv1.ConditionTrue {
		// now that the desired state look like it won't lead to identity as write-key, test the current state
	NextResource:
		for _, gr := range c.encryptedGRs {
			s, ok := currentState[gr]
			if !ok {
				cond.Status = operatorv1.ConditionFalse
				cond.Reason = "EncryptionInProgress"
				cond.Message = fmt.Sprintf("Resource %s is not encrypted", gr.String())
				break NextResource
			}

			if s.WriteKey.Mode == state.Identity {
				if allMigrated(c.encryptedGRs, s.WriteKey.Migrated.Resources) {
					cond.Status = operatorv1.ConditionFalse
					cond.Reason = "DecryptionCompleted"
					cond.Message = "Encryption mode set to identity and everything is decrypted"
				} else {
					cond.Status = operatorv1.ConditionFalse
					cond.Reason = "DecryptionInProgress"
					cond.Message = "Encryption mode set to identity and decryption is not finished"
				}
				break
			}

			// go through read keys until we find a completely migrated one. Finding an identity mode before
			// means migration is ongoing. :
			for _, rk := range s.ReadKeys {
				if rk.Mode == state.Identity {
					cond.Status = operatorv1.ConditionFalse
					cond.Reason = "EncryptionInProgress"
					cond.Message = "Encryption is ongoing"
					break NextResource
				}
				if migratedSet(rk.Migrated.Resources).Has(gr.String()) {
					continue NextResource
				}
			}

			cond.Status = operatorv1.ConditionFalse
			cond.Reason = "EncryptionInProgress"
			cond.Message = fmt.Sprintf("Resource %s is being encrypted", gr.String())
			break
		}
	}

	// update Encrypted condition
	_, _, updateError := operatorv1helpers.UpdateStatus(c.operatorClient, operatorv1helpers.UpdateConditionFn(cond))
	return updateError
}

func allMigrated(toBeEncrypted, migrated []schema.GroupResource) bool {
	s := migratedSet(migrated)
	for _, gr := range toBeEncrypted {
		if !s.Has(gr.String()) {
			return false
		}
	}
	return true
}

func migratedSet(grs []schema.GroupResource) sets.String {
	migrated := sets.NewString()
	for _, gr := range grs {
		migrated.Insert(gr.String())
	}
	return migrated
}

func grString(grs []schema.GroupResource) string {
	ss := make([]string, 0, len(grs))
	for _, gr := range grs {
		ss = append(ss, gr.String())
	}
	return strings.Join(ss, ", ")
}
