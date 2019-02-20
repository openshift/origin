package resourceapply

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/imdario/mergo"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/openshift/library-go/pkg/operator/events"
)

var serviceMonitorGVR = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}

func ensureServiceMonitorSpec(required, existing *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	requiredSpec, _, err := unstructured.NestedMap(required.UnstructuredContent(), "spec")
	if err != nil {
		return nil, false, err
	}
	existingSpec, _, err := unstructured.NestedMap(existing.UnstructuredContent(), "spec")
	if err != nil {
		return nil, false, err
	}

	if err := mergo.Merge(&existingSpec, &requiredSpec); err != nil {
		return nil, false, err
	}

	if equality.Semantic.DeepEqual(existingSpec, requiredSpec) {
		return existing, false, nil
	}

	existingCopy := existing.DeepCopy()
	if err := unstructured.SetNestedMap(existingCopy.UnstructuredContent(), existingSpec, "spec"); err != nil {
		return nil, true, err
	}

	return existingCopy, true, nil
}

// ApplyServiceMonitor applies the Prometheus service monitor.
func ApplyServiceMonitor(client dynamic.Interface, recorder events.Recorder, serviceMonitorBytes []byte) (bool, error) {
	monitorJSON, err := yaml.YAMLToJSON(serviceMonitorBytes)
	if err != nil {
		return false, err
	}

	monitorObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, monitorJSON)
	if err != nil {
		return false, err
	}

	required, ok := monitorObj.(*unstructured.Unstructured)
	if !ok {
		return false, fmt.Errorf("unexpected object in %t", monitorObj)
	}

	namespace := required.GetNamespace()

	existing, err := client.Resource(serviceMonitorGVR).Namespace(namespace).Get(required.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, createErr := client.Resource(serviceMonitorGVR).Namespace(namespace).Create(required, metav1.CreateOptions{})
		if createErr != nil {
			recorder.Warningf("ServiceMonitorCreateFailed", "Failed to create ServiceMonitor.monitoring.coreos.com/v1: %v", createErr)
			return true, createErr
		}
		recorder.Eventf("ServiceMonitorCreated", "Created ServiceMonitor.monitoring.coreos.com/v1 because it was missing")
		return true, nil
	}

	existingCopy := existing.DeepCopy()

	updated, endpointsModified, err := ensureServiceMonitorSpec(required, existingCopy)
	if err != nil {
		return false, err
	}

	if !endpointsModified {
		return false, nil
	}

	if glog.V(4) {
		glog.Infof("ServiceMonitor %q changes: %v", namespace+"/"+required.GetName(), JSONPatch(existing, existingCopy))
	}

	if _, err = client.Resource(serviceMonitorGVR).Namespace(namespace).Update(updated, metav1.UpdateOptions{}); err != nil {
		recorder.Warningf("ServiceMonitorUpdateFailed", "Failed to update ServiceMonitor.monitoring.coreos.com/v1: %v", err)
		return true, err
	}

	recorder.Eventf("ServiceMonitorUpdated", "Updated ServiceMonitor.monitoring.coreos.com/v1 because it changed")
	return true, err
}
