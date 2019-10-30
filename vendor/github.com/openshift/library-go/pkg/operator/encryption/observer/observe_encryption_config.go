package observer

import (
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1lister "k8s.io/client-go/listers/core/v1"

	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/events"
)

type SecretsListers interface {
	SecretLister() corev1lister.SecretLister
}

// NewEncryptionConfigObserver sets encryption-provider-config flag to /etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config
// in the configuration file if encryption-config in the targetNamespace is found
//
// note:
// the flag is not removed when the encryption-config was accidentally removed
// there is an active reconciliation loop in place that will eventually synchronize the missing resource
func NewEncryptionConfigObserver(targetNamespace string, encryptionConfFilePath string) configobserver.ObserveConfigFunc {
	return func(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
		encryptionConfigPath := []string{"apiServerArguments", "encryption-provider-config"}
		listers := genericListers.(SecretsListers)
		var errs []error
		previouslyObservedConfig := map[string]interface{}{}

		existingEncryptionConfig, _, err := unstructured.NestedStringSlice(existingConfig, encryptionConfigPath...)
		if err != nil {
			return previouslyObservedConfig, append(errs, err)
		}

		if len(existingEncryptionConfig) > 0 {
			if err := unstructured.SetNestedStringSlice(previouslyObservedConfig, existingEncryptionConfig, encryptionConfigPath...); err != nil {
				errs = append(errs, err)
			}
		}

		previousEncryptionConfigFound := len(existingEncryptionConfig) > 0
		observedConfig := map[string]interface{}{}

		encryptionConfigSecret, err := listers.SecretLister().Secrets(targetNamespace).Get(encryptionconfig.EncryptionConfSecretName)
		if errors.IsNotFound(err) {
			// warn only if the encryption-provider-config flag was set before
			if previousEncryptionConfigFound {
				recorder.Warningf("ObserveEncryptionConfigNotFound", "encryption config secret %s/%s not found after encryption has been enabled", targetNamespace, encryptionconfig.EncryptionConfSecretName)
			}
			// encryption secret is optional so it doesn't prevent apiserver from running
			// there is an active reconciliation loop in place that will eventually synchronize the missing resource
			return previouslyObservedConfig, errs // do not append the not found error
		}
		if err != nil {
			recorder.Warningf("ObserveEncryptionConfigGetErr", "failed to get encryption config secret %s/%s: %v", targetNamespace, encryptionconfig.EncryptionConfSecretName, err)
			return previouslyObservedConfig, append(errs, err)
		}
		if len(encryptionConfigSecret.Data[encryptionconfig.EncryptionConfSecretKey]) == 0 {
			recorder.Warningf("ObserveEncryptionConfigNoData", "encryption config secret %s/%s missing data", targetNamespace, encryptionconfig.EncryptionConfSecretName)
			return previouslyObservedConfig, errs
		}

		if err := unstructured.SetNestedStringSlice(observedConfig, []string{encryptionConfFilePath}, encryptionConfigPath...); err != nil {
			recorder.Warningf("ObserveEncryptionConfigFailedSet", "failed setting encryption config: %v", err)
			return previouslyObservedConfig, append(errs, err)
		}

		if !equality.Semantic.DeepEqual(existingEncryptionConfig, []string{encryptionConfFilePath}) {
			recorder.Eventf("ObserveEncryptionConfigChanged", "encryption config file changed from %s to %s", existingEncryptionConfig, encryptionConfFilePath)
		}

		return observedConfig, errs
	}
}
