package resourceapply

import (
	"context"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policyclientv1 "k8s.io/client-go/kubernetes/typed/policy/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

func ApplyPodDisruptionBudget(ctx context.Context, client policyclientv1.PodDisruptionBudgetsGetter, recorder events.Recorder, required *policyv1.PodDisruptionBudget) (*policyv1.PodDisruptionBudget, bool, error) {
	existing, err := client.PodDisruptionBudgets(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.PodDisruptionBudgets(required.Namespace).Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*policyv1.PodDisruptionBudget), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	if contentSame && !modified {
		return existingCopy, false, nil
	}

	existingCopy.Spec = required.Spec

	if klog.V(2).Enabled() {
		klog.Infof("PodDisruptionBudget %q changes: %v", required.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.PodDisruptionBudgets(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	return actual, true, err
}

func DeletePodDisruptionBudget(ctx context.Context, client policyclientv1.PodDisruptionBudgetsGetter, recorder events.Recorder, required *policyv1.PodDisruptionBudget) (*policyv1.PodDisruptionBudget, bool, error) {
	err := client.PodDisruptionBudgets(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
