package resourcehash

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/listers/core/v1"
)

// GetConfigMapHash returns a hash of the configmap data
func GetConfigMapHash(obj *corev1.ConfigMap) (string, error) {
	hasher := fnv.New32()
	if err := json.NewEncoder(hasher).Encode(obj.Data); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil)), nil
}

// GetSecretHash returns a hash of the secret data
func GetSecretHash(obj *corev1.Secret) (string, error) {
	hasher := fnv.New32()
	if err := json.NewEncoder(hasher).Encode(obj.Data); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil)), nil
}

// MultipleObjectHashStringMap returns a map of key/hash pairs suitable for merging into a configmap
func MultipleObjectHashStringMap(objs ...runtime.Object) (map[string]string, error) {
	ret := map[string]string{}

	for _, obj := range objs {
		switch t := obj.(type) {
		case *corev1.ConfigMap:
			hash, err := GetConfigMapHash(t)
			if err != nil {
				return nil, err
			}
			// this string coercion is lossy, but it should be fairly controlled and must be an allowed name
			ret[mapKeyFor("configmap", t.Namespace, t.Name)] = hash

		case *corev1.Secret:
			hash, err := GetSecretHash(t)
			if err != nil {
				return nil, err
			}
			// this string coercion is lossy, but it should be fairly controlled and must be an allowed name
			ret[mapKeyFor("secret", t.Namespace, t.Name)] = hash

		default:
			return nil, fmt.Errorf("%T is not handled", t)
		}
	}

	return ret, nil
}

func mapKeyFor(resource, namespace, name string) string {
	return fmt.Sprintf("%s.%s.%s", namespace, name, resource)
}

// ObjectReference can be used to reference a particular resource.  Not all group resources are respected by all methods.
type ObjectReference struct {
	Resource  schema.GroupResource
	Namespace string
	Name      string
}

// MultipleObjectHashStringMapForObjectReferences returns a map of key/hash pairs suitable for merging into a configmap
func MultipleObjectHashStringMapForObjectReferences(client kubernetes.Interface, objRefs ...*ObjectReference) (map[string]string, error) {
	objs := []runtime.Object{}

	for _, objRef := range objRefs {
		switch objRef.Resource {
		case schema.GroupResource{Resource: "configmap"}, schema.GroupResource{Resource: "configmaps"}:
			obj, err := client.CoreV1().ConfigMaps(objRef.Namespace).Get(objRef.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				// don't error, just don't list the key. this is different than empty
				continue
			}
			if err != nil {
				return nil, err
			}
			objs = append(objs, obj)

		case schema.GroupResource{Resource: "secret"}, schema.GroupResource{Resource: "secrets"}:
			obj, err := client.CoreV1().Secrets(objRef.Namespace).Get(objRef.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				// don't error, just don't list the key. this is different than empty
				continue
			}
			if err != nil {
				return nil, err
			}
			objs = append(objs, obj)

		default:
			return nil, fmt.Errorf("%v is not handled", objRef.Resource)
		}
	}

	return MultipleObjectHashStringMap(objs...)
}

// MultipleObjectHashStringMapForObjectReferenceFromLister is MultipleObjectHashStringMapForObjectReferences using a lister for performance
func MultipleObjectHashStringMapForObjectReferenceFromLister(configmapLister v1.ConfigMapLister, secretLister v1.SecretLister, objRefs ...*ObjectReference) (map[string]string, error) {
	objs := []runtime.Object{}

	for _, objRef := range objRefs {
		switch objRef.Resource {
		case schema.GroupResource{Resource: "configmap"}, schema.GroupResource{Resource: "configmaps"}:
			obj, err := configmapLister.ConfigMaps(objRef.Namespace).Get(objRef.Name)
			if apierrors.IsNotFound(err) {
				// don't error, just don't list the key. this is different than empty
				continue
			}
			if err != nil {
				return nil, err
			}
			objs = append(objs, obj)

		case schema.GroupResource{Resource: "secret"}, schema.GroupResource{Resource: "secrets"}:
			obj, err := secretLister.Secrets(objRef.Namespace).Get(objRef.Name)
			if apierrors.IsNotFound(err) {
				// don't error, just don't list the key. this is different than empty
				continue
			}
			if err != nil {
				return nil, err
			}
			objs = append(objs, obj)

		default:
			return nil, fmt.Errorf("%v is not handled", objRef.Resource)
		}
	}

	return MultipleObjectHashStringMap(objs...)
}

func NewObjectRef() *ObjectReference {
	return &ObjectReference{}
}

func (r *ObjectReference) ForConfigMap() *ObjectReference {
	r.Resource = schema.GroupResource{Resource: "configmaps"}
	return r
}

func (r *ObjectReference) ForSecret() *ObjectReference {
	r.Resource = schema.GroupResource{Resource: "secrets"}
	return r
}

func (r *ObjectReference) Named(name string) *ObjectReference {
	r.Name = name
	return r
}

func (r *ObjectReference) InNamespace(namespace string) *ObjectReference {
	r.Namespace = namespace
	return r
}
