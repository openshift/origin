package ingress

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ensureServiceCAConfigMap ensures the configmap for the service CA bundle
// exists.  Returns a Boolean indicating whether the configmap exists, the
// configmap if it does exist, and an error value.
func (r *reconciler) ensureServiceCAConfigMap() (bool, *corev1.ConfigMap, error) {
	wantCM, desired, err := desiredServiceCAConfigMap()
	if err != nil {
		return false, nil, fmt.Errorf("failed to build configmap: %v", err)
	}

	haveCM, current, err := r.currentServiceCAConfigMap()
	if err != nil {
		return false, nil, err
	}

	switch {
	case !wantCM && !haveCM:
		return false, nil, nil
	case !wantCM && haveCM:
		if err := r.client.Delete(context.TODO(), current); err != nil {
			if !errors.IsNotFound(err) {
				return true, current, fmt.Errorf("failed to delete configmap: %v", err)
			}
		} else {
			log.Info("deleted configmap", "configmap", current)
		}
		return false, nil, nil
	case wantCM && !haveCM:
		if err := r.client.Create(context.TODO(), desired); err != nil {
			return false, nil, fmt.Errorf("failed to create configmap: %v", err)
		}
		log.Info("created configmap", "configmap", desired)
		return r.currentServiceCAConfigMap()
	case wantCM && haveCM:
		if updated, err := r.updateServiceCAConfigMap(current, desired); err != nil {
			return true, current, fmt.Errorf("failed to update configmap: %v", err)
		} else if updated {
			return r.currentServiceCAConfigMap()
		}
	}

	return true, current, nil
}

// desiredServiceCAConfigMap returns the desired configmap for the service CA
// bundle.  Returns a Boolean indicating whether a configmap is desired, as well
// as the configmap if one is desired.
func desiredServiceCAConfigMap() (bool, *corev1.ConfigMap, error) {
	name := controller.ServiceCAConfigMapName()
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"description": "ConfigMap providing service CA bundle.",
				"service.beta.openshift.io/inject-cabundle": "true",
			},
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}

	return true, &cm, nil
}

// currentServiceCAConfigMap returns the current configmap for the service CA
// bundle.  Returns a Boolean indicating whether the configmap existed, the
// configmap if it did exist, and an error value.
func (r *reconciler) currentServiceCAConfigMap() (bool, *corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), controller.ServiceCAConfigMapName(), cm); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, cm, nil
}

// updateServiceCAConfigMap updates the configmap for the service CA bundle if
// an update is needed.  In particular, the "inject-cabundle" annotation must be
// set so that the serving cert signer updates the configmap's data with the
// service CA bundle (updateServiceCAConfigMap itself does not set any data).
// Returns a Boolean indicating whether updateServiceCAConfigMap updated the
// configmap, and an error value.
func (r *reconciler) updateServiceCAConfigMap(current, desired *corev1.ConfigMap) (bool, error) {
	if current.Annotations["service.beta.openshift.io/inject-cabundle"] == "true" {
		return false, nil
	}

	updated := current.DeepCopy()
	updated.Annotations["service.beta.openshift.io/inject-cabundle"] = "true"
	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		if errors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}
	log.Info("updated configmap", "namespace", updated.Namespace, "name", updated.Name, "diff", diff)
	return true, nil
}
