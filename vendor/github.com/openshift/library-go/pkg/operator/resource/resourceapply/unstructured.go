package resourceapply

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/openshift/library-go/pkg/operator/events"
)

// ApplyKnownUnstructured applies few selected Unstructured types, where it semantic knowledge
// to merge existing & required objects intelligently. Feel free to add more.
func ApplyKnownUnstructured(ctx context.Context, client dynamic.Interface, recorder events.Recorder, obj *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	switch obj.GetObjectKind().GroupVersionKind().GroupKind() {
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "ServiceMonitor"}:
		return ApplyServiceMonitor(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "PrometheusRule"}:
		return ApplyPrometheusRule(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "snapshot.storage.k8s.io", Kind: "VolumeSnapshotClass"}:
		return ApplyVolumeSnapshotClass(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "Alertmanager"}:
		return ApplyAlertmanager(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "Prometheus"}:
		return ApplyPrometheus(ctx, client, recorder, obj)

	}

	return nil, false, fmt.Errorf("unsupported object type: %s", obj.GetKind())
}

// DeleteKnownUnstructured deletes few selected Unstructured types
func DeleteKnownUnstructured(ctx context.Context, client dynamic.Interface, recorder events.Recorder, obj *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	switch obj.GetObjectKind().GroupVersionKind().GroupKind() {
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "ServiceMonitor"}:
		return DeleteServiceMonitor(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "PrometheusRule"}:
		return DeletePrometheusRule(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "snapshot.storage.k8s.io", Kind: "VolumeSnapshotClass"}:
		return DeleteVolumeSnapshotClass(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "Alertmanager"}:
		return DeleteAlertmanager(ctx, client, recorder, obj)
	case schema.GroupKind{Group: "monitoring.coreos.com", Kind: "Prometheus"}:
		return DeletePrometheus(ctx, client, recorder, obj)

	}

	return nil, false, fmt.Errorf("unsupported object type: %s", obj.GetKind())
}
