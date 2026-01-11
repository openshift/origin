package resourceapply

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionregistrationclientv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	admissionregistrationclientv1beta1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	"k8s.io/klog/v2"
)

// ApplyMutatingWebhookConfigurationImproved ensures the form of the specified
// mutatingwebhookconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// mutatingwebhookconfiguration will be merged with the existing mutatingwebhookconfiguration
// and an update performed if the mutatingwebhookconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyMutatingWebhookConfigurationImproved(ctx context.Context, client admissionregistrationclientv1.MutatingWebhookConfigurationsGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1.MutatingWebhookConfiguration, cache ResourceCache) (*admissionregistrationv1.MutatingWebhookConfiguration, bool, error) {

	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}

	existing, err := client.MutatingWebhookConfigurations().Get(ctx, requiredOriginal.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		required := requiredOriginal.DeepCopy()
		actual, err := client.MutatingWebhookConfigurations().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(required).(*admissionregistrationv1.MutatingWebhookConfiguration), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(requiredOriginal, existing) {
		return existing, false, nil
	}

	required := requiredOriginal.DeepCopy()
	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	copyMutatingWebhookCABundle(existing, required)
	webhooksEquivalent := equality.Semantic.DeepEqual(existingCopy.Webhooks, required.Webhooks)
	if webhooksEquivalent && !modified {
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, existingCopy)
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	toWrite.Webhooks = required.Webhooks

	klog.V(2).Infof("MutatingWebhookConfiguration %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.MutatingWebhookConfigurations().Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
	cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
	return actual, true, nil
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

// ApplyValidatingWebhookConfigurationImproved ensures the form of the specified
// validatingwebhookconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// validatingwebhookconfiguration will be merged with the existing validatingwebhookconfiguration
// and an update performed if the validatingwebhookconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyValidatingWebhookConfigurationImproved(ctx context.Context, client admissionregistrationclientv1.ValidatingWebhookConfigurationsGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1.ValidatingWebhookConfiguration, cache ResourceCache) (*admissionregistrationv1.ValidatingWebhookConfiguration, bool, error) {
	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}

	existing, err := client.ValidatingWebhookConfigurations().Get(ctx, requiredOriginal.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		required := requiredOriginal.DeepCopy()
		actual, err := client.ValidatingWebhookConfigurations().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(required).(*admissionregistrationv1.ValidatingWebhookConfiguration), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(requiredOriginal, existing) {
		return existing, false, nil
	}

	required := requiredOriginal.DeepCopy()
	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	copyValidatingWebhookCABundle(existing, required)
	webhooksEquivalent := equality.Semantic.DeepEqual(existingCopy.Webhooks, required.Webhooks)
	if webhooksEquivalent && !modified {
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, existingCopy)
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	toWrite.Webhooks = required.Webhooks

	klog.V(2).Infof("ValidatingWebhookConfiguration %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.ValidatingWebhookConfigurations().Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
	cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
	return actual, true, nil
}

func DeleteValidatingWebhookConfiguration(ctx context.Context, client admissionregistrationclientv1.ValidatingWebhookConfigurationsGetter, recorder events.Recorder, required *admissionregistrationv1.ValidatingWebhookConfiguration) (*admissionregistrationv1.ValidatingWebhookConfiguration, bool, error) {
	err := client.ValidatingWebhookConfigurations().Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
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

// ApplyValidatingAdmissionPolicyV1beta1 ensures the form of the specified
// validatingadmissionpolicyconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// validatingadmissionpolicyconfiguration will be merged with the existing validatingadmissionpolicyconfiguration
// and an update performed if the validatingadmissionpolicyconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyValidatingAdmissionPolicyV1beta1(ctx context.Context, client admissionregistrationclientv1beta1.ValidatingAdmissionPoliciesGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1beta1.ValidatingAdmissionPolicy, cache ResourceCache) (*admissionregistrationv1beta1.ValidatingAdmissionPolicy, bool, error) {
	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}

	existing, err := client.ValidatingAdmissionPolicies().Get(ctx, requiredOriginal.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		required := requiredOriginal.DeepCopy()
		actual, err := client.ValidatingAdmissionPolicies().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(required).(*admissionregistrationv1beta1.ValidatingAdmissionPolicy), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(requiredOriginal, existing) {
		return existing, false, nil
	}

	required := requiredOriginal.DeepCopy()
	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	specEquivalent := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	if specEquivalent && !modified {
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, existingCopy)
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	toWrite.Spec = required.Spec

	klog.V(2).Infof("ValidatingAdmissionPolicyConfigurationV1beta1 %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.ValidatingAdmissionPolicies().Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
	cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
	return actual, true, nil
}

// ApplyValidatingAdmissionPolicyV1 ensures the form of the specified
// validatingadmissionpolicyconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// validatingadmissionpolicyconfiguration will be merged with the existing validatingadmissionpolicyconfiguration
// and an update performed if the validatingadmissionpolicyconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyValidatingAdmissionPolicyV1(ctx context.Context, client admissionregistrationclientv1.ValidatingAdmissionPoliciesGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1.ValidatingAdmissionPolicy, cache ResourceCache) (*admissionregistrationv1.ValidatingAdmissionPolicy, bool, error) {
	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}

	existing, err := client.ValidatingAdmissionPolicies().Get(ctx, requiredOriginal.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		required := requiredOriginal.DeepCopy()
		actual, err := client.ValidatingAdmissionPolicies().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(required).(*admissionregistrationv1.ValidatingAdmissionPolicy), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(requiredOriginal, existing) {
		return existing, false, nil
	}

	required := requiredOriginal.DeepCopy()
	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	specEquivalent := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	if specEquivalent && !modified {
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, existingCopy)
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	toWrite.Spec = required.Spec

	klog.V(2).Infof("ValidatingAdmissionPolicyConfigurationV1 %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.ValidatingAdmissionPolicies().Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
	cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
	return actual, true, nil
}

// ApplyValidatingAdmissionPolicyBindingV1beta1 ensures the form of the specified
// validatingadmissionpolicybindingconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// validatingadmissionpolicybindingconfiguration will be merged with the existing validatingadmissionpolicybindingconfiguration
// and an update performed if the validatingadmissionpolicybindingconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyValidatingAdmissionPolicyBindingV1beta1(ctx context.Context, client admissionregistrationclientv1beta1.ValidatingAdmissionPolicyBindingsGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, cache ResourceCache) (*admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, bool, error) {
	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}

	existing, err := client.ValidatingAdmissionPolicyBindings().Get(ctx, requiredOriginal.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		required := requiredOriginal.DeepCopy()
		actual, err := client.ValidatingAdmissionPolicyBindings().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(required).(*admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(requiredOriginal, existing) {
		return existing, false, nil
	}

	required := requiredOriginal.DeepCopy()
	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	specEquivalent := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	if specEquivalent && !modified {
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, existingCopy)
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	toWrite.Spec = required.Spec

	klog.V(2).Infof("ValidatingAdmissionPolicyBindingConfigurationV1beta1 %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.ValidatingAdmissionPolicyBindings().Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
	cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
	return actual, true, nil
}

// ApplyValidatingAdmissionPolicyBindingV1 ensures the form of the specified
// validatingadmissionpolicybindingconfiguration is present in the API. If it does not exist,
// it will be created. If it does exist, the metadata of the required
// validatingadmissionpolicybindingconfiguration will be merged with the existing validatingadmissionpolicybindingconfiguration
// and an update performed if the validatingadmissionpolicybindingconfiguration spec and metadata differ from
// the previously required spec and metadata based on generation change.
func ApplyValidatingAdmissionPolicyBindingV1(ctx context.Context, client admissionregistrationclientv1.ValidatingAdmissionPolicyBindingsGetter, recorder events.Recorder,
	requiredOriginal *admissionregistrationv1.ValidatingAdmissionPolicyBinding, cache ResourceCache) (*admissionregistrationv1.ValidatingAdmissionPolicyBinding, bool, error) {
	if requiredOriginal == nil {
		return nil, false, fmt.Errorf("Unexpected nil instead of an object")
	}

	existing, err := client.ValidatingAdmissionPolicyBindings().Get(ctx, requiredOriginal.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		required := requiredOriginal.DeepCopy()
		actual, err := client.ValidatingAdmissionPolicyBindings().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(required).(*admissionregistrationv1.ValidatingAdmissionPolicyBinding), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		if err != nil {
			return nil, false, err
		}
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
		return actual, true, nil
	} else if err != nil {
		return nil, false, err
	}

	if cache.SafeToSkipApply(requiredOriginal, existing) {
		return existing, false, nil
	}

	required := requiredOriginal.DeepCopy()
	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	specEquivalent := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)
	if specEquivalent && !modified {
		// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
		cache.UpdateCachedResourceMetadata(requiredOriginal, existingCopy)
		return existingCopy, false, nil
	}
	// at this point we know that we're going to perform a write.  We're just trying to get the object correct
	toWrite := existingCopy // shallow copy so the code reads easier
	toWrite.Spec = required.Spec

	klog.V(2).Infof("ValidatingAdmissionPolicyBindingConfigurationV1 %q changes: %v", required.GetNamespace()+"/"+required.GetName(), JSONPatchNoError(existing, toWrite))

	actual, err := client.ValidatingAdmissionPolicyBindings().Update(ctx, toWrite, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)
	if err != nil {
		return nil, false, err
	}
	// need to store the original so that the early comparison of hashes is done based on the original, not a mutated copy
	cache.UpdateCachedResourceMetadata(requiredOriginal, actual)
	return actual, true, nil
}
