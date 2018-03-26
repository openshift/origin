package resourceapply

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func ApplyNamespace(client coreclientv1.NamespacesGetter, required *corev1.Namespace) (bool, error) {
	existing, err := client.Namespaces().Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	needsCreate := apierrors.IsNotFound(err)

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	if !*modified {
		return false, nil
	}
	if needsCreate {
		_, err := client.Namespaces().Create(existing)
		return true, err
	}

	_, err = client.Namespaces().Update(existing)
	return true, err
}

func ApplyService(client coreclientv1.ServicesGetter, required *corev1.Service) (bool, error) {
	existing, err := client.Services(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	needsCreate := apierrors.IsNotFound(err)

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureService(modified, existing, *required)

	if !*modified {
		return false, nil
	}
	if needsCreate {
		_, err := client.Services(required.Namespace).Create(existing)
		return true, err
	}

	_, err = client.Services(required.Namespace).Update(existing)
	return true, err
}

func ApplyServiceAccount(client coreclientv1.ServiceAccountsGetter, required *corev1.ServiceAccount) (bool, error) {
	existing, err := client.ServiceAccounts(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	needsCreate := apierrors.IsNotFound(err)

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	if !*modified {
		return false, nil
	}
	if needsCreate {
		_, err := client.ServiceAccounts(required.Namespace).Create(existing)
		return true, err
	}

	_, err = client.ServiceAccounts(required.Namespace).Update(existing)
	return true, err
}

func ApplyConfigMapForResourceVersion(client coreclientv1.ConfigMapsGetter, resourceVersion string, required *corev1.ConfigMap) (bool, error) {
	existing, err := client.ConfigMaps(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	// for configmaps, we have usually based our new configmap on an existing one (merged configs), so we have to ensure valid preconditions
	if existing.ResourceVersion != resourceVersion {
		return false, fmt.Errorf("resourceVersion doesn't match; have %q, need %q", existing.ResourceVersion, resourceVersion)
	}
	needsCreate := apierrors.IsNotFound(err)

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureConfigMap(modified, existing, *required)

	if !*modified {
		return false, nil
	}
	if needsCreate {
		_, err := client.ConfigMaps(required.Namespace).Create(existing)
		return true, err
	}

	_, err = client.ConfigMaps(required.Namespace).Update(existing)
	return true, err
}
