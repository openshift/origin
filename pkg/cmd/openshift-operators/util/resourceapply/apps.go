package resourceapply

import (
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
)

// ApplyDeployment merges objectmeta and requires matching generation
func ApplyDeployment(client appsclientv1.DeploymentsGetter, required *appsv1.Deployment, expectedGeneration int64, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	existing, err := client.Deployments(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Deployments(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if forceDeployment {
		// forces a deployment
		forceString := string(uuid.NewUUID())
		if required.Annotations == nil {
			required.Annotations = map[string]string{}
		}
		if required.Spec.Template.Annotations == nil {
			required.Spec.Template.Annotations = map[string]string{}
		}
		required.Annotations["operator.openshift.io/force"] = forceString
		required.Spec.Template.Annotations["operator.openshift.io/force"] = forceString
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if *modified {
		actual, err := client.Deployments(required.Namespace).Update(existing)
		return actual, true, err
	}
	if existing.ObjectMeta.Generation == expectedGeneration {
		return existing, false, nil
	}
	existing.Spec = *required.Spec.DeepCopy()

	actual, err := client.Deployments(required.Namespace).Update(existing)
	return actual, true, err
}
