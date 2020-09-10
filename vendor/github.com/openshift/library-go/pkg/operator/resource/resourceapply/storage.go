package resourceapply

import (
	"context"

	"k8s.io/klog/v2"

	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storageclientv1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	storageclientv1beta1 "k8s.io/client-go/kubernetes/typed/storage/v1beta1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyStorageClass merges objectmeta, tries to write everything else
func ApplyStorageClass(client storageclientv1.StorageClassesGetter, recorder events.Recorder, required *storagev1.StorageClass) (*storagev1.StorageClass, bool,
	error) {
	existing, err := client.StorageClasses().Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.StorageClasses().Create(context.TODO(), required, metav1.CreateOptions{})
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
	actual, err := client.StorageClasses().Update(context.TODO(), requiredCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyCSIDriverV1Beta1 merges objectmeta, does not worry about anything else
func ApplyCSIDriverV1Beta1(client storageclientv1beta1.CSIDriversGetter, recorder events.Recorder, required *storagev1beta1.CSIDriver) (*storagev1beta1.CSIDriver, bool, error) {
	existing, err := client.CSIDrivers().Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.CSIDrivers().Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return existingCopy, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("CSIDriver %q changes: %v", required.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.CSIDrivers().Update(context.TODO(), existingCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}

// ApplyCSIDriver merges objectmeta, does not worry about anything else
func ApplyCSIDriver(client storageclientv1.CSIDriversGetter, recorder events.Recorder, required *storagev1.CSIDriver) (*storagev1.CSIDriver, bool, error) {
	existing, err := client.CSIDrivers().Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.CSIDrivers().Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	if !*modified {
		return existingCopy, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("CSIDriver %q changes: %v", required.Name, JSONPatchNoError(existing, existingCopy))
	}

	// TODO: Spec is read-only, so this will fail if user changes it. Should we simply ignore it?
	actual, err := client.CSIDrivers().Update(context.TODO(), existingCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}
