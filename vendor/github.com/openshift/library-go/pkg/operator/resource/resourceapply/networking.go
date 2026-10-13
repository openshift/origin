package resourceapply

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkingclientv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyNetworkPolicy merges objectmeta and requires spec
func ApplyNetworkPolicy(ctx context.Context, client networkingclientv1.NetworkPoliciesGetter, recorder events.Recorder, required *networkingv1.NetworkPolicy, cache ResourceCache) (*networkingv1.NetworkPolicy, bool, error) {
	existing, err := client.NetworkPolicies(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.NetworkPolicies(required.Namespace).Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*networkingv1.NetworkPolicy), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		cache.UpdateCachedResourceMetadata(required, actual)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(required, existing) {
		return existing, false, nil
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	specContentSame := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	if specContentSame && !modified {
		cache.UpdateCachedResourceMetadata(required, existingCopy)
		return existingCopy, false, nil
	}

	existingCopy.Spec = required.Spec

	if klog.V(2).Enabled() {
		klog.Infof("NetworkPolicy %q changes: %v", required.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.NetworkPolicies(existingCopy.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	cache.UpdateCachedResourceMetadata(required, actual)
	return actual, true, err
}

func DeleteNetworkPolicy(ctx context.Context, client networkingclientv1.NetworkPoliciesGetter, recorder events.Recorder, required *networkingv1.NetworkPolicy) (*networkingv1.NetworkPolicy, bool, error) {
	err := client.NetworkPolicies(required.Namespace).Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
