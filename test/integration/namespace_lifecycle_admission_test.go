package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeclient "github.com/openshift/origin/pkg/route/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestNamespaceLifecycleAdmission(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminRouteClient := routeclient.NewForConfigOrDie(clusterAdminClientConfig).Route()
	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeInternalClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	for _, ns := range []string{"default", "openshift", "openshift-infra"} {
		if err := clusterAdminKubeClientset.Core().Namespaces().Delete(ns, nil); err == nil {
			t.Fatalf("expected error deleting %q namespace, got none", ns)
		}
	}

	// Create a namespace directly (not via a project)
	ns := &kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	ns, err = clusterAdminKubeClientset.Core().Namespaces().Create(ns)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns.Spec.Finalizers) == 0 {
		t.Fatal("expected at least one finalizer")
	}
	found := false
	for _, f := range ns.Spec.Finalizers {
		if f == projectapi.FinalizerOrigin {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("didn't expect origin finalizer to be present, got %#v", ns.Spec.Finalizers)
	}
	nsWatch, err := clusterAdminKubeClientset.Core().Namespaces().Watch(metav1.SingleObject(ns.ObjectMeta))
	if err != nil {
		t.Fatal(err)
	}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ns, err = clusterAdminKubeClientset.Core().Namespaces().Get("test", metav1.GetOptions{})
		if err != nil {
			return err
		}
		ns.Spec.Finalizers = append(ns.Spec.Finalizers, projectapi.FinalizerOrigin)
		t.Log(spew.Sdump(ns))
		afterUpdate, err := clusterAdminKubeClientset.Core().Namespaces().Finalize(ns)
		t.Log(spew.Sdump(afterUpdate))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	// watch to see the finalizer added
	for {
		found := false
		var event watch.Event
		select {
		case event = <-nsWatch.ResultChan():
			t.Log(spew.Sdump(event))
			if event.Type != watch.Modified {
				t.Fatal(spew.Sdump(event))
			}
			updatedNamespace := event.Object.(*kapi.Namespace)
			for _, curr := range updatedNamespace.Spec.Finalizers {
				if curr == projectapi.FinalizerOrigin {
					found = true
				}
			}
		case <-time.After(5 * time.Second):
			t.Fatal("too long")
		}
		if found {
			break
		}
		t.Log("not found yet")
	}
	// watch to see the finalizer removed by controller
	select {
	case obj := <-nsWatch.ResultChan():
		if obj.Type != watch.Modified {
			t.Fatal(spew.Sdump(obj))
		}
		updatedNamespace := obj.Object.(*kapi.Namespace)
		found := false
		for _, curr := range updatedNamespace.Spec.Finalizers {
			if curr == projectapi.FinalizerOrigin {
				found = true
			}
		}
		if found {
			t.Fatal(spew.Sdump(obj))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("too long")
	}

	// Create an origin object
	route := &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route"},
		Spec:       routeapi.RouteSpec{To: routeapi.RouteTargetReference{Kind: "Service", Name: "test"}},
	}
	route, err = clusterAdminRouteClient.Routes(ns.Name).Create(route)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the namespace
	err = clusterAdminKubeClientset.Core().Namespaces().Delete(ns.Name, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Try to create an origin object in a terminating namespace and ensure it is forbidden
	route = &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route2"},
		Spec:       routeapi.RouteSpec{To: routeapi.RouteTargetReference{Kind: "Service", Name: "test"}},
	}
	_, err = clusterAdminRouteClient.Routes(ns.Name).Create(route)
	if err == nil || !strings.Contains(err.Error(), "it is being terminated") {
		t.Fatalf("Expected forbidden error because of a terminating namespace, got %v", err)
	}
}
