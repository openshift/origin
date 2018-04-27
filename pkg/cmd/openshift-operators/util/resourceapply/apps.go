package resourceapply

import (
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
)

func ApplyDaemonSet(client appsclientv1.DaemonSetsGetter, required *appsv1.DaemonSet) (bool, error) {
	existing, err := client.DaemonSets(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := client.DaemonSets(required.Namespace).Create(required)
		return true, err
	}
	if err != nil {
		return false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureDaemonSet(modified, existing, *required)
	if !*modified {
		return false, nil
	}

	_, err = client.DaemonSets(required.Namespace).Update(existing)
	return true, err
}

func ApplyDeployment(client appsclientv1.DeploymentsGetter, required *appsv1.Deployment) (*appsv1.Deployment, bool, error) {
	existing, err := client.Deployments(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Deployments(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureDeployment(modified, existing, *required)
	if !*modified {
		return existing, false, nil
	}

	actual, err := client.Deployments(required.Namespace).Update(existing)
	return actual, true, err
}
