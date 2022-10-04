package ingress

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	operatorcontroller "github.com/openshift/cluster-ingress-operator/pkg/operator/controller"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ensureServiceMonitor ensures the servicemonitor exists for a given
// ingresscontroller.  Returns a Boolean indicating whether the servicemonitor
// exists, the servicemonitor if it does exist, and an error value.
func (r *reconciler) ensureServiceMonitor(ic *operatorv1.IngressController, svc *corev1.Service, deploymentRef metav1.OwnerReference) (bool, *unstructured.Unstructured, error) {
	desired := desiredServiceMonitor(ic, svc, deploymentRef)

	haveSM, current, err := r.currentServiceMonitor(ic)
	if err != nil {
		return false, nil, err
	}

	switch {
	case !haveSM:
		if err := r.client.Create(context.TODO(), desired); err != nil {
			return false, nil, fmt.Errorf("failed to create servicemonitor %s/%s: %v", desired.GetNamespace(), desired.GetName(), err)
		}
		log.Info("created servicemonitor", "namespace", desired.GetNamespace(), "name", desired.GetName())
		return r.currentServiceMonitor(ic)
	case haveSM:
		if updated, err := r.updateServiceMonitor(current, desired); err != nil {
			return true, current, fmt.Errorf("failed to update servicemonitor %s/%s: %v", desired.GetNamespace(), desired.GetName(), err)
		} else if updated {
			return r.currentServiceMonitor(ic)
		}
	}

	return true, current, nil
}

// desiredServiceMonitor returns the desired servicemonitor for the given
// ingresscontroller and service.
func desiredServiceMonitor(ic *operatorv1.IngressController, svc *corev1.Service, deploymentRef metav1.OwnerReference) *unstructured.Unstructured {
	name := controller.IngressControllerServiceMonitorName(ic)
	sm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": name.Namespace,
				"name":      name.Name,
			},
			"spec": map[string]interface{}{
				"namespaceSelector": map[string]interface{}{
					"matchNames": []interface{}{
						operatorcontroller.DefaultOperandNamespace,
					},
				},
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						manifests.OwningIngressControllerLabel: ic.Name,
					},
				},
				// It is important to use the type []interface{}
				// for the "endpoints" field.  Using
				// []map[string]interface{} causes at least two
				// problems: first, DeepCopy will fail with
				// "cannot deep copy []map[string]interface {}";
				// second, the API returns an object that uses
				// type []interface{} for this field, so
				// DeepEqual against an API object will always
				// return false.
				"endpoints": []interface{}{
					map[string]interface{}{
						"bearerTokenFile": "/var/run/secrets/kubernetes.io/serviceaccount/token",
						"interval":        "30s",
						"port":            "metrics",
						"scheme":          "https",
						"path":            "/metrics",
						"tlsConfig": map[string]interface{}{
							"caFile":     "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt",
							"serverName": fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace),
						},
					},
				},
			},
		},
	}
	sm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Kind:    "ServiceMonitor",
		Version: "v1",
	})
	sm.SetOwnerReferences([]metav1.OwnerReference{deploymentRef})
	return sm
}

// currentServiceMonitor returns the current servicemonitor.  Returns a Boolean
// indicating whether the servicemonitor existed, the servicemonitor if it did
// exist, and an error value.
func (r *reconciler) currentServiceMonitor(ic *operatorv1.IngressController) (bool, *unstructured.Unstructured, error) {
	sm := &unstructured.Unstructured{}
	sm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Kind:    "ServiceMonitor",
		Version: "v1",
	})
	if err := r.client.Get(context.TODO(), controller.IngressControllerServiceMonitorName(ic), sm); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, sm, nil
}

// updateServiceMonitor updates a servicemonitor.  Returns a Boolean indicating
// whether the servicemonitor was updated, and an error value.
func (r *reconciler) updateServiceMonitor(current, desired *unstructured.Unstructured) (bool, error) {
	changed, updated := serviceMonitorChanged(current, desired)
	if !changed {
		return false, nil
	}

	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		return false, err
	}
	log.Info("updated servicemonitor", "namespace", updated.GetNamespace(), "name", updated.GetName(), "diff", diff)
	return true, nil
}

// serviceMonitorChanged checks if current servicemonitor spec matches the
// expected spec and if not returns an updated one.
func serviceMonitorChanged(current, expected *unstructured.Unstructured) (bool, *unstructured.Unstructured) {
	if reflect.DeepEqual(current.Object["spec"], expected.Object["spec"]) {
		return false, nil
	}

	updated := current.DeepCopy()
	updated.Object["spec"] = expected.Object["spec"]
	return true, updated
}
