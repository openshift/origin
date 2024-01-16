package resourceapply

import (
	"context"

	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

var serviceMonitorGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}

func ensureGenericSpec(required, existing *unstructured.Unstructured, mimicDefaultingFn mimicDefaultingFunc, equalityChecker equalityChecker) (*unstructured.Unstructured, bool, error) {
	requiredCopy := required.DeepCopy()
	mimicDefaultingFn(requiredCopy)
	requiredSpec, _, err := unstructured.NestedMap(requiredCopy.UnstructuredContent(), "spec")
	if err != nil {
		return nil, false, err
	}
	existingSpec, _, err := unstructured.NestedMap(existing.UnstructuredContent(), "spec")
	if err != nil {
		return nil, false, err
	}

	if equalityChecker.DeepEqual(existingSpec, requiredSpec) {
		return existing, false, nil
	}

	existingCopy := existing.DeepCopy()
	if err := unstructured.SetNestedMap(existingCopy.UnstructuredContent(), requiredSpec, "spec"); err != nil {
		return nil, true, err
	}

	return existingCopy, true, nil
}

// mimicDefaultingFunc is used to set fields that are defaulted.  This allows for sparse manifests to apply correctly.
// For instance, if field .spec.foo is set to 10 if not set, then a function of this type could be used to set
// the field to 10 to match the comparison.  This is soemtimes (often?) easier than updating the semantic equality.
// We often see this in places like RBAC and CRD.  Logically it can happen generically too.
type mimicDefaultingFunc func(obj *unstructured.Unstructured)

func noDefaulting(obj *unstructured.Unstructured) {}

// equalityChecker allows for custom equality comparisons.  This can be used to allow equality checks to skip certain
// operator managed fields.  This capability allows something like .spec.scale to be specified or changed by a component
// like HPA.  Use this capability sparingly.  Most places ought to just use `equality.Semantic`
type equalityChecker interface {
	DeepEqual(a1, a2 interface{}) bool
}

// ApplyServiceMonitor applies the Prometheus service monitor.
func ApplyServiceMonitor(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	namespace := required.GetNamespace()
	existing, err := client.Resource(serviceMonitorGVR).Namespace(namespace).Get(ctx, required.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		newObj, createErr := client.Resource(serviceMonitorGVR).Namespace(namespace).Create(ctx, required, metav1.CreateOptions{})
		if createErr != nil {
			recorder.Warningf("ServiceMonitorCreateFailed", "Failed to create ServiceMonitor.monitoring.coreos.com/v1: %v", createErr)
			return nil, true, createErr
		}
		recorder.Eventf("ServiceMonitorCreated", "Created ServiceMonitor.monitoring.coreos.com/v1 because it was missing")
		return newObj, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	existingCopy := existing.DeepCopy()

	toUpdate, modified, err := ensureGenericSpec(required, existingCopy, noDefaulting, equality.Semantic)
	if err != nil {
		return nil, false, err
	}

	if !modified {
		return nil, false, nil
	}

	if klog.V(2).Enabled() {
		klog.Infof("ServiceMonitor %q changes: %v", namespace+"/"+required.GetName(), JSONPatchNoError(existing, toUpdate))
	}

	newObj, err := client.Resource(serviceMonitorGVR).Namespace(namespace).Update(ctx, toUpdate, metav1.UpdateOptions{})
	if err != nil {
		recorder.Warningf("ServiceMonitorUpdateFailed", "Failed to update ServiceMonitor.monitoring.coreos.com/v1: %v", err)
		return nil, true, err
	}

	recorder.Eventf("ServiceMonitorUpdated", "Updated ServiceMonitor.monitoring.coreos.com/v1 because it changed")
	return newObj, true, err
}

var prometheusRuleGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules"}

// ApplyPrometheusRule applies the PrometheusRule
func ApplyPrometheusRule(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	namespace := required.GetNamespace()

	existing, err := client.Resource(prometheusRuleGVR).Namespace(namespace).Get(ctx, required.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		newObj, createErr := client.Resource(prometheusRuleGVR).Namespace(namespace).Create(ctx, required, metav1.CreateOptions{})
		if createErr != nil {
			recorder.Warningf("PrometheusRuleCreateFailed", "Failed to create PrometheusRule.monitoring.coreos.com/v1: %v", createErr)
			return nil, true, createErr
		}
		recorder.Eventf("PrometheusRuleCreated", "Created PrometheusRule.monitoring.coreos.com/v1 because it was missing")
		return newObj, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	existingCopy := existing.DeepCopy()

	toUpdate, modified, err := ensureGenericSpec(required, existingCopy, noDefaulting, equality.Semantic)
	if err != nil {
		return nil, false, err
	}

	if !modified {
		return nil, false, nil
	}

	if klog.V(2).Enabled() {
		klog.Infof("PrometheusRule %q changes: %v", namespace+"/"+required.GetName(), JSONPatchNoError(existing, toUpdate))
	}

	newObj, err := client.Resource(prometheusRuleGVR).Namespace(namespace).Update(ctx, toUpdate, metav1.UpdateOptions{})
	if err != nil {
		recorder.Warningf("PrometheusRuleUpdateFailed", "Failed to update PrometheusRule.monitoring.coreos.com/v1: %v", err)
		return nil, true, err
	}

	recorder.Eventf("PrometheusRuleUpdated", "Updated PrometheusRule.monitoring.coreos.com/v1 because it changed")
	return newObj, true, err
}

func DeletePrometheusRule(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	namespace := required.GetNamespace()
	err := client.Resource(prometheusRuleGVR).Namespace(namespace).Delete(ctx, required.GetName(), metav1.DeleteOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	reportDeleteEvent(recorder, required, err)
	return nil, true, nil
}

func DeleteServiceMonitor(ctx context.Context, client dynamic.Interface, recorder events.Recorder, required *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	namespace := required.GetNamespace()
	err := client.Resource(serviceMonitorGVR).Namespace(namespace).Delete(ctx, required.GetName(), metav1.DeleteOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	reportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
