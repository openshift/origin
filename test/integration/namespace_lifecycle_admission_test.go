package integration

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
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

	// Create an origin object
	route := &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route"},
		Spec:       routeapi.RouteSpec{To: routeapi.RouteTargetReference{Kind: "Service", Name: "test"}},
	}
	route, err = clusterAdminRouteClient.Routes(ns.Name).Create(route)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the origin finalizer is added
	ns, err = clusterAdminKubeClientset.Core().Namespaces().Get(ns.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, f := range ns.Spec.Finalizers {
		if f == projectapi.FinalizerOrigin {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected origin finalizer, got %#v", ns.Spec.Finalizers)
	}

	// Delete the namespace
	// We don't have to worry about racing the namespace deletion controller because we've only started the master
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
