package delegated

import (
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"

	"github.com/go-openapi/errors"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func TestDelegatedWait(t *testing.T) {
	cache := &testRoleBindingLister{}
	storage := &REST{roleBindings: cache}

	cache.namespacelister = &testRoleBindingNamespaceLister{}
	cache.namespacelister.bindings = map[string]*rbac.RoleBinding{}
	cache.namespacelister.bindings["anything"] = nil

	waitReturnedCh := waitForResultChannel(storage)

	select {
	case <-waitReturnedCh:
		t.Error("waitForRoleBinding() failed to block pending rolebinding creation")
	case <-time.After(1 * time.Second):
	}

	cache.addAdminRolebinding()

	select {
	case <-waitReturnedCh:
	case <-time.After(1 * time.Second):
		t.Error("waitForRoleBinding() failed to unblock after rolebinding creation")
	}
}

func waitForResultChannel(storage *REST) chan struct{} {
	ret := make(chan struct{})

	go func() {
		storage.waitForRoleBinding("foo", bootstrappolicy.AdminRoleName)
		close(ret)
	}()

	return ret
}

type testRoleBindingNamespaceLister struct {
	bindings map[string]*rbac.RoleBinding
	lock     sync.Mutex
}

func (t *testRoleBindingNamespaceLister) List(selector labels.Selector) (ret []*rbac.RoleBinding, err error) {
	return ret, nil
}

func (t *testRoleBindingNamespaceLister) Get(name string) (*rbac.RoleBinding, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.bindings[bootstrappolicy.AdminRoleName] != nil {
		return t.bindings[bootstrappolicy.AdminRoleName], nil
	}
	return nil, errors.NotFound("could not find role " + bootstrappolicy.AdminRoleName)
}

type testRoleBindingLister struct {
	namespacelister *testRoleBindingNamespaceLister
}

func (t *testRoleBindingLister) RoleBindings(namespace string) rbaclisters.RoleBindingNamespaceLister {
	return t.namespacelister
}

func (t *testRoleBindingLister) List(selector labels.Selector) ([]*rbac.RoleBinding, error) {
	return nil, nil
}

func (t *testRoleBindingLister) addAdminRolebinding() {
	t.namespacelister.lock.Lock()
	defer t.namespacelister.lock.Unlock()
	t.namespacelister.bindings[bootstrappolicy.AdminRoleName] = &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bootstrappolicy.AdminRoleName},
	}
}
