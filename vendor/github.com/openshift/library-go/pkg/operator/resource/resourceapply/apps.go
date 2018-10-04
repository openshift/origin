package resourceapply

import (
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyDeployment merges objectmeta and requires matching generation. It returns the final Object, whether any change as made, and an error
func ApplyDeployment(client appsclientv1.DeploymentsGetter, required *appsv1.Deployment, expectedGeneration int64, forceRollout bool) (*appsv1.Deployment, bool, error) {
	existing, err := client.Deployments(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Deployments(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	// there was no change to metadata, the generation was right, and we weren't asked for force the deployment
	if !*modified && existing.ObjectMeta.Generation == expectedGeneration && !forceRollout {
		return existing, false, nil
	}

	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existing // shallow copy so the code reads easier
	toWrite.Spec = *required.Spec.DeepCopy()
	if forceRollout {
		// forces a deployment
		forceString := string(uuid.NewUUID())
		if toWrite.Annotations == nil {
			toWrite.Annotations = map[string]string{}
		}
		if toWrite.Spec.Template.Annotations == nil {
			toWrite.Spec.Template.Annotations = map[string]string{}
		}
		toWrite.Annotations["operator.openshift.io/force"] = forceString
		toWrite.Spec.Template.Annotations["operator.openshift.io/force"] = forceString
	}

	actual, err := client.Deployments(required.Namespace).Update(toWrite)
	return actual, true, err
}

// ApplyDaemonSet merges objectmeta and requires matching generation. It returns the final Object, whether any change as made, and an error
func ApplyDaemonSet(client appsclientv1.DaemonSetsGetter, required *appsv1.DaemonSet, expectedGeneration int64, forceRollout bool) (*appsv1.DaemonSet, bool, error) {
	existing, err := client.DaemonSets(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.DaemonSets(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	// there was no change to metadata, the generation was right, and we weren't asked for force the deployment
	if !*modified && existing.ObjectMeta.Generation == expectedGeneration && !forceRollout {
		return existing, false, nil
	}

	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existing // shallow copy so the code reads easier
	toWrite.Spec = *required.Spec.DeepCopy()
	if forceRollout {
		// forces a deployment
		forceString := string(uuid.NewUUID())
		if toWrite.Annotations == nil {
			toWrite.Annotations = map[string]string{}
		}
		if toWrite.Spec.Template.Annotations == nil {
			toWrite.Spec.Template.Annotations = map[string]string{}
		}
		toWrite.Annotations["operator.openshift.io/force"] = forceString
		toWrite.Spec.Template.Annotations["operator.openshift.io/force"] = forceString
	}

	actual, err := client.DaemonSets(required.Namespace).Update(toWrite)
	return actual, true, err
}
