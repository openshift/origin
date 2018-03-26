package resourceapply

import (
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func ApplyDaemonSet(client appsclientv1.DaemonSetsGetter, required *appsv1.DaemonSet) (bool, error) {
	existing, err := client.DaemonSets(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	needsCreate := apierrors.IsNotFound(err)

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureDaemonSet(modified, existing, *required)

	if !*modified {
		return false, nil
	}
	if needsCreate {
		_, err := client.DaemonSets(required.Namespace).Create(existing)
		return true, err
	}

	_, err = client.DaemonSets(required.Namespace).Update(existing)
	return true, err
}

func ApplyDeployment(client appsclientv1.DeploymentsGetter, required *appsv1.Deployment) (bool, error) {
	existing, err := client.Deployments(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	needsCreate := apierrors.IsNotFound(err)

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureDeployment(modified, existing, *required)

	if !*modified {
		return false, nil
	}
	if needsCreate {
		_, err := client.Deployments(required.Namespace).Create(existing)
		return true, err
	}

	_, err = client.Deployments(required.Namespace).Update(existing)
	return true, err
}
