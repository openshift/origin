package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	apiserverv1 "k8s.io/apiserver/pkg/apis/config/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
	"k8s.io/utils/pointer"

	operatorv1 "github.com/openshift/api/operator/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/encryption/crypto"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

// encryptionSecretMigrationInterval determines how much time must pass after a key has been observed as
// migrated before a new key is created by the key minting controller.  The new key's ID will be one
// greater than the last key's ID (the first key has a key ID of 1).
const encryptionSecretMigrationInterval = time.Hour * 24 * 7 // one week

// keyController creates new keys if necessary. It
// * watches
//   - secrets in openshift-config-managed
//   - pods in target namespace
//   - secrets in target namespace
// * computes a new, desired encryption config from encryption-config-<revision>
//   and the existing keys in openshift-config-managed.
// * derives from the desired encryption config whether a new key is needed due to
//   - encryption is being enabled via the API or
//   - a new to-be-encrypted resource shows up or
//   - the EncryptionType in the API does not match with the newest existing key or
//   - based on time (once a week is the proposed rotation interval) or
//   - an external reason given as a string in .encryption.reason of UnsupportedConfigOverrides.
//   It then creates it.
//
// Note: the "based on time" reason for a new key is based on the annotation
//       encryption.apiserver.operator.openshift.io/migrated-timestamp instead of
//       the key secret's creationTimestamp because the clock is supposed to
//       start when a migration has been finished, not when it begins.
type keyController struct {
	operatorClient  operatorv1helpers.OperatorClient
	apiServerClient configv1client.APIServerInterface

	component                string
	name                     string
	encryptionSecretSelector metav1.ListOptions

	deployer     statemachine.Deployer
	secretClient corev1client.SecretsGetter
	provider     Provider
}

func NewKeyController(
	component string,
	provider Provider,
	deployer statemachine.Deployer,
	operatorClient operatorv1helpers.OperatorClient,
	apiServerClient configv1client.APIServerInterface,
	apiServerInformer configv1informers.APIServerInformer,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	secretClient corev1client.SecretsGetter,
	encryptionSecretSelector metav1.ListOptions,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &keyController{
		operatorClient:  operatorClient,
		apiServerClient: apiServerClient,

		component: component,
		name:      "EncryptionKeyController",

		encryptionSecretSelector: encryptionSecretSelector,
		deployer:                 deployer,
		provider:                 provider,
		secretClient:             secretClient,
	}

	return factory.New().
		WithSync(c.sync).
		ResyncEvery(time.Second). // TODO: Is the 1s resync really necessary?
		WithInformers(
			apiServerInformer.Informer(),
			operatorClient.Informer(),
			kubeInformersForNamespaces.InformersFor("openshift-config-managed").Core().V1().Secrets().Informer(),
			deployer,
		).ToController(c.name, eventRecorder.WithComponentSuffix("encryption-key-controller"))
}

func (c *keyController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	if ready, err := shouldRunEncryptionController(c.operatorClient, c.provider.ShouldRunEncryptionControllers); err != nil || !ready {
		return err // we will get re-kicked when the operator status updates
	}

	configError := c.checkAndCreateKeys(syncCtx, c.provider.EncryptedGRs())

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   "EncryptionKeyControllerDegraded",
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

func (c *keyController) checkAndCreateKeys(syncContext factory.SyncContext, encryptedGRs []schema.GroupResource) error {
	currentMode, externalReason, err := c.getCurrentModeAndExternalReason()
	if err != nil {
		return err
	}

	currentConfig, desiredEncryptionState, secrets, isProgressingReason, err := statemachine.GetEncryptionConfigAndState(c.deployer, c.secretClient, c.encryptionSecretSelector, encryptedGRs)
	if err != nil {
		return err
	}
	if len(isProgressingReason) > 0 {
		syncContext.Queue().AddAfter(syncContext.QueueKey(), 2*time.Minute)
		return nil
	}

	// avoid intended start of encryption
	hasBeenOnBefore := currentConfig != nil || len(secrets) > 0
	if currentMode == state.Identity && !hasBeenOnBefore {
		return nil
	}

	var (
		newKeyRequired bool
		newKeyID       uint64
		reasons        []string
	)

	// note here that desiredEncryptionState is never empty because getDesiredEncryptionState
	// fills up the state with all resources and set identity write key if write key secrets
	// are missing.

	var commonReason *string
	for gr, grKeys := range desiredEncryptionState {
		latestKeyID, internalReason, needed := needsNewKey(grKeys, currentMode, externalReason, encryptedGRs)
		if !needed {
			continue
		}

		if commonReason == nil {
			commonReason = &internalReason
		} else if *commonReason != internalReason {
			commonReason = pointer.StringPtr("") // this means we have no common reason
		}

		newKeyRequired = true
		nextKeyID := latestKeyID + 1
		if newKeyID < nextKeyID {
			newKeyID = nextKeyID
		}
		reasons = append(reasons, fmt.Sprintf("%s-%s", gr.Resource, internalReason))
	}
	if !newKeyRequired {
		return nil
	}
	if commonReason != nil && len(*commonReason) > 0 && len(reasons) > 1 {
		reasons = []string{*commonReason} // don't repeat reasons
	}

	sort.Sort(sort.StringSlice(reasons))
	internalReason := strings.Join(reasons, ", ")
	keySecret, err := c.generateKeySecret(newKeyID, currentMode, internalReason, externalReason)
	if err != nil {
		return fmt.Errorf("failed to create key: %v", err)
	}
	_, createErr := c.secretClient.Secrets("openshift-config-managed").Create(context.TODO(), keySecret, metav1.CreateOptions{})
	if errors.IsAlreadyExists(createErr) {
		return c.validateExistingSecret(keySecret, newKeyID)
	}
	if createErr != nil {
		syncContext.Recorder().Warningf("EncryptionKeyCreateFailed", "Secret %q failed to create: %v", keySecret.Name, err)
		return createErr
	}

	syncContext.Recorder().Eventf("EncryptionKeyCreated", "Secret %q successfully created: %q", keySecret.Name, reasons)

	return nil
}

func (c *keyController) validateExistingSecret(keySecret *corev1.Secret, keyID uint64) error {
	actualKeySecret, err := c.secretClient.Secrets("openshift-config-managed").Get(context.TODO(), keySecret.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	actualKeyID, ok := state.NameToKeyID(actualKeySecret.Name)
	if !ok || actualKeyID != keyID {
		// TODO we can just get stuck in degraded here ...
		return fmt.Errorf("secret %s has an invalid name, new keys cannot be created for encryption target", keySecret.Name)
	}

	if _, err := secrets.ToKeyState(actualKeySecret); err != nil {
		return fmt.Errorf("secret %s is invalid, new keys cannot be created for encryption target", keySecret.Name)
	}

	return nil // we made this key earlier
}

func (c *keyController) generateKeySecret(keyID uint64, currentMode state.Mode, internalReason, externalReason string) (*corev1.Secret, error) {
	bs := crypto.ModeToNewKeyFunc[currentMode]()
	ks := state.KeyState{
		Key: apiserverv1.Key{
			Name:   fmt.Sprintf("%d", keyID),
			Secret: base64.StdEncoding.EncodeToString(bs),
		},
		Mode:           currentMode,
		InternalReason: internalReason,
		ExternalReason: externalReason,
	}
	return secrets.FromKeyState(c.component, ks)
}

func (c *keyController) getCurrentModeAndExternalReason() (state.Mode, string, error) {
	apiServer, err := c.apiServerClient.Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	operatorSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return "", "", err
	}

	// TODO make this un-settable once set
	// ex: we could require the tech preview no upgrade flag to be set before we will honor this field
	type unsupportedEncryptionConfig struct {
		Encryption struct {
			Reason string `json:"reason"`
		} `json:"encryption"`
	}
	encryptionConfig := &unsupportedEncryptionConfig{}
	if raw := operatorSpec.UnsupportedConfigOverrides.Raw; len(raw) > 0 {
		jsonRaw, err := kyaml.ToJSON(raw)
		if err != nil {
			klog.Warning(err)
			// maybe it's just json
			jsonRaw = raw
		}
		if err := json.Unmarshal(jsonRaw, encryptionConfig); err != nil {
			return "", "", err
		}
	}

	reason := encryptionConfig.Encryption.Reason
	switch currentMode := state.Mode(apiServer.Spec.Encryption.Type); currentMode {
	case state.AESCBC, state.Identity: // secretbox is disabled for now
		return currentMode, reason, nil
	case "": // unspecified means use the default (which can change over time)
		return state.DefaultMode, reason, nil
	default:
		return "", "", fmt.Errorf("unknown encryption mode configured: %s", currentMode)
	}
}

// needsNewKey checks whether a new key must be created for the given resource. If true, it also returns the latest
// used key ID and a reason string.
func needsNewKey(grKeys state.GroupResourceState, currentMode state.Mode, externalReason string, encryptedGRs []schema.GroupResource) (uint64, string, bool) {
	// we always need to have some encryption keys unless we are turned off
	if len(grKeys.ReadKeys) == 0 {
		return 0, "key-does-not-exist", currentMode != state.Identity
	}

	latestKey := grKeys.ReadKeys[0]
	latestKeyID, ok := state.NameToKeyID(latestKey.Key.Name)
	if !ok {
		return latestKeyID, fmt.Sprintf("key-secret-%d-is-invalid", latestKeyID), true
	}

	// if latest secret has been deleted, we will never be able to migrate to that key.
	if !latestKey.Backed {
		return latestKeyID, fmt.Sprintf("encryption-config-key-%d-not-backed-by-secret", latestKeyID), true
	}

	// check that we have pruned read-keys: the write-keys, plus at most one more backed read-key (potentially some unbacked once before)
	backedKeys := 0
	for _, rk := range grKeys.ReadKeys {
		if rk.Backed {
			backedKeys++
		}
	}
	if backedKeys > 2 {
		return 0, "", false
	}

	// we have not migrated the latest key, do nothing until that is complete
	if allMigrated, _, _ := state.MigratedFor(encryptedGRs, latestKey); !allMigrated {
		return 0, "", false
	}

	// if the most recent secret was encrypted in a mode different than the current mode, we need to generate a new key
	if latestKey.Mode != currentMode {
		return latestKeyID, "encryption-mode-changed", true
	}

	// if the most recent secret turned off encryption and we want to keep it that way, do nothing
	if latestKey.Mode == state.Identity && currentMode == state.Identity {
		return 0, "", false
	}

	// if the most recent secret has a different external reason than the current reason, we need to generate a new key
	if latestKey.ExternalReason != externalReason && len(externalReason) != 0 {
		return latestKeyID, "external-reason-changed", true
	}

	// we check for encryptionSecretMigratedTimestamp set by migration controller to determine when migration completed
	// this also generates back pressure for key rotation when migration takes a long time or was recently completed
	return latestKeyID, "rotation-interval-has-passed", time.Since(latestKey.Migrated.Timestamp) > encryptionSecretMigrationInterval
}
