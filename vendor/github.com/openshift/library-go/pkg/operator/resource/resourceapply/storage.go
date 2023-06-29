package resourceapply

import (
	"context"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storageclientv1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

const (
	// Label on the CSIDriver to declare the driver's effective pod security profile
	csiInlineVolProfileLabel = "security.openshift.io/csi-ephemeral-volume-profile"

	defaultScAnnotationKey = "storageclass.kubernetes.io/is-default-class"
)

var (
	// Exempt labels are not overwritten if the value has changed
	exemptCSIDriverLabels = []string{
		csiInlineVolProfileLabel,
	}
)

// ApplyStorageClass merges objectmeta, tries to write everything else
func ApplyStorageClass(ctx context.Context, client storageclientv1.StorageClassesGetter, recorder events.Recorder, required *storagev1.StorageClass) (*storagev1.StorageClass, bool,
	error) {
	existing, err := client.StorageClasses().Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.StorageClasses().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*storagev1.StorageClass), metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	if required.ObjectMeta.ResourceVersion != "" && required.ObjectMeta.ResourceVersion != existing.ObjectMeta.ResourceVersion {
		err = fmt.Errorf("rejected to update StorageClass %s because the object has been modified: desired/actual ResourceVersion: %v/%v",
			required.Name, required.ObjectMeta.ResourceVersion, existing.ObjectMeta.ResourceVersion)
		return nil, false, err
	}
	// Our caller may not be able to set required.ObjectMeta.ResourceVersion. We only want to overwrite value of
	// default storage class annotation if it is missing in existing.Annotations
	if existing.Annotations != nil {
		if _, ok := existing.Annotations[defaultScAnnotationKey]; ok {
			if required.Annotations == nil {
				required.Annotations = make(map[string]string)
			}
			required.Annotations[defaultScAnnotationKey] = existing.Annotations[defaultScAnnotationKey]
		}
	}

	// First, let's compare ObjectMeta from both objects
	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()
	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)

	// Second, let's compare the other fields. StorageClass doesn't have a spec and we don't
	// want to miss fields, so we have to copy required to get all fields
	// and then overwrite ObjectMeta and TypeMeta from the original.
	requiredCopy := required.DeepCopy()
	requiredCopy.ObjectMeta = *existingCopy.ObjectMeta.DeepCopy()
	requiredCopy.TypeMeta = existingCopy.TypeMeta

	contentSame := equality.Semantic.DeepEqual(existingCopy, requiredCopy)
	if contentSame && !*modified {
		return existing, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("StorageClass %q changes: %v", required.Name, JSONPatchNoError(existingCopy, requiredCopy))
	}

	if storageClassNeedsRecreate(existingCopy, requiredCopy) {
		requiredCopy.ObjectMeta.ResourceVersion = ""
		err = client.StorageClasses().Delete(ctx, existingCopy.Name, metav1.DeleteOptions{})
		reportDeleteEvent(recorder, requiredCopy, err, "Deleting StorageClass to re-create it with updated parameters")
		if err != nil && !apierrors.IsNotFound(err) {
			return existing, false, err
		}
		actual, err := client.StorageClasses().Create(ctx, requiredCopy, metav1.CreateOptions{})
		if err != nil && apierrors.IsAlreadyExists(err) {
			// Delete() few lines above did not really delete the object,
			// the API server is probably waiting for a finalizer removal or so.
			// Report an error, but something else than "Already exists", because
			// that would be very confusing - Apply failed because the object
			// already exists???
			err = fmt.Errorf("failed to re-create StorageClass %s, waiting for the original object to be deleted", existingCopy.Name)
		} else if err != nil {
			err = fmt.Errorf("failed to re-create StorageClass %s: %s", existingCopy.Name, err)
		}
		reportCreateEvent(recorder, actual, err)
		return actual, true, err
	}

	// Only mutable fields need a change
	actual, err := client.StorageClasses().Update(ctx, requiredCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

func storageClassNeedsRecreate(oldSC, newSC *storagev1.StorageClass) bool {
	// Based on kubernetes/kubernetes/pkg/apis/storage/validation/validation.go,
	// these fields are immutable.
	if !equality.Semantic.DeepEqual(oldSC.Parameters, newSC.Parameters) {
		return true
	}
	if oldSC.Provisioner != newSC.Provisioner {
		return true
	}

	// In theory, ReclaimPolicy is always set, just in case:
	if (oldSC.ReclaimPolicy == nil && newSC.ReclaimPolicy != nil) ||
		(oldSC.ReclaimPolicy != nil && newSC.ReclaimPolicy == nil) {
		return true
	}
	if oldSC.ReclaimPolicy != nil && newSC.ReclaimPolicy != nil && *oldSC.ReclaimPolicy != *newSC.ReclaimPolicy {
		return true
	}

	if !equality.Semantic.DeepEqual(oldSC.VolumeBindingMode, newSC.VolumeBindingMode) {
		return true
	}
	return false
}

// ApplyCSIDriver merges objectmeta, does not worry about anything else
func ApplyCSIDriver(ctx context.Context, client storageclientv1.CSIDriversGetter, recorder events.Recorder, requiredOriginal *storagev1.CSIDriver) (*storagev1.CSIDriver, bool, error) {

	required := requiredOriginal.DeepCopy()
	if required.Annotations == nil {
		required.Annotations = map[string]string{}
	}
	if err := SetSpecHashAnnotation(&required.ObjectMeta, required.Spec); err != nil {
		return nil, false, err
	}
	if err := validateRequiredCSIDriverLabels(required); err != nil {
		return nil, false, err
	}

	existing, err := client.CSIDrivers().Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.CSIDrivers().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*storagev1.CSIDriver), metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	// Exempt labels are not overwritten if the value has changed. They get set
	// once during creation, but the admin may choose to set a different value.
	// If the label is removed, it reverts back to the default value.
	for _, exemptLabel := range exemptCSIDriverLabels {
		if existingValue, ok := existing.Labels[exemptLabel]; ok {
			required.Labels[exemptLabel] = existingValue
		}
	}

	metadataModified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()
	resourcemerge.EnsureObjectMeta(metadataModified, &existingCopy.ObjectMeta, required.ObjectMeta)

	requiredSpecHash := required.Annotations[specHashAnnotation]
	existingSpecHash := existing.Annotations[specHashAnnotation]
	sameSpec := requiredSpecHash == existingSpecHash
	if sameSpec && !*metadataModified {
		return existing, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("CSIDriver %q changes: %v", required.Name, JSONPatchNoError(existing, existingCopy))
	}

	if sameSpec {
		// Update metadata by a simple Update call
		actual, err := client.CSIDrivers().Update(ctx, existingCopy, metav1.UpdateOptions{})
		reportUpdateEvent(recorder, required, err)
		return actual, true, err
	}

	existingCopy.Spec = required.Spec
	existingCopy.ObjectMeta.ResourceVersion = ""
	// Spec is read-only after creation. Delete and re-create the object
	err = client.CSIDrivers().Delete(ctx, existingCopy.Name, metav1.DeleteOptions{})
	reportDeleteEvent(recorder, existingCopy, err, "Deleting CSIDriver to re-create it with updated parameters")
	if err != nil && !apierrors.IsNotFound(err) {
		return existing, false, err
	}
	actual, err := client.CSIDrivers().Create(ctx, existingCopy, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) {
		// Delete() few lines above did not really delete the object,
		// the API server is probably waiting for a finalizer removal or so.
		// Report an error, but something else than "Already exists", because
		// that would be very confusing - Apply failed because the object
		// already exists???
		err = fmt.Errorf("failed to re-create CSIDriver object %s, waiting for the original object to be deleted", existingCopy.Name)
	} else if err != nil {
		err = fmt.Errorf("failed to re-create CSIDriver %s: %s", existingCopy.Name, err)
	}
	reportCreateEvent(recorder, existingCopy, err)
	return actual, true, err
}

func validateRequiredCSIDriverLabels(required *storagev1.CSIDriver) error {
	supportsEphemeralVolumes := false
	for _, mode := range required.Spec.VolumeLifecycleModes {
		if mode == storagev1.VolumeLifecycleEphemeral {
			supportsEphemeralVolumes = true
			break
		}
	}
	// All OCP managed CSI drivers that support the Ephemeral volume
	// lifecycle mode must provide a profile label the be matched against
	// the pod security policy for the namespace of the pod.
	// Valid values are: restricted, baseline, privileged.
	_, labelFound := required.Labels[csiInlineVolProfileLabel]
	if supportsEphemeralVolumes && !labelFound {
		return fmt.Errorf("CSIDriver %s supports Ephemeral volume lifecycle but is missing required label %s", required.Name, csiInlineVolProfileLabel)
	}
	return nil
}

func DeleteStorageClass(ctx context.Context, client storageclientv1.StorageClassesGetter, recorder events.Recorder, required *storagev1.StorageClass) (*storagev1.StorageClass, bool,
	error) {
	err := client.StorageClasses().Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	reportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeleteCSIDriver(ctx context.Context, client storageclientv1.CSIDriversGetter, recorder events.Recorder, required *storagev1.CSIDriver) (*storagev1.CSIDriver, bool, error) {
	err := client.CSIDrivers().Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	reportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
