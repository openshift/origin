package resourceapply

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationclientv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyMutatingWebhookConfiguration ensures the form of the specified
// mutatingwebhookconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// mutatingwebhookconfiguration will be merged with the existing mutatingwebhookconfiguration
// and an update performed if the mutatingwebhookconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyMutatingWebhookConfiguration(client admissionregistrationclientv1.MutatingWebhookConfigurationsGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1.MutatingWebhookConfiguration, expectedGeneration int64) (*admissionregistrationv1.MutatingWebhookConfiguration, bool, error) {

	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}
	required := requiredOriginal.DeepCopy()

	existing, err := client.MutatingWebhookConfigurations().Get(context.TODO(), required.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.MutatingWebhookConfigurations().Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !*modified && existingCopy.GetGeneration() == expectedGeneration {
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	copyMutatingWebhookCABundle(existing, required)
	toWrite.Webhooks = required.Webhooks

	klog.V(4).Infof("MutatingWebhookConfiguration %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.MutatingWebhookConfigurations().Update(context.TODO(), toWrite, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	return actual, *modified || actual.GetGeneration() > existingCopy.GetGeneration(), nil
}

// copyMutatingWebhookCABundle populates webhooks[].clientConfig.caBundle fields from existing resource if it was set before
// and is not set in present. This provides upgrade compatibility with service-ca-bundle operator.
func copyMutatingWebhookCABundle(from, to *admissionregistrationv1.MutatingWebhookConfiguration) {
	fromMap := make(map[string]admissionregistrationv1.MutatingWebhook, len(from.Webhooks))
	for _, webhook := range from.Webhooks {
		fromMap[webhook.Name] = webhook
	}

	for i, wh := range to.Webhooks {
		if existing, ok := fromMap[wh.Name]; ok && wh.ClientConfig.CABundle == nil {
			to.Webhooks[i].ClientConfig.CABundle = existing.ClientConfig.CABundle
		}
	}
}

// ApplyValidatingWebhookConfiguration ensures the form of the specified
// validatingwebhookconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// validatingwebhookconfiguration will be merged with the existing validatingwebhookconfiguration
// and an update performed if the validatingwebhookconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyValidatingWebhookConfiguration(client admissionregistrationclientv1.ValidatingWebhookConfigurationsGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1.ValidatingWebhookConfiguration, expectedGeneration int64) (*admissionregistrationv1.ValidatingWebhookConfiguration, bool, error) {
	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}
	required := requiredOriginal.DeepCopy()

	existing, err := client.ValidatingWebhookConfigurations().Get(context.TODO(), required.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.ValidatingWebhookConfigurations().Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !*modified && existingCopy.GetGeneration() == expectedGeneration {
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	copyValidatingWebhookCABundle(existing, required)
	toWrite.Webhooks = required.Webhooks

	klog.V(4).Infof("ValidatingWebhookConfiguration %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.ValidatingWebhookConfigurations().Update(context.TODO(), toWrite, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	return actual, *modified || actual.GetGeneration() > existingCopy.GetGeneration(), nil
}

// copyValidatingWebhookCABundle populates webhooks[].clientConfig.caBundle fields from existing resource if it was set before
// and is not set in present. This provides upgrade compatibility with service-ca-bundle operator.
func copyValidatingWebhookCABundle(from, to *admissionregistrationv1.ValidatingWebhookConfiguration) {
	fromMap := make(map[string]admissionregistrationv1.ValidatingWebhook, len(from.Webhooks))
	for _, webhook := range from.Webhooks {
		fromMap[webhook.Name] = webhook
	}

	for i, wh := range to.Webhooks {
		if existing, ok := fromMap[wh.Name]; ok && wh.ClientConfig.CABundle == nil {
			to.Webhooks[i].ClientConfig.CABundle = existing.ClientConfig.CABundle
		}
	}
}
