package resourceapply

import (
	"context"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

var alertmanagerGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "alertmanagers"}
var prometheusGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses"}
var prometheusRuleGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules"}
var serviceMonitorGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}

// ApplyAlertmanager applies the Alertmanager.
func ApplyAlertmanager(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return ApplyUnstructuredResourceImproved(ctx, client, recorder, required, noCache, alertmanagerGVR, nil, nil)
}

// DeleteAlertmanager deletes the Alertmanager.
func DeleteAlertmanager(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return DeleteUnstructuredResource(ctx, client, recorder, required, alertmanagerGVR)
}

// ApplyPrometheus applies the Prometheus.
func ApplyPrometheus(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return ApplyUnstructuredResourceImproved(ctx, client, recorder, required, noCache, prometheusGVR, nil, nil)
}

// DeletePrometheus deletes the Prometheus.
func DeletePrometheus(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return DeleteUnstructuredResource(ctx, client, recorder, required, prometheusGVR)
}

// ApplyPrometheusRule applies the PrometheusRule.
func ApplyPrometheusRule(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return ApplyUnstructuredResourceImproved(ctx, client, recorder, required, noCache, prometheusRuleGVR, nil, nil)
}

// DeletePrometheusRule deletes the PrometheusRule.
func DeletePrometheusRule(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return DeleteUnstructuredResource(ctx, client, recorder, required, prometheusRuleGVR)
}

// ApplyServiceMonitor applies the ServiceMonitor.
func ApplyServiceMonitor(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return ApplyUnstructuredResourceImproved(ctx, client, recorder, required, noCache, serviceMonitorGVR, nil, nil)
}

// DeleteServiceMonitor deletes the ServiceMonitor.
func DeleteServiceMonitor(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	return DeleteUnstructuredResource(ctx, client, recorder, required, serviceMonitorGVR)
}

// ApplyUnstructuredResourceImproved can utilize the cache to reconcile the existing resource to the desired state.
// NOTE: A `nil` defaultingFunc and equalityChecker are assigned resourceapply.noDefaulting and equality.Semantic,
// respectively. Users are recommended to instantiate a cache to benefit from the memoization machinery.
func ApplyUnstructuredResourceImproved(
	ctx context.Context,
	client dynamic.Interface,
	recorder events.Recorder,
	required *unstructured.Unstructured,
	cache ResourceCache,
	resourceGVR schema.GroupVersionResource,
	defaultingFunc mimicDefaultingFunc,
	equalityChecker equalityChecker,
) (*unstructured.Unstructured, bool, error) {
	name := required.GetName()
	namespace := required.GetNamespace()

	// Create if resource does not exist, and update cache with new metadata.
	if cache == nil {
		cache = noCache
	}
	existing, err := client.Resource(resourceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		want, errCreate := client.Resource(resourceGVR).Namespace(namespace).Create(ctx, required, metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, errCreate)
		cache.UpdateCachedResourceMetadata(required, want)
		return want, true, errCreate
	}
	if err != nil {
		return nil, false, err
	}

	// Skip if the cache is non-nil, and the metadata hashes and resource version hashes match.
	if cache.SafeToSkipApply(required, existing) {
		return existing, false, nil
	}

	existingCopy := existing.DeepCopy()

	// Replace and/or merge certain metadata fields.
	didMetadataModify := false
	err = resourcemerge.EnsureObjectMetaForUnstructured(&didMetadataModify, existingCopy, required)
	if err != nil {
		return nil, false, err
	}

	// Deep-check the spec objects for equality, and update the cache in either case.
	if defaultingFunc == nil {
		defaultingFunc = noDefaulting
	}
	if equalityChecker == nil {
		equalityChecker = equality.Semantic
	}
	didSpecModify := false
	err = ensureGenericSpec(&didSpecModify, required, existingCopy, defaultingFunc, equalityChecker)
	if err != nil {
		return nil, false, err
	}
	if !didSpecModify && !didMetadataModify {
		// Update cache even if certain fields are not modified, in order to maintain a consistent cache based on the
		// resource hash. The resource hash depends on the entire metadata, not just the fields that were checked above,
		cache.UpdateCachedResourceMetadata(required, existingCopy)
		return existingCopy, false, nil
	}

	// Perform update if resource exists but different from the required (desired) one.
	if klog.V(4).Enabled() {
		klog.Infof("%s %q changes: %v", resourceGVR.String(), namespace+"/"+name, JSONPatchNoError(existing, existingCopy))
	}
	actual, errUpdate := client.Resource(resourceGVR).Namespace(namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, existingCopy, errUpdate)
	cache.UpdateCachedResourceMetadata(existingCopy, actual)
	return actual, true, errUpdate
}

// DeleteUnstructuredResource deletes the unstructured resource.
func DeleteUnstructuredResource(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured, resourceGVR schema.GroupVersionResource) (*unstructured.Unstructured, bool, error) {
	err := client.Resource(resourceGVR).Namespace(required.GetNamespace()).Delete(ctx, required.GetName(), metav1.DeleteOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func ensureGenericSpec(didSpecModify *bool, required, existing *unstructured.Unstructured, mimicDefaultingFn mimicDefaultingFunc, equalityChecker equalityChecker) error {
	mimicDefaultingFn(required)
	requiredSpec, _, err := unstructured.NestedMap(required.UnstructuredContent(), "spec")
	if err != nil {
		return err
	}
	existingSpec, _, err := unstructured.NestedMap(existing.UnstructuredContent(), "spec")
	if err != nil {
		return err
	}

	if equalityChecker.DeepEqual(existingSpec, requiredSpec) {
		return nil
	}

	if err = unstructured.SetNestedMap(existing.UnstructuredContent(), requiredSpec, "spec"); err != nil {
		return err
	}
	*didSpecModify = true

	return nil
}

// mimicDefaultingFunc is used to set fields that are defaulted.  This allows for sparse manifests to apply correctly.
// For instance, if field .spec.foo is set to 10 if not set, then a function of this type could be used to set
// the field to 10 to match the comparison.  This is sometimes (often?) easier than updating the semantic equality.
// We often see this in places like RBAC and CRD.  Logically it can happen generically too.
type mimicDefaultingFunc func(obj *unstructured.Unstructured)

func noDefaulting(*unstructured.Unstructured) {}

// equalityChecker allows for custom equality comparisons.  This can be used to allow equality checks to skip certain
// operator managed fields.  This capability allows something like .spec.scale to be specified or changed by a component
// like HPA.  Use this capability sparingly.  Most places ought to just use `equality.Semantic`
type equalityChecker interface {
	DeepEqual(existing, required interface{}) bool
}
