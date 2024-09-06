package resourceapply

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"k8s.io/klog/v2"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// The Apply<type> methods in this file ensure that a resource is created or updated to match
// the form provided by the caller.
//
// If the resource does not yet exist, it will be created.
//
// If the resource exists, the metadata of the required resource will be merged with the
// existing resource and an update will be performed if the spec and metadata differ between
// the required and existing resources. To be reliable, the input of the required spec from
// the operator should be stable. It does not need to set all fields, since some fields are
// defaulted server-side.  Detection of spec drift from intent by other actors is determined
// by generation, not by spec comparison.
//
// To ensure an update in response to state external to the resource spec, the caller should
// set an annotation representing that external state e.g.
//
//   `myoperator.openshift.io/config-resource-version: <resourceVersion>`
//
// An update will be performed if:
//
// - The required resource metadata differs from that of the existing resource.
//   - The difference will be detected by comparing the name, namespace, labels and
//   annotations of the 2 resources.
//
// - The generation expected by the operator differs from generation of the existing
// resource.
//   - This is the likely result of an actor other than the operator updating a resource
//   managed by the operator.
//
// - The spec of the required resource differs from the spec of the existing resource.
//   - The difference will be detected via metadata comparison since the hash of the
//   resource's spec will be set as an annotation prior to comparison.

const specHashAnnotation = "operator.openshift.io/spec-hash"

// SetSpecHashAnnotation computes the hash of the provided spec and sets an annotation of the
// hash on the provided ObjectMeta. This method is used internally by Apply<type> methods, and
// is exposed to support testing with fake clients that need to know the mutated form of the
// resource resulting from an Apply<type> call.
func SetSpecHashAnnotation(objMeta *metav1.ObjectMeta, spec interface{}) error {
	jsonBytes, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	specHash := fmt.Sprintf("%x", sha256.Sum256(jsonBytes))
	if objMeta.Annotations == nil {
		objMeta.Annotations = map[string]string{}
	}
	objMeta.Annotations[specHashAnnotation] = specHash
	return nil
}

// ApplyDeployment ensures the form of the specified deployment is present in the API. If it
// does not exist, it will be created. If it does exist, the metadata of the required
// deployment will be merged with the existing deployment and an update performed if the
// deployment spec and metadata differ from the previously required spec and metadata. For
// further detail, check the top-level comment.
//
// NOTE: The previous implementation of this method was renamed to
// ApplyDeploymentWithForce. If are reading this in response to a compile error due to the
// change in signature, you have the following options:
//
// - Update the calling code to rely on the spec comparison provided by the new
// implementation. If the code in question was specifying the force parameter to ensure
// rollout in response to changes in resources external to the deployment, it will need to be
// revised to set that external state as an annotation e.g.
//
//	myoperator.openshift.io/my-resource: <resourceVersion>
//
// - Update the call to use ApplyDeploymentWithForce. This is available as a temporary measure
// but the method is deprecated and will be removed in 4.6.
func ApplyDeployment(ctx context.Context, client appsclientv1.DeploymentsGetter, recorder events.Recorder,
	requiredOriginal *appsv1.Deployment, expectedGeneration int64) (*appsv1.Deployment, bool, error) {

	required := requiredOriginal.DeepCopy()
	err := SetSpecHashAnnotation(&required.ObjectMeta, required.Spec)
	if err != nil {
		return nil, false, err
	}

	return ApplyDeploymentWithForce(ctx, client, recorder, required, expectedGeneration, false)
}

// ApplyDeploymentWithForce merges objectmeta and requires matching generation. It returns the final Object, whether any change as made, and an error.
//
// DEPRECATED - This method will be removed in 4.6 and callers will need to migrate to ApplyDeployment before then.
func ApplyDeploymentWithForce(ctx context.Context, client appsclientv1.DeploymentsGetter, recorder events.Recorder, requiredOriginal *appsv1.Deployment, expectedGeneration int64,
	forceRollout bool) (*appsv1.Deployment, bool, error) {

	required := requiredOriginal.DeepCopy()
	if required.Annotations == nil {
		required.Annotations = map[string]string{}
	}
	if _, ok := required.Annotations[specHashAnnotation]; !ok {
		// If the spec hash annotation is not present, the caller expects the
		// pull-spec annotation to be applied.
		required.Annotations["operator.openshift.io/pull-spec"] = required.Spec.Template.Spec.Containers[0].Image
	}
	existing, err := client.Deployments(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Deployments(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	// there was no change to metadata, the generation was right, and we weren't asked for force the deployment
	if !modified && existingCopy.ObjectMeta.Generation == expectedGeneration && !forceRollout {
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

	if klog.V(2).Enabled() {
		klog.Infof("Deployment %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, toWrite))
	}

	actual, err := client.Deployments(required.Namespace).Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyDaemonSet ensures the form of the specified daemonset is present in the API. If it
// does not exist, it will be created. If it does exist, the metadata of the required
// daemonset will be merged with the existing daemonset and an update performed if the
// daemonset spec and metadata differ from the previously required spec and metadata. For
// further detail, check the top-level comment.
//
// NOTE: The previous implementation of this method was renamed to ApplyDaemonSetWithForce. If
// are reading this in response to a compile error due to the change in signature, you have
// the following options:
//
// - Update the calling code to rely on the spec comparison provided by the new
// implementation. If the code in question was specifying the force parameter to ensure
// rollout in response to changes in resources external to the daemonset, it will need to be
// revised to set that external state as an annotation e.g.
//
//	myoperator.openshift.io/my-resource: <resourceVersion>
//
// - Update the call to use ApplyDaemonSetWithForce. This is available as a temporary measure
// but the method is deprecated and will be removed in 4.6.
func ApplyDaemonSet(ctx context.Context, client appsclientv1.DaemonSetsGetter, recorder events.Recorder,
	requiredOriginal *appsv1.DaemonSet, expectedGeneration int64) (*appsv1.DaemonSet, bool, error) {

	required := requiredOriginal.DeepCopy()
	err := SetSpecHashAnnotation(&required.ObjectMeta, required.Spec)
	if err != nil {
		return nil, false, err
	}

	return ApplyDaemonSetWithForce(ctx, client, recorder, required, expectedGeneration, false)
}

// ApplyDaemonSetWithForce merges objectmeta and requires matching generation. It returns the final Object, whether any change as made, and an error
// DEPRECATED - This method will be removed in 4.6 and callers will need to migrate to ApplyDaemonSet before then.
func ApplyDaemonSetWithForce(ctx context.Context, client appsclientv1.DaemonSetsGetter, recorder events.Recorder, requiredOriginal *appsv1.DaemonSet, expectedGeneration int64, forceRollout bool) (*appsv1.DaemonSet, bool, error) {
	required := requiredOriginal.DeepCopy()
	if required.Annotations == nil {
		required.Annotations = map[string]string{}
	}
	if _, ok := required.Annotations[specHashAnnotation]; !ok {
		// If the spec hash annotation is not present, the caller expects the
		// pull-spec annotation to be applied.
		required.Annotations["operator.openshift.io/pull-spec"] = required.Spec.Template.Spec.Containers[0].Image
	}
	existing, err := client.DaemonSets(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.DaemonSets(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	// there was no change to metadata, the generation was right, and we weren't asked for force the deployment
	if !modified && existingCopy.ObjectMeta.Generation == expectedGeneration && !forceRollout {
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

	if klog.V(2).Enabled() {
		klog.Infof("DaemonSet %q changes: %v", required.Namespace+"/"+required.Name, JSONPatchNoError(existing, toWrite))
	}
	actual, err := client.DaemonSets(required.Namespace).Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	return actual, true, err
}

func DeleteDeployment(ctx context.Context, client appsclientv1.DeploymentsGetter, recorder events.Recorder, required *appsv1.Deployment) (*appsv1.Deployment, bool, error) {
	err := client.Deployments(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeleteDaemonSet(ctx context.Context, client appsclientv1.DaemonSetsGetter, recorder events.Recorder, required *appsv1.DaemonSet) (*appsv1.DaemonSet, bool, error) {
	err := client.DaemonSets(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
