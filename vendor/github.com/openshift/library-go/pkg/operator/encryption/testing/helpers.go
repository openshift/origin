package testing

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	clientgotesting "k8s.io/client-go/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	encryptionSecretKeyDataForTest           = "encryption.apiserver.operator.openshift.io-key"
	encryptionSecretMigratedTimestampForTest = "encryption.apiserver.operator.openshift.io/migrated-timestamp"
	encryptionSecretMigratedResourcesForTest = "encryption.apiserver.operator.openshift.io/migrated-resources"
)

func CreateEncryptionKeySecretNoData(targetNS string, grs []schema.GroupResource, keyID uint64) *corev1.Secret {
	return CreateEncryptionKeySecretNoDataWithMode(targetNS, grs, keyID, "aescbc")
}

func CreateEncryptionKeySecretNoDataWithMode(targetNS string, grs []schema.GroupResource, keyID uint64, mode string) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("encryption-key-%s-%d", targetNS, keyID),
			Namespace: "openshift-config-managed",
			Annotations: map[string]string{
				state.KubernetesDescriptionKey: state.KubernetesDescriptionScaryValue,

				"encryption.apiserver.operator.openshift.io/mode":            mode,
				"encryption.apiserver.operator.openshift.io/internal-reason": "no-secrets",
				"encryption.apiserver.operator.openshift.io/external-reason": "",
			},
			Labels: map[string]string{
				"encryption.apiserver.operator.openshift.io/component": targetNS,
			},
			Finalizers: []string{"encryption.apiserver.operator.openshift.io/deletion-protection"},
		},
		Data: map[string][]byte{},
	}

	if len(grs) > 0 {
		migratedResourceBytes, err := json.Marshal(secrets.MigratedGroupResources{Resources: grs})
		if err != nil {
			panic(err)
		}
		s.Annotations[encryptionSecretMigratedResourcesForTest] = string(migratedResourceBytes)
	}

	return s
}

func CreateEncryptionKeySecretWithRawKey(targetNS string, grs []schema.GroupResource, keyID uint64, rawKey []byte) *corev1.Secret {
	return CreateEncryptionKeySecretWithRawKeyWithMode(targetNS, grs, keyID, rawKey, "aescbc")
}

func CreateEncryptionKeySecretWithRawKeyWithMode(targetNS string, grs []schema.GroupResource, keyID uint64, rawKey []byte, mode string) *corev1.Secret {
	secret := CreateEncryptionKeySecretNoDataWithMode(targetNS, grs, keyID, mode)
	secret.Data[encryptionSecretKeyDataForTest] = rawKey
	return secret
}

func CreateEncryptionKeySecretWithKeyFromExistingSecret(targetNS string, grs []schema.GroupResource, keyID uint64, existingSecret *corev1.Secret) *corev1.Secret {
	secret := CreateEncryptionKeySecretNoData(targetNS, grs, keyID)
	if rawKey, exist := existingSecret.Data[encryptionSecretKeyDataForTest]; exist {
		secret.Data[encryptionSecretKeyDataForTest] = rawKey
	}
	return secret
}

func CreateMigratedEncryptionKeySecretWithRawKey(targetNS string, grs []schema.GroupResource, keyID uint64, rawKey []byte, ts time.Time) *corev1.Secret {
	secret := CreateEncryptionKeySecretWithRawKey(targetNS, grs, keyID, rawKey)
	secret.Annotations[encryptionSecretMigratedTimestampForTest] = ts.Format(time.RFC3339)
	return secret
}

func CreateExpiredMigratedEncryptionKeySecretWithRawKey(targetNS string, grs []schema.GroupResource, keyID uint64, rawKey []byte) *corev1.Secret {
	return CreateMigratedEncryptionKeySecretWithRawKey(targetNS, grs, keyID, rawKey, time.Now().Add(-(time.Hour*24*7 + time.Hour)))
}

func CreateDummyKubeAPIPod(name, namespace string, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"apiserver": "true",
				"revision":  "1",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func CreateDummyKubeAPIPodInUnknownPhase(name, namespace string, nodeName string) *corev1.Pod {
	p := CreateDummyKubeAPIPod(name, namespace, nodeName)
	p.Status.Phase = corev1.PodUnknown
	return p
}

func ValidateActionsVerbs(actualActions []clientgotesting.Action, expectedActions []string) error {
	if len(actualActions) != len(expectedActions) {
		return fmt.Errorf("expected to get %d actions but got %d, expected=%v, got=%v", len(expectedActions), len(actualActions), expectedActions, actionStrings(actualActions))
	}
	for i, a := range actualActions {
		if got, expected := actionString(a), expectedActions[i]; got != expected {
			return fmt.Errorf("at %d got %s, expected %s", i, got, expected)
		}
	}
	return nil
}

func actionString(a clientgotesting.Action) string {
	return a.GetVerb() + ":" + a.GetResource().Resource + ":" + a.GetNamespace()
}

func actionStrings(actions []clientgotesting.Action) []string {
	res := make([]string, 0, len(actions))
	for _, a := range actions {
		res = append(res, actionString(a))
	}
	return res
}

func CreateEncryptionCfgNoWriteKey(keyID string, keyBase64 string, resources ...string) *apiserverconfigv1.EncryptionConfiguration {
	keysResources := []EncryptionKeysResourceTuple{}
	for _, resource := range resources {
		keysResources = append(keysResources, EncryptionKeysResourceTuple{
			Resource: resource,
			Keys: []apiserverconfigv1.Key{
				{Name: keyID, Secret: keyBase64},
			},
		})

	}
	return CreateEncryptionCfgNoWriteKeyMultipleReadKeys(keysResources)
}

func CreateEncryptionCfgNoWriteKeyMultipleReadKeys(keysResources []EncryptionKeysResourceTuple) *apiserverconfigv1.EncryptionConfiguration {
	ec := &apiserverconfigv1.EncryptionConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EncryptionConfiguration",
			APIVersion: "apiserver.config.k8s.io/v1",
		},
		Resources: []apiserverconfigv1.ResourceConfiguration{},
	}

	for _, keysResource := range keysResources {
		rc := apiserverconfigv1.ResourceConfiguration{
			Resources: []string{keysResource.Resource},
			Providers: []apiserverconfigv1.ProviderConfiguration{
				{
					Identity: &apiserverconfigv1.IdentityConfiguration{},
				},
			},
		}
		for i, key := range keysResource.Keys {
			desiredMode := ""
			if len(keysResource.Modes) == len(keysResource.Keys) {
				desiredMode = keysResource.Modes[i]
			}
			switch desiredMode {
			case "aesgcm":
				rc.Providers = append(rc.Providers, apiserverconfigv1.ProviderConfiguration{
					AESGCM: &apiserverconfigv1.AESConfiguration{
						Keys: []apiserverconfigv1.Key{key},
					},
				})
			default:
				rc.Providers = append(rc.Providers, apiserverconfigv1.ProviderConfiguration{
					AESCBC: &apiserverconfigv1.AESConfiguration{
						Keys: []apiserverconfigv1.Key{key},
					},
				})
			}
		}
		ec.Resources = append(ec.Resources, rc)
	}

	return ec
}

func CreateEncryptionCfgWithWriteKey(keysResources []EncryptionKeysResourceTuple) *apiserverconfigv1.EncryptionConfiguration {
	configurations := []apiserverconfigv1.ResourceConfiguration{}
	for _, keysResource := range keysResources {
		// TODO allow secretbox -> not sure if EncryptionKeysResourceTuple makes sense
		providers := []apiserverconfigv1.ProviderConfiguration{}
		for _, key := range keysResource.Keys {
			providers = append(providers, apiserverconfigv1.ProviderConfiguration{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: []apiserverconfigv1.Key{key},
				},
			})
		}
		providers = append(providers, apiserverconfigv1.ProviderConfiguration{
			Identity: &apiserverconfigv1.IdentityConfiguration{},
		})

		configurations = append(configurations, apiserverconfigv1.ResourceConfiguration{
			Resources: []string{keysResource.Resource},
			Providers: providers,
		})
	}

	return &apiserverconfigv1.EncryptionConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EncryptionConfiguration",
			APIVersion: "apiserver.config.k8s.io/v1",
		},
		Resources: configurations,
	}
}

type EncryptionKeysResourceTuple struct {
	Resource string
	Keys     []apiserverconfigv1.Key
	// an ordered list of an encryption modes thatch matches the keys
	// for example mode[0] matches keys[0]
	Modes []string
}

func ValidateOperatorClientConditions(ts *testing.T, operatorClient v1helpers.OperatorClient, expectedConditions []operatorv1.OperatorCondition) {
	ts.Helper()
	_, status, _, err := operatorClient.GetOperatorState()
	if err != nil {
		ts.Fatal(err)
	}

	if len(status.Conditions) != len(expectedConditions) {
		ts.Fatalf("expected to get %d conditions from operator client but got %d:\n\nexpected=%v\n\ngot=%v", len(expectedConditions), len(status.Conditions), expectedConditions, status.Conditions)
	}

	for _, actualCondition := range status.Conditions {
		actualConditionValidated := false
		for _, expectedCondition := range expectedConditions {
			expectedCondition.LastTransitionTime = actualCondition.LastTransitionTime
			if equality.Semantic.DeepEqual(expectedCondition, actualCondition) {
				actualConditionValidated = true
				break
			}
		}
		if !actualConditionValidated {
			ts.Fatalf("unexpected condition found %#v", actualCondition)
		}

	}
}

func ValidateEncryptionKey(secret *corev1.Secret) error {
	rawKey, exist := secret.Data[encryptionSecretKeyDataForTest]
	if !exist {
		return errors.New("the secret doesn't contain an encryption key")
	}
	if len(rawKey) != 32 {
		return fmt.Errorf("incorrect length of the encryption key, expected 32, got %d bytes", len(rawKey))
	}
	return nil
}
