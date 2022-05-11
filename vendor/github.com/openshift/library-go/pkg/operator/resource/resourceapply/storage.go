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

	// TODO if provisioner, parameters, reclaimpolicy, or volumebindingmode are different, update will fail so delete and recreate
	actual, err := client.StorageClasses().Update(ctx, requiredCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyCSIDriver merges objectmeta, does not worry about anything else
func ApplyCSIDriver(ctx context.Context, client storageclientv1.CSIDriversGetter, recorder events.Recorder, requiredOriginal *storagev1.CSIDriver) (*storagev1.CSIDriver, bool, error) {

	required := requiredOriginal.DeepCopy()
	if required.Annotations == nil {
		required.Annotations = map[string]string{}
	}
	err := SetSpecHashAnnotation(&required.ObjectMeta, required.Spec)
	if err != nil {
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
	}
	reportCreateEvent(recorder, existingCopy, err)
	return actual, true, err
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
