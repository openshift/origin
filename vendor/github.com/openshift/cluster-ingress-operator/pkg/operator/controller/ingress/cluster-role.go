package ingress

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
)

func (r *reconciler) ensureClusterRole() (bool, *rbacv1.ClusterRole, error) {
	wantCR, desired, err := desiredClusterRole()
	if err != nil {
		return false, nil, fmt.Errorf("failed to build cluster role: %v", err)
	}

	haveCR, current, err := r.currentClusterRole()
	if err != nil {
		return false, nil, err
	}

	switch {
	case !wantCR && !haveCR:
		return false, nil, nil
	case !wantCR && haveCR:
		if err := r.client.Delete(context.TODO(), current); err != nil {
			if !errors.IsNotFound(err) {
				return true, current, fmt.Errorf("failed to delete cluster role: %v", err)
			}
		} else {
			log.Info("deleted cluster role", "current", current)
		}
		return false, nil, nil
	case wantCR && !haveCR:
		if err := r.client.Create(context.TODO(), desired); err != nil {
			return false, nil, fmt.Errorf("failed to create cluster role: %v", err)
		}
		log.Info("created cluster role", "desired", desired)
		return r.currentClusterRole()
	case wantCR && haveCR:
		if updated, err := r.updateClusterRole(current, desired); err != nil {
			return true, current, fmt.Errorf("failed to update cluster role: %v", err)
		} else if updated {
			return r.currentClusterRole()
		}
	}

	return true, current, nil
}

func desiredClusterRole() (bool, *rbacv1.ClusterRole, error) {
	return true, manifests.RouterClusterRole(), nil
}

func (r *reconciler) currentClusterRole() (bool, *rbacv1.ClusterRole, error) {
	cr := &rbacv1.ClusterRole{}
	name := types.NamespacedName{
		Name: manifests.RouterClusterRole().Name,
	}
	if err := r.client.Get(context.TODO(), name, cr); err != nil {
		if errors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, cr, nil
}

func (r *reconciler) updateClusterRole(current, desired *rbacv1.ClusterRole) (bool, error) {
	if reflect.DeepEqual(current.Rules, desired.Rules) {
		return false, nil
	}
	updated := current.DeepCopy()
	updated.Rules = desired.Rules
	// Diff before updating because the client may mutate the object.
	diff := cmp.Diff(current, updated, cmpopts.EquateEmpty())
	if err := r.client.Update(context.TODO(), updated); err != nil {
		if errors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}
	log.Info("updated cluster role", "namespace", updated.Namespace, "name", updated.Name, "diff", diff)
	return true, nil
}
