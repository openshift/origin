package resourceapply

import (
	"context"

	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	VolumeSnapshotClassGroup    = "snapshot.storage.k8s.io"
	VolumeSnapshotClassVersion  = "v1"
	VolumeSnapshotClassResource = "volumesnapshotclasses"
)

var volumeSnapshotClassResourceGVR schema.GroupVersionResource = schema.GroupVersionResource{
	Group:    VolumeSnapshotClassGroup,
	Version:  VolumeSnapshotClassVersion,
	Resource: VolumeSnapshotClassResource,
}

func ensureGenericVolumeSnapshotClass(required, existing *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	var existingCopy *unstructured.Unstructured

	// Apply "parameters"
	requiredParameters, _, err := unstructured.NestedMap(required.UnstructuredContent(), "parameters")
	if err != nil {
		return nil, false, err
	}
	existingParameters, _, err := unstructured.NestedMap(existing.UnstructuredContent(), "parameters")
	if err != nil {
		return nil, false, err
	}
	if !equality.Semantic.DeepEqual(existingParameters, requiredParameters) {
		if existingCopy == nil {
			existingCopy = existing.DeepCopy()
		}
		if err := unstructured.SetNestedMap(existingCopy.UnstructuredContent(), requiredParameters, "parameters"); err != nil {
			return nil, true, err
		}
	}

	// Apply "driver" and "deletionPolicy"
	for _, fieldName := range []string{"driver", "deletionPolicy"} {
		requiredField, _, err := unstructured.NestedString(required.UnstructuredContent(), fieldName)
		if err != nil {
			return nil, false, err
		}
		existingField, _, err := unstructured.NestedString(existing.UnstructuredContent(), fieldName)
		if err != nil {
			return nil, false, err
		}
		if requiredField != existingField {
			if existingCopy == nil {
				existingCopy = existing.DeepCopy()
			}
			if err := unstructured.SetNestedField(existingCopy.UnstructuredContent(), requiredField, fieldName); err != nil {
				return nil, true, err
			}
		}
	}

	// If existingCopy is not nil, then the object has been modified
	if existingCopy != nil {
		return existingCopy, true, nil
	}

	return existing, false, nil
}

// ApplyVolumeSnapshotClass applies Volume Snapshot Class.
func ApplyVolumeSnapshotClass(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	existing, err := client.Resource(volumeSnapshotClassResourceGVR).Get(ctx, required.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		newObj, createErr := client.Resource(volumeSnapshotClassResourceGVR).Create(ctx, required, metav1.CreateOptions{})
		if createErr != nil {
			recorder.Warningf("VolumeSnapshotClassCreateFailed", "Failed to create VolumeSnapshotClass.snapshot.storage.k8s.io/v1: %v", createErr)
			return nil, true, createErr
		}
		recorder.Eventf("VolumeSnapshotClassCreated", "Created VolumeSnapshotClass.snapshot.storage.k8s.io/v1 because it was missing")
		return newObj, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	toUpdate, modified, err := ensureGenericVolumeSnapshotClass(required, existing)
	if err != nil {
		return nil, false, err
	}

	if !modified {
		return existing, false, nil
	}

	if klog.V(2).Enabled() {
		klog.Infof("VolumeSnapshotClass %q changes: %v", required.GetName(), JSONPatchNoError(existing, toUpdate))
	}

	newObj, err := client.Resource(volumeSnapshotClassResourceGVR).Update(ctx, toUpdate, metav1.UpdateOptions{})
	if err != nil {
		recorder.Warningf("VolumeSnapshotClassFailed", "Failed to update VolumeSnapshotClass.snapshot.storage.k8s.io/v1: %v", err)
		return nil, true, err
	}

	recorder.Eventf("VolumeSnapshotClassUpdated", "Updated VolumeSnapshotClass.snapshot.storage.k8s.io/v1 because it changed")
	return newObj, true, err
}

func DeleteVolumeSnapshotClass(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	namespace := required.GetNamespace()
	err := client.Resource(volumeSnapshotClassResourceGVR).Namespace(namespace).Delete(ctx, required.GetName(), metav1.DeleteOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	reportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
