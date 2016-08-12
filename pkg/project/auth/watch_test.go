package auth

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	projectapi "github.com/openshift/origin/pkg/project/api"
	projectcache "github.com/openshift/origin/pkg/project/cache"
)

func newTestWatcher(username string, groups []string, namespaces ...*kapi.Namespace) (*userProjectWatcher, *fakeAuthCache) {
	objects := []runtime.Object{}
	for i := range namespaces {
		objects = append(objects, namespaces[i])
	}
	mockClient := testclient.NewSimpleFake(objects...)

	projectCache := projectcache.NewProjectCache(mockClient.Namespaces(), "")
	projectCache.Run()
	fakeAuthCache := &fakeAuthCache{}

	return NewUserProjectWatcher(&user.DefaultInfo{Name: username, Groups: groups}, sets.NewString("*"), projectCache, fakeAuthCache, false), fakeAuthCache
}

type fakeAuthCache struct {
	namespaces []*kapi.Namespace

	removed []CacheWatcher
}

func (w *fakeAuthCache) RemoveWatcher(watcher CacheWatcher) {
	w.removed = append(w.removed, watcher)
}

func (w *fakeAuthCache) List(userInfo user.Info) (*kapi.NamespaceList, error) {
	ret := &kapi.NamespaceList{}
	if w.namespaces != nil {
		for i := range w.namespaces {
			ret.Items = append(ret.Items, *w.namespaces[i])
		}
	}

	return ret, nil
}

func TestFullIncoming(t *testing.T) {
	watcher, fakeAuthCache := newTestWatcher("bob", nil, newNamespaces("ns-01")...)
	watcher.cacheIncoming = make(chan watch.Event)

	go watcher.Watch()
	watcher.cacheIncoming <- watch.Event{Type: watch.Added}

	// this call should not block and we should see a failure
	watcher.GroupMembershipChanged("ns-01", sets.NewString("bob"), sets.String{})
	if len(fakeAuthCache.removed) != 1 {
		t.Errorf("should have removed self")
	}

	err := wait.PollImmediate(10*time.Millisecond, 5*time.Second, func() (done bool, err error) {
		if len(watcher.cacheError) > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	for {
		repeat := false
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				t.Fatalf("channel closed")
			}
			// this happens when the cacheIncoming block wins the select race
			if event.Type == watch.Added {
				repeat = true
				break
			}
			// this should be an error
			if event.Type != watch.Error {
				t.Errorf("expected error, got %v", event)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout")
		}
		if !repeat {
			break
		}
	}
}

func TestAddModifyDeleteEventsByUser(t *testing.T) {
	watcher, _ := newTestWatcher("bob", nil, newNamespaces("ns-01")...)
	go watcher.Watch()

	watcher.GroupMembershipChanged("ns-01", sets.NewString("bob"), sets.String{})
	select {
	case event := <-watcher.ResultChan():
		if event.Type != watch.Added {
			t.Errorf("expected added, got %v", event)
		}
		if event.Object.(*projectapi.Project).Name != "ns-01" {
			t.Errorf("expected %v, got %#v", "ns-01", event.Object)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}

	// the object didn't change, we shouldn't observe it
	watcher.GroupMembershipChanged("ns-01", sets.NewString("bob"), sets.String{})
	select {
	case event := <-watcher.ResultChan():
		t.Fatalf("unexpected event %v", event)
	case <-time.After(3 * time.Second):
	}

	watcher.GroupMembershipChanged("ns-01", sets.NewString("alice"), sets.String{})
	select {
	case event := <-watcher.ResultChan():
		if event.Type != watch.Deleted {
			t.Errorf("expected Deleted, got %v", event)
		}
		if event.Object.(*projectapi.Project).Name != "ns-01" {
			t.Errorf("expected %v, got %#v", "ns-01", event.Object)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestAddModifyDeleteEventsByGroup(t *testing.T) {
	watcher, _ := newTestWatcher("bob", []string{"group-one"}, newNamespaces("ns-01")...)
	go watcher.Watch()

	watcher.GroupMembershipChanged("ns-01", sets.String{}, sets.NewString("group-one"))
	select {
	case event := <-watcher.ResultChan():
		if event.Type != watch.Added {
			t.Errorf("expected added, got %v", event)
		}
		if event.Object.(*projectapi.Project).Name != "ns-01" {
			t.Errorf("expected %v, got %#v", "ns-01", event.Object)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}

	// the object didn't change, we shouldn't observe it
	watcher.GroupMembershipChanged("ns-01", sets.String{}, sets.NewString("group-one"))
	select {
	case event := <-watcher.ResultChan():
		t.Fatalf("unexpected event %v", event)
	case <-time.After(3 * time.Second):
	}

	watcher.GroupMembershipChanged("ns-01", sets.String{}, sets.NewString("group-two"))
	select {
	case event := <-watcher.ResultChan():
		if event.Type != watch.Deleted {
			t.Errorf("expected Deleted, got %v", event)
		}
		if event.Object.(*projectapi.Project).Name != "ns-01" {
			t.Errorf("expected %v, got %#v", "ns-01", event.Object)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}
}

func newNamespaces(names ...string) []*kapi.Namespace {
	ret := []*kapi.Namespace{}
	for _, name := range names {
		ret = append(ret, &kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: name}})
	}

	return ret
}
