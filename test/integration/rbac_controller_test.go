package integration

import (
	"testing"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestRBACController(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}

	originClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	ns := "rbac-controller-namespace"

	if _, err := kubeClient.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
		t.Fatalf("Error creating namespace: %v", err)
	}

	// Initial creation
	clusterrole, err := originClient.ClusterRoles().Create(&authorizationapi.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "rbac-controller-clusterrole"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clusterrolebinding, err := originClient.ClusterRoleBindings().Create(&authorizationapi.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "rbac-controller-clusterrolebinding"},
		RoleRef:    kapi.ObjectReference{Name: "rbac-controller-clusterrole"},
	})
	if err != nil {
		t.Fatal(err)
	}
	role, err := originClient.Roles(ns).Create(&authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "rbac-controller-role"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rolebinding, err := originClient.RoleBindings(ns).Create(&authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "rbac-controller-rolebinding"},
		RoleRef:    kapi.ObjectReference{Name: "rbac-controller-role", Namespace: ns},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Ensure propagation
	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		if _, err := kubeClient.Rbac().ClusterRoles().Get(clusterrole.Name, metav1.GetOptions{}); kapierrors.IsNotFound(err) {
			t.Logf("Retrying: %v", err)
			return false, nil
		} else if err != nil {
			t.Fatal(err)
		}

		if _, err := kubeClient.Rbac().Roles(ns).Get(role.Name, metav1.GetOptions{}); kapierrors.IsNotFound(err) {
			t.Logf("Retrying: %v", err)
			return false, nil
		} else if err != nil {
			t.Fatal(err)
		}

		if _, err := kubeClient.Rbac().ClusterRoleBindings().Get(clusterrolebinding.Name, metav1.GetOptions{}); kapierrors.IsNotFound(err) {
			t.Logf("Retrying: %v", err)
			return false, nil
		} else if err != nil {
			t.Fatal(err)
		}

		if _, err := kubeClient.Rbac().RoleBindings(ns).Get(rolebinding.Name, metav1.GetOptions{}); kapierrors.IsNotFound(err) {
			t.Logf("Retrying: %v", err)
			return false, nil
		} else if err != nil {
			t.Fatal(err)
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("created objects did not propagate: %v", err)
	}

	// Update
	clusterrole.Labels = map[string]string{"updated": "true"}
	clusterrolebinding.Labels = map[string]string{"updated": "true"}
	role.Labels = map[string]string{"updated": "true"}
	rolebinding.Labels = map[string]string{"updated": "true"}

	clusterrole, err = originClient.ClusterRoles().Update(clusterrole)
	if err != nil {
		t.Fatal(err)
	}
	clusterrolebinding, err = originClient.ClusterRoleBindings().Update(clusterrolebinding)
	if err != nil {
		t.Fatal(err)
	}
	role, err = originClient.Roles(ns).Update(role)
	if err != nil {
		t.Fatal(err)
	}
	rolebinding, err = originClient.RoleBindings(ns).Update(rolebinding)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure propagation
	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		if rbacObject, err := kubeClient.Rbac().ClusterRoles().Get(clusterrole.Name, metav1.GetOptions{}); err != nil {
			t.Fatal(err)
		} else if rbacObject.Labels["updated"] != "true" {
			t.Logf("not updated yet: %#v", rbacObject)
			return false, nil
		}

		if rbacObject, err := kubeClient.Rbac().Roles(ns).Get(role.Name, metav1.GetOptions{}); err != nil {
			t.Fatal(err)
		} else if rbacObject.Labels["updated"] != "true" {
			t.Logf("not updated yet: %#v", rbacObject)
			return false, nil
		}

		if rbacObject, err := kubeClient.Rbac().ClusterRoleBindings().Get(clusterrolebinding.Name, metav1.GetOptions{}); err != nil {
			t.Fatal(err)
		} else if rbacObject.Labels["updated"] != "true" {
			t.Logf("not updated yet: %#v", rbacObject)
			return false, nil
		}

		if rbacObject, err := kubeClient.Rbac().RoleBindings(ns).Get(rolebinding.Name, metav1.GetOptions{}); err != nil {
			t.Fatal(err)
		} else if rbacObject.Labels["updated"] != "true" {
			t.Logf("not updated yet: %#v", rbacObject)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("updated objects did not propagate: %v", err)
	}

	// Delete
	err = originClient.ClusterRoles().Delete(clusterrole.Name)
	if err != nil {
		t.Fatal(err)
	}
	err = originClient.ClusterRoleBindings().Delete(clusterrolebinding.Name)
	if err != nil {
		t.Fatal(err)
	}
	err = originClient.Roles(ns).Delete(role.Name)
	if err != nil {
		t.Fatal(err)
	}
	err = originClient.RoleBindings(ns).Delete(rolebinding.Name)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure propagation
	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		if rbacObject, err := kubeClient.Rbac().ClusterRoles().Get(clusterrole.Name, metav1.GetOptions{}); err != nil && !kapierrors.IsNotFound(err) {
			t.Fatal(err)
		} else if err == nil {
			t.Logf("not deleted yet: %#v", rbacObject)
			return false, nil
		}

		if rbacObject, err := kubeClient.Rbac().Roles(ns).Get(role.Name, metav1.GetOptions{}); err != nil && !kapierrors.IsNotFound(err) {
			t.Fatal(err)
		} else if err == nil {
			t.Logf("not deleted yet: %#v", rbacObject)
			return false, nil
		}

		if rbacObject, err := kubeClient.Rbac().ClusterRoleBindings().Get(clusterrolebinding.Name, metav1.GetOptions{}); err != nil && !kapierrors.IsNotFound(err) {
			t.Fatal(err)
		} else if err == nil {
			t.Logf("not deleted yet: %#v", rbacObject)
			return false, nil
		}

		if rbacObject, err := kubeClient.Rbac().RoleBindings(ns).Get(rolebinding.Name, metav1.GetOptions{}); err != nil && !kapierrors.IsNotFound(err) {
			t.Fatal(err)
		} else if err == nil {
			t.Logf("not deleted yet: %#v", rbacObject)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("deleted objects did not propagate: %v", err)
	}
}
