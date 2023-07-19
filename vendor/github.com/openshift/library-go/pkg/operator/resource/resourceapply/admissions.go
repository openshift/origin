package resourceapply

import (
	"context"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionclientv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyValidatingWebhookConfiguration merges objectmeta, update webhooks.
func ApplyValidatingWebhookConfiguration(client admissionclientv1.ValidatingWebhookConfigurationsGetter, recorder events.Recorder, required *admissionv1.ValidatingWebhookConfiguration) (*admissionv1.ValidatingWebhookConfiguration, bool, error) {
	existing, err := client.ValidatingWebhookConfigurations().Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ValidatingWebhookConfigurations().
			Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy.Webhooks, required.Webhooks)
	if contentSame && !*modified {
		return existingCopy, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("ValidatingWebhookConfiguration %q changes: %v", required.Name, JSONPatchNoError(existing, required))
	}

	actual, err := client.ValidatingWebhookConfigurations().Update(context.TODO(), required, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyMutatingWebhookConfiguration merges objectmeta, update webhooks.
func ApplyMutatingWebhookConfiguration(client admissionclientv1.MutatingWebhookConfigurationsGetter, recorder events.Recorder, required *admissionv1.MutatingWebhookConfiguration) (*admissionv1.MutatingWebhookConfiguration, bool, error) {
	existing, err := client.MutatingWebhookConfigurations().Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.MutatingWebhookConfigurations().
			Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy.Webhooks, required.Webhooks)
	if contentSame && !*modified {
		return existingCopy, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("ValidatingWebhookConfiguration %q changes: %v", required.Name, JSONPatchNoError(existing, required))
	}

	actual, err := client.MutatingWebhookConfigurations().Update(context.TODO(), required, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}
