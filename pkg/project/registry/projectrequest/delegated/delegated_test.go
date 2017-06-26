package delegated

import (
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func TestDelegatedWait(t *testing.T) {
	cache := &testReadOnlyPolicyBinding{}
	storage := &REST{policyBindings: cache}

	cache.binding = &authorizationapi.PolicyBinding{}
	cache.binding.RoleBindings = map[string]*authorizationapi.RoleBinding{}
	cache.binding.RoleBindings["anything"] = nil

	callReturnedCh := testWait(storage)

	select {
	case <-callReturnedCh:
		t.Errorf("too fast, should have blocked")
	case <-time.After(1 * time.Second):
	}

	func() {
		cache.lock.Lock()
		defer cache.lock.Unlock()
		cache.binding.RoleBindings[bootstrappolicy.AdminRoleName] = nil
	}()

	select {
	case <-callReturnedCh:
	case <-time.After(1 * time.Second):
		t.Errorf("too slow, should have returned")
	}
}

func testWait(storage *REST) chan struct{} {
	ret := make(chan struct{})

	go func() {
		storage.waitForRoleBinding("foo", bootstrappolicy.AdminRoleName)
		close(ret)
	}()

	return ret
}

type testReadOnlyPolicyBinding struct {
	binding *authorizationapi.PolicyBinding
	lock    sync.Mutex
}

func (t *testReadOnlyPolicyBinding) PolicyBindings(namespace string) authorizationlister.PolicyBindingNamespaceLister {
	return t
}

// ReadOnlyPolicyBindingInterface exposes methods on PolicyBindings resources
func (t *testReadOnlyPolicyBinding) List(label labels.Selector) ([]*authorizationapi.PolicyBinding, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var ret []*authorizationapi.PolicyBinding
	if t.binding != nil {
		returning := authorizationapi.PolicyBinding{}
		returning.RoleBindings = map[string]*authorizationapi.RoleBinding{}
		for k, v := range t.binding.RoleBindings {
			returning.RoleBindings[k] = v
		}

		ret = []*authorizationapi.PolicyBinding{&returning}
	}

	return ret, nil
}

func (t *testReadOnlyPolicyBinding) Get(name string) (*authorizationapi.PolicyBinding, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.binding, nil
}
