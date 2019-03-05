package resourceapply

import (
	"github.com/golang/glog"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyDeployment merges objectmeta and requires matching generation. It returns the final Object, whether any change as made, and an error
func ApplyDeployment(client appsclientv1.DeploymentsGetter, recorder events.Recorder, required *appsv1.Deployment, expectedGeneration int64,
	forceRollout bool) (*appsv1.Deployment, bool, error) {
	if required.Annotations == nil {
		required.Annotations = map[string]string{}
	}
	required.Annotations["operator.openshift.io/pull-spec"] = required.Spec.Template.Spec.Containers[0].Image
	existing, err := client.Deployments(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Deployments(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	// there was no change to metadata, the generation was right, and we weren't asked for force the deployment
	if !*modified && existingCopy.ObjectMeta.Generation == expectedGeneration && !forceRollout {
		return existingCopy, false, nil
	}

	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
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

	if glog.V(4) {
		glog.Infof("Deployment %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, toWrite))
	}

	actual, err := client.Deployments(required.Namespace).Update(toWrite)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyDaemonSet merges objectmeta and requires matching generation. It returns the final Object, whether any change as made, and an error
func ApplyDaemonSet(client appsclientv1.DaemonSetsGetter, recorder events.Recorder, required *appsv1.DaemonSet, expectedGeneration int64, forceRollout bool) (*appsv1.DaemonSet, bool, error) {
	if required.Annotations == nil {
		required.Annotations = map[string]string{}
	}
	required.Annotations["operator.openshift.io/pull-spec"] = required.Spec.Template.Spec.Containers[0].Image
	existing, err := client.DaemonSets(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.DaemonSets(required.Namespace).Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	// there was no change to metadata, the generation was right, and we weren't asked for force the deployment
	if !*modified && existingCopy.ObjectMeta.Generation == expectedGeneration && !forceRollout {
		return existingCopy, false, nil
	}

	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
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

	if glog.V(4) {
		glog.Infof("DaemonSet %q changes: %v", required.Namespace+"/"+required.Name, JSONPatch(existing, toWrite))
	}
	actual, err := client.DaemonSets(required.Namespace).Update(toWrite)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}
