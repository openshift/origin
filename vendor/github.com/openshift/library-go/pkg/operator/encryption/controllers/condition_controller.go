package controllers

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	conditionWorkKey = "key"
)

// conditionController maintains the Encrypted condition. It sets it to true iff there is a
// fully migrated read-key in the current config, and no later key is of identity type.
type conditionController struct {
	operatorClient operatorv1helpers.OperatorClient

	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder

	preRunCachesSynced []cache.InformerSynced

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
) *conditionController {
	c := &conditionController{
		operatorClient: operatorClient,

		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EncryptionConditionController"),
		eventRecorder: eventRecorder.WithComponentSuffix("encryption-condition-controller"),

		encryptedGRs: encryptedGRs,

		encryptionSecretSelector: encryptionSecretSelector,
		deployer:                 deployer,
		secretClient:             secretClient,
	}

	c.preRunCachesSynced = setUpInformers(deployer, operatorClient, kubeInformersForNamespaces, c.eventHandler())

	return c
}

func (c *conditionController) sync() error {
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

func (c *conditionController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting EncryptionConditionController")
	defer klog.Infof("Shutting down EncryptionConditionController")
	if !cache.WaitForCacheSync(stopCh, c.preRunCachesSynced...) {
		utilruntime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}

	// only start one worker
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *conditionController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *conditionController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with: %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

func (c *conditionController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(conditionWorkKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(conditionWorkKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(conditionWorkKey) },
	}
}

func grString(grs []schema.GroupResource) string {
	ss := make([]string, 0, len(grs))
	for _, gr := range grs {
		ss = append(ss, gr.String())
	}
	return strings.Join(ss, ", ")
}
