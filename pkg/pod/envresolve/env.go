package envresolve

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/v1/resource"
	"k8s.io/kubernetes/pkg/fieldpath"
)

// ResourceStore defines a new resource store data structure
type ResourceStore struct {
	SecretStore    map[string]*corev1.Secret
	ConfigMapStore map[string]*corev1.ConfigMap
}

// NewResourceStore returns a pointer to a new resource store data structure
func NewResourceStore() *ResourceStore {
	return &ResourceStore{
		SecretStore:    make(map[string]*corev1.Secret),
		ConfigMapStore: make(map[string]*corev1.ConfigMap),
	}
}

// getSecretRefValue returns the value of a secret in the supplied namespace
func getSecretRefValue(client kubernetes.Interface, namespace string, store *ResourceStore, secretSelector *corev1.SecretKeySelector) (string, error) {
	secret, ok := store.SecretStore[secretSelector.Name]
	if !ok {
		var err error
		secret, err = client.CoreV1().Secrets(namespace).Get(secretSelector.Name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		store.SecretStore[secretSelector.Name] = secret
	}
	if data, ok := secret.Data[secretSelector.Key]; ok {
		return string(data), nil
	}
	return "", fmt.Errorf("key %s not found in secret %s", secretSelector.Key, secretSelector.Name)

}

// getConfigMapRefValue returns the value of a configmap in the supplied namespace
func getConfigMapRefValue(client kubernetes.Interface, namespace string, store *ResourceStore, configMapSelector *corev1.ConfigMapKeySelector) (string, error) {
	configMap, ok := store.ConfigMapStore[configMapSelector.Name]
	if !ok {
		var err error
		configMap, err = client.Core().ConfigMaps(namespace).Get(configMapSelector.Name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		store.ConfigMapStore[configMapSelector.Name] = configMap
	}
	if data, ok := configMap.Data[configMapSelector.Key]; ok {
		return string(data), nil
	}
	return "", fmt.Errorf("key %s not found in config map %s", configMapSelector.Key, configMapSelector.Name)
}

// getFieldRef returns the value of the supplied path in the given object
func getFieldRef(obj runtime.Object, from *corev1.EnvVarSource) (string, error) {
	return fieldpath.ExtractFieldPathAsString(obj, from.FieldRef.FieldPath)
}

// getResourceFieldRef returns the value of a resource in the given container
func getResourceFieldRef(from *corev1.EnvVarSource, c *corev1.Container) (string, error) {
	return resource.ExtractContainerResourceValue(from.ResourceFieldRef, c)
}

// GenEnvVarRefValue returns the value referenced by the supplied EnvVarSource given the other supplied information
func GetEnvVarRefValue(kc kubernetes.Interface, ns string, store *ResourceStore, from *corev1.EnvVarSource, obj runtime.Object, c *corev1.Container) (string, error) {
	if from.SecretKeyRef != nil {
		return getSecretRefValue(kc, ns, store, from.SecretKeyRef)
	}

	if from.ConfigMapKeyRef != nil {
		return getConfigMapRefValue(kc, ns, store, from.ConfigMapKeyRef)
	}

	if from.FieldRef != nil {
		return getFieldRef(obj, from)
	}

	if from.ResourceFieldRef != nil {
		return getResourceFieldRef(from, c)
	}

	return "", fmt.Errorf("invalid valueFrom")
}

// GenEnvVarRefString returns a text description of the supplied EnvVarSource
func GetEnvVarRefString(from *corev1.EnvVarSource) string {
	if from.ConfigMapKeyRef != nil {
		return fmt.Sprintf("configmap %s, key %s", from.ConfigMapKeyRef.Name, from.ConfigMapKeyRef.Key)
	}

	if from.SecretKeyRef != nil {
		return fmt.Sprintf("secret %s, key %s", from.SecretKeyRef.Name, from.SecretKeyRef.Key)
	}

	if from.FieldRef != nil {
		return fmt.Sprintf("field path %s", from.FieldRef.FieldPath)
	}

	if from.ResourceFieldRef != nil {
		containerPrefix := ""
		if from.ResourceFieldRef.ContainerName != "" {
			containerPrefix = fmt.Sprintf("%s/", from.ResourceFieldRef.ContainerName)
		}
		return fmt.Sprintf("resource field %s%s", containerPrefix, from.ResourceFieldRef.Resource)
	}

	return "invalid valueFrom"
}
