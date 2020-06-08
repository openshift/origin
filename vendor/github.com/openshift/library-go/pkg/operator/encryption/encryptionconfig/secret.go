package encryptionconfig

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"

	"github.com/openshift/library-go/pkg/operator/encryption/state"
)

var (
	apiserverScheme = runtime.NewScheme()
	apiserverCodecs = serializer.NewCodecFactory(apiserverScheme)
)

func init() {
	utilruntime.Must(apiserverconfigv1.AddToScheme(apiserverScheme))
}

// EncryptionConfSecretName is the name of the final encryption config secret that is revisioned per apiserver rollout.
const EncryptionConfSecretName = "encryption-config"

// EncryptionConfSecretKey is the map data key used to store the raw bytes of the final encryption config.
const EncryptionConfSecretKey = "encryption-config"

func FromSecret(encryptionConfigSecret *corev1.Secret) (*apiserverconfigv1.EncryptionConfiguration, error) {
	data, ok := encryptionConfigSecret.Data[EncryptionConfSecretKey]
	if !ok {
		return nil, nil
	}

	decoder := apiserverCodecs.UniversalDecoder(apiserverconfigv1.SchemeGroupVersion)
	encryptionConfigObj, err := runtime.Decode(decoder, data)
	if err != nil {
		return nil, err
	}

	encryptionConfig, ok := encryptionConfigObj.(*apiserverconfigv1.EncryptionConfiguration)
	if !ok {
		return nil, fmt.Errorf("unexpected wrong type %T", encryptionConfigObj)
	}
	return encryptionConfig, nil
}

func ToSecret(ns, name string, encryptionCfg *apiserverconfigv1.EncryptionConfiguration) (*corev1.Secret, error) {
	encoder := apiserverCodecs.LegacyCodec(apiserverconfigv1.SchemeGroupVersion)
	rawEncryptionCfg, err := runtime.Encode(encoder, encryptionCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the encryption config: %v", err)
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Annotations: map[string]string{
				state.KubernetesDescriptionKey: state.KubernetesDescriptionScaryValue,
			},
			Finalizers: []string{"encryption.apiserver.operator.openshift.io/deletion-protection"},
		},
		Data: map[string][]byte{
			EncryptionConfSecretName: rawEncryptionCfg,
		},
		Type: corev1.SecretTypeOpaque,
	}, nil
}
