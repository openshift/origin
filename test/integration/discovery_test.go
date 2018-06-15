package integration

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestDiscoveryGroupVersions(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error starting test master: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resources, err := clusterAdminKubeClient.Discovery().ServerResources()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, resource := range resources {
		gv, err := schema.ParseGroupVersion(resource.GroupVersion)
		if err != nil {
			continue
		}
		allowedKubeVersions := sets.NewString(configapi.KubeAPIGroupsToAllowedVersions[gv.Group]...)
		allowedOriginVersions := sets.NewString(configapi.OriginAPIGroupsToAllowedVersions[gv.Group]...)
		if !allowedKubeVersions.Has(gv.Version) && !allowedOriginVersions.Has(gv.Version) {
			t.Errorf("Disallowed group/version found in discovery: %#v", gv)
		}
	}

	expectedGroupVersions := sets.NewString()
	for group, versions := range configapi.KubeAPIGroupsToAllowedVersions {
		for _, version := range versions {
			expectedGroupVersions.Insert(schema.GroupVersion{Group: group, Version: version}.String())
		}
	}
	for group, versions := range configapi.OriginAPIGroupsToAllowedVersions {
		for _, version := range versions {
			expectedGroupVersions.Insert(schema.GroupVersion{Group: group, Version: version}.String())
		}
	}

	discoveredGroupVersions := sets.NewString()
	for _, resource := range resources {
		gv, err := schema.ParseGroupVersion(resource.GroupVersion)
		if err != nil {
			t.Errorf("Error parsing gv %q: %v", resource.GroupVersion, err)
			continue
		}
		discoveredGroupVersions.Insert(gv.String())
	}
	if !reflect.DeepEqual(discoveredGroupVersions, expectedGroupVersions) {
		t.Fatalf("Expected %#v, got %#v", expectedGroupVersions.List(), discoveredGroupVersions.List())
	}

}
