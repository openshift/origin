package resourceapply

import (
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
	"k8s.io/apimachinery/pkg/api/equality"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// ApplyNamespace merges objectmeta, does not worry about anything else
func ApplyNamespace(client coreclientv1.NamespacesGetter, required *corev1.Namespace) (bool, error) {
	existing, err := client.Namespaces().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := client.Namespaces().Create(required)
		return true, err
	}
	if err != nil {
		return false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return false, nil
	}

	_, err = client.Namespaces().Update(existing)
	return true, err
}

// ApplyService merges objectmeta and requires
// TODO, since this cannot determine whether changes are due to legitimate actors (api server) or illegitimate ones (users), we cannot update
// TODO I've special cased the selector for now
func ApplyService(client coreclientv1.ServicesGetter, required *corev1.Service) (bool, error) {
	existing, err := client.Services(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := client.Services(required.Namespace).Create(required)
		return true, err
	}
	if err != nil {
		return false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if *modified {
		_, err = client.Services(required.Namespace).Update(existing)
		return true, err
	}

	if equality.Semantic.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
		return false, nil
	}
	existing.Spec.Selector = required.Spec.Selector

	_, err = client.Services(required.Namespace).Update(existing)
	return true, err
}

// ApplyServiceAccount merges objectmeta, does not worry about anything else
func ApplyServiceAccount(client coreclientv1.ServiceAccountsGetter, required *corev1.ServiceAccount) (bool, error) {
	existing, err := client.ServiceAccounts(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := client.ServiceAccounts(required.Namespace).Create(required)
		return true, err
	}
	if err != nil {
		return false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return false, nil
	}

	_, err = client.ServiceAccounts(required.Namespace).Update(existing)
	return true, err
}

// ApplyConfigMap merges objectmeta, requires data
func ApplyConfigMap(client coreclientv1.ConfigMapsGetter, required *corev1.ConfigMap) (bool, error) {
	existing, err := client.ConfigMaps(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := client.ConfigMaps(required.Namespace).Create(required)
		return true, err
	}
	if err != nil {
		return false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return false, nil
	}
	if equality.Semantic.DeepEqual(existing.Data, required.Data) {
		return false, nil
	}
	existing.Data = required.Data

	_, err = client.ConfigMaps(required.Namespace).Update(existing)
	return true, err
}
