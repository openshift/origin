package ingress

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	operatorcontroller "github.com/openshift/cluster-ingress-operator/pkg/operator/controller"
)

// ensureRouterNamespace ensures that the router namespace exists.
func (r *reconciler) ensureRouterNamespace() (bool, *corev1.Namespace, error) {
	desired := manifests.RouterNamespace()

	haveNamespace, current, err := r.currentRouterNamespace()
	if err != nil {
		return false, nil, err
	}

	switch {
	case !haveNamespace:
		if err := r.client.Create(context.TODO(), desired); err != nil {
			return false, nil, fmt.Errorf("failed to create router namespace: %v", err)
		}
		log.Info("created router namespace", "desired", desired)
		return r.currentRouterNamespace()
	case haveNamespace:
		if updated, err := r.updateRouterNamespace(current, desired); err != nil {
			return true, current, fmt.Errorf("failed to update router namespace: %v", err)
		} else if updated {
			return r.currentRouterNamespace()
		}
	}
	return true, current, nil
}

// currentRouterNamespace gets the current router namespace resource.
func (r *reconciler) currentRouterNamespace() (bool, *corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	name := types.NamespacedName{
		Name: operatorcontroller.DefaultOperandNamespace,
	}
	if err := r.client.Get(context.TODO(), name, namespace); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, namespace, nil
}

// updateRouterNamespace updates the router namespace if an appropriate change
// has been detected.
func (r *reconciler) updateRouterNamespace(current, desired *corev1.Namespace) (bool, error) {
	changed, updated := routerNamespaceChanged(current, desired)
	if !changed {
		return false, nil
	}
	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		if errors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}
	log.Info("updated router namespace", "namespace", updated.Name, "diff", diff)
	return true, nil
}

// routerNamespaceChanged returns true if current and expected differ by any
// of the annotations or the labels managed by the operator.
// Other namespace labels and annotations are not overwritten so that
// values written by other controllers are preserved.
func routerNamespaceChanged(current, expected *corev1.Namespace) (bool, *corev1.Namespace) {
	knownAnnotations := []string{
		"workload.openshift.io/allowed",
	}

	knownLabels := []string{
		"openshift.io/cluster-monitoring",
		"name",
		"network.openshift.io/policy-group",
		"policy-group.network.openshift.io/ingress",
		"pod-security.kubernetes.io/enforce",
		"pod-security.kubernetes.io/audit",
		"pod-security.kubernetes.io/warn",
	}

	updated := current.DeepCopy()
	changed := false

	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}

	if updated.Labels == nil {
		updated.Labels = map[string]string{}
	}

	for _, annotation := range knownAnnotations {
		if val, ok := current.Annotations[annotation]; !ok || val != expected.Annotations[annotation] {
			updated.Annotations[annotation] = expected.Annotations[annotation]
			changed = true
		}
	}

	for _, label := range knownLabels {
		if val, ok := current.Labels[label]; !ok || val != expected.Labels[label] {
			updated.Labels[label] = expected.Labels[label]
			changed = true
		}
	}

	if !changed {
		return false, nil
	}

	return true, updated
}

func (r *reconciler) ensureRouterServiceAccount() error {
	sa := manifests.RouterServiceAccount()
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: sa.Namespace, Name: sa.Name}, sa); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get router service account %s/%s: %v", sa.Namespace, sa.Name, err)
		}
		if err := r.client.Create(context.TODO(), sa); err != nil {
			return fmt.Errorf("failed to create router service account %s/%s: %v", sa.Namespace, sa.Name, err)
		}
		log.Info("created router service account", "namespace", sa.Namespace, "name", sa.Name)
	}

	return nil
}

func (r *reconciler) ensureRouterClusterRoleBinding() error {
	crb := manifests.RouterClusterRoleBinding()
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: crb.Name}, crb); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get router cluster role binding %s: %v", crb.Name, err)
		}
		if err := r.client.Create(context.TODO(), crb); err != nil {
			return fmt.Errorf("failed to create router cluster role binding %s: %v", crb.Name, err)
		}
		log.Info("created router cluster role binding", "name", crb.Name)
	}
	return nil
}
