package resourceapply

import (
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

func ApplyClusterRoleBinding(client rbacclientv1.ClusterRoleBindingsGetter, required *rbacv1.ClusterRoleBinding) (bool, error) {
	existing, err := client.ClusterRoleBindings().Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	needsCreate := apierrors.IsNotFound(err)

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureClusterRoleBinding(modified, existing, *required)

	if !*modified {
		return false, nil
	}
	if needsCreate {
		_, err := client.ClusterRoleBindings().Create(existing)
		return true, err
	}

	_, err = client.ClusterRoleBindings().Update(existing)
	return true, err
}
