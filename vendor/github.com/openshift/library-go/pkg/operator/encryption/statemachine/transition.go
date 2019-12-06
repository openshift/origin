package statemachine

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
)

// Deployer abstracts the deployment machanism like the static pod controllers.
type Deployer interface {
	// DeployedEncryptionConfigSecret returns the deployed encryption config and whether all
	// instances of the operand have acknowledged it.
	DeployedEncryptionConfigSecret() (secret *corev1.Secret, converged bool, err error)

	// AddEventHandler registers a event handler whenever the backing resource change
	// that might influence the result of DeployedEncryptionConfigSecret.
	AddEventHandler(handler cache.ResourceEventHandler) []cache.InformerSynced
}

func GetEncryptionConfigAndState(
	deployer Deployer,
	secretClient corev1client.SecretsGetter,
	encryptionSecretSelector metav1.ListOptions,
	encryptedGRs []schema.GroupResource,
) (current *apiserverconfigv1.EncryptionConfiguration, desired map[schema.GroupResource]state.GroupResourceState, encryptionSecrets []*corev1.Secret, transitioningReason string, err error) {
	// get current config
	encryptionConfigSecret, converged, err := deployer.DeployedEncryptionConfigSecret()
	if err != nil {
		return nil, nil, nil, "", err
	}
	if !converged {
		return nil, nil, nil, "APIServerRevisionNotConverged", nil
	}
	var encryptionConfig *apiserverconfigv1.EncryptionConfiguration
	if encryptionConfigSecret != nil {
		encryptionConfig, err = encryptionconfig.FromSecret(encryptionConfigSecret)
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("invalid encryption config %s/%s: %v", encryptionConfigSecret.Namespace, encryptionConfigSecret.Name, err)
		}
	}

	// compute desired config
	encryptionSecrets, err = secrets.ListKeySecrets(secretClient, encryptionSecretSelector)
	if err != nil {
		return nil, nil, nil, "", err
	}
	desiredEncryptionState := getDesiredEncryptionState(encryptionConfig, encryptionSecrets, encryptedGRs)

	return encryptionConfig, desiredEncryptionState, encryptionSecrets, "", nil
}

// getDesiredEncryptionState returns the desired state of encryption for all resources.
// To do this it compares the current state against the available secrets and to-be-encrypted resources.
// oldEncryptionConfig can be nil if there is no config yet.
// If there are no secrets, the identity is set for all resources as write key.
// It is assumed that encryptionSecrets are all valid.
//
// The basic rules are:
//
// 1. don't do anything if there are key secrets.
// 2. every GR must have all the read-keys (existing as secrets) since last complete migration.
// 3. if (2) is the case, the write-key must be the most recent key.
// 4. if (2) and (3) are the case, all non-write keys should be removed.
func getDesiredEncryptionState(oldEncryptionConfig *apiserverconfigv1.EncryptionConfiguration, encryptionSecrets []*corev1.Secret, toBeEncryptedGRs []schema.GroupResource) map[schema.GroupResource]state.GroupResourceState {
	//
	// STEP 0: start with old encryption config, and alter it towards the desired state in the following STEPs.
	//
	desiredEncryptionState, backedKeys := encryptionconfig.ToEncryptionState(oldEncryptionConfig, encryptionSecrets)
	if desiredEncryptionState == nil {
		desiredEncryptionState = make(map[schema.GroupResource]state.GroupResourceState, len(toBeEncryptedGRs))
	}

	// add new resources without keys. These resources will trigger STEP 2.
	oldEncryptedGRs := make([]schema.GroupResource, 0, len(desiredEncryptionState))
	for _, gr := range toBeEncryptedGRs {
		if _, ok := desiredEncryptionState[gr]; !ok {
			desiredEncryptionState[gr] = state.GroupResourceState{}
		} else {
			oldEncryptedGRs = append(oldEncryptedGRs, gr)
		}
	}

	//
	// STEP 1: without secrets, wait for the key controller to create one
	//
	// the code after this point assumes at least one secret
	if len(backedKeys) == 0 {
		klog.V(4).Infof("no encryption secrets found")
		return desiredEncryptionState
	}

	//
	// STEP 2: verify to have all necessary read-keys. If not, add them, deploy and wait for stability.
	//
	// Note: we never drop keys here. Dropping only happens in STEP 4.
	// Note: only keysWithPotentiallyPersistedData are considered. There might be more which are not pruned yet by the pruning controller.
	//
	// TODO: allow removing resources (e.g. on downgrades) and transition back to identity.
	allReadSecretsAsExpected := true
	currentlyEncryptedGRs := oldEncryptedGRs
	if oldEncryptionConfig == nil {
		// if the config is not there, we assume it was deleted. Assume worst case when finding
		// potentially persisted data keys.
		currentlyEncryptedGRs = toBeEncryptedGRs
	}
	expectedReadSecrets := state.KeysWithPotentiallyPersistedDataAndNextReadKey(currentlyEncryptedGRs, backedKeys)
	for gr, grState := range desiredEncryptionState {
		changed := false
		for _, expected := range expectedReadSecrets {
			found := false
			for _, rk := range grState.ReadKeys {
				if state.EqualKeyAndEqualID(&rk, &expected) {
					found = true
					break
				}
			}
			if !found {
				// Just adding raw key without trusting any metadata on it
				grState.ReadKeys = state.SortRecentFirst(append(grState.ReadKeys, expected)) // sort into right position
				changed = true
				allReadSecretsAsExpected = false
				klog.V(4).Infof("encrypted resource %s misses read key %s", gr, expected.Key.Name)
			}
		}
		if changed {
			grState.ReadKeys = state.SortRecentFirst(grState.ReadKeys)
			desiredEncryptionState[gr] = grState
		}

		// potential write-key must be backed. Otherwise stop here in STEP 2 and let key controller create a new key.
		if !grState.ReadKeys[0].Backed {
			allReadSecretsAsExpected = false
		}
	}
	if !allReadSecretsAsExpected {
		klog.V(4).Infof("not all read secrets in sync")
		return desiredEncryptionState
	}

	//
	// STEP 3: with consistent read-keys, verify first read-key is write-key. If not, set write-key and wait for stability.
	//
	writeKey := backedKeys[0]
	allWriteSecretsAsExpected := true
	for gr, grState := range desiredEncryptionState {
		if !grState.HasWriteKey() || !state.EqualKeyAndEqualID(&grState.WriteKey, &writeKey) {
			allWriteSecretsAsExpected = false
			klog.V(4).Infof("encrypted resource %s does not have write key %s", gr, writeKey.Key.Name)
			break
		}
	}
	if !allWriteSecretsAsExpected {
		klog.V(4).Infof("not all write secrets in sync")
		for gr := range desiredEncryptionState {
			grState := desiredEncryptionState[gr]
			grState.WriteKey = writeKey
			desiredEncryptionState[gr] = grState
		}
		return desiredEncryptionState
	}

	//
	// STEP 4: with consistent read-keys and write-keys, remove every read-key other than the write-key and one last read key.
	//
	// Note: because read-keys are consistent, currentlyEncryptedGRs equals toBeEncryptedGRs
	allMigrated, _, reason := state.MigratedFor(currentlyEncryptedGRs, writeKey)
	if !allMigrated {
		klog.V(4).Infof(reason)
		return desiredEncryptionState
	}
	for gr := range desiredEncryptionState {
		grState := desiredEncryptionState[gr]

		// cut down read keys to all expected read keys, and everything in between
		if len(expectedReadSecrets) == 0 {
			grState.ReadKeys = []state.KeyState{}
		} else {
			lastExpected := expectedReadSecrets[len(expectedReadSecrets)-1]
			for i, rk := range grState.ReadKeys {
				if state.EqualKeyAndEqualID(&rk, &lastExpected) {
					grState.ReadKeys = grState.ReadKeys[:i+1]
					break
				}
			}
		}

		desiredEncryptionState[gr] = grState
	}
	klog.V(4).Infof("write key %s set as sole write key", writeKey.Key.Name)
	return desiredEncryptionState
}
