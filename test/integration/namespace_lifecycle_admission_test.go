package integration

import (
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestNamespaceLifecycleAdmission(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	for _, ns := range []string{"default", "openshift", "openshift-infra"} {
		if err := clusterAdminKubeClient.Namespaces().Delete(ns); err == nil {
			t.Fatalf("expected error deleting %q namespace, got none", ns)
		}
	}

	// Create a namespace directly (not via a project)
	ns := &kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: "test"}}
	ns, err = clusterAdminKubeClient.Namespaces().Create(ns)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns.Spec.Finalizers) == 0 {
		t.Fatal("expected at least one finalizer")
	}
	found := false
	for _, f := range ns.Spec.Finalizers {
		if f == api.FinalizerOrigin {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("didn't expect origin finalizer to be present, got %#v", ns.Spec.Finalizers)
	}

	// Create an origin object
	route := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "route"},
		Spec:       routeapi.RouteSpec{To: routeapi.RouteTargetReference{Kind: "Service", Name: "test"}},
	}
	route, err = clusterAdminClient.Routes(ns.Name).Create(route)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the origin finalizer is added
	ns, err = clusterAdminKubeClient.Namespaces().Get(ns.Name)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, f := range ns.Spec.Finalizers {
		if f == api.FinalizerOrigin {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected origin finalizer, got %#v", ns.Spec.Finalizers)
	}

	// Delete the namespace
	// We don't have to worry about racing the namespace deletion controller because we've only started the master
	err = clusterAdminKubeClient.Namespaces().Delete(ns.Name)
	if err != nil {
		t.Fatal(err)
	}

	// Try to create an origin object in a terminating namespace and ensure it is forbidden
	route = &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "route2"},
		Spec:       routeapi.RouteSpec{To: routeapi.RouteTargetReference{Kind: "Service", Name: "test"}},
	}
	_, err = clusterAdminClient.Routes(ns.Name).Create(route)
	if err == nil || !strings.Contains(err.Error(), "it is being terminated") {
		t.Fatalf("Expected forbidden error because of a terminating namespace, got %v", err)
	}
}
