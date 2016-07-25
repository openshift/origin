package integration

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestDiscoveryGroupVersions(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error starting test master: %v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resources, err := clusterAdminKubeClient.Discovery().ServerResources()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, resource := range resources {
		gv, err := unversioned.ParseGroupVersion(resource.GroupVersion)
		if err != nil {
			continue
		}
		allowedVersions := sets.NewString(configapi.KubeAPIGroupsToAllowedVersions[gv.Group]...)
		if !allowedVersions.Has(gv.Version) {
			t.Errorf("Disallowed group/version found in discovery: %#v", gv)
		}
	}

	expectedGroupVersions := sets.NewString()
	for group, versions := range configapi.KubeAPIGroupsToAllowedVersions {
		for _, version := range versions {
			expectedGroupVersions.Insert(unversioned.GroupVersion{Group: group, Version: version}.String())
		}
	}

	discoveredGroupVersions := sets.StringKeySet(resources)
	if !reflect.DeepEqual(discoveredGroupVersions, expectedGroupVersions) {
		t.Fatalf("Expected %#v, got %#v", expectedGroupVersions.List(), discoveredGroupVersions.List())
	}

}
