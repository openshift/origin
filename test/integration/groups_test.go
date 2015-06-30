// +build integration,!no-etcd

package integration

import (
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
	testutil "github.com/openshift/origin/test/util"
)

func TestBasicGroupManipulation(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig := *clusterAdminClientConfig
	valerieClientConfig.Username = ""
	valerieClientConfig.Password = ""
	valerieClientConfig.BearerToken = ""
	valerieClientConfig.CertFile = ""
	valerieClientConfig.KeyFile = ""
	valerieClientConfig.CertData = nil
	valerieClientConfig.KeyData = nil

	accessToken, err := tokencmd.RequestToken(&valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken
	valerieOpenshiftClient, err := client.New(&valerieClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// make sure we don't get back system groups
	firstValerie, err := clusterAdminClient.Users().Get("valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(firstValerie.Groups) != 0 {
		t.Errorf("unexpected groups: %v", firstValerie.Groups)
	}

	// make sure that user/~ returns groups for unbacked users
	expectedClusterAdminGroups := []string{"system:cluster-admins"}
	clusterAdminUser, err := clusterAdminClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(clusterAdminUser.Groups, expectedClusterAdminGroups) {
		t.Errorf("expected %v, got %v", clusterAdminUser.Groups, expectedClusterAdminGroups)
	}

	valerieGroups := []string{"thegroup"}
	firstValerie.Groups = append(firstValerie.Groups, valerieGroups...)
	_, err = clusterAdminClient.Users().Update(firstValerie)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// make sure that user/~ doesn't get back system groups when it merges
	secondValerie, err := valerieOpenshiftClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(secondValerie.Groups, valerieGroups) {
		t.Errorf("expected %v, got %v", secondValerie.Groups, valerieGroups)
	}

	_, err = valerieOpenshiftClient.Projects().Get("empty")
	if err == nil {
		t.Fatalf("expected error")
	}

	emptyProject := &projectapi.Project{}
	emptyProject.Name = "empty"
	_, err = clusterAdminClient.Projects().Create(emptyProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBinding := &authorizationapi.RoleBinding{}
	roleBinding.Name = "admins"
	roleBinding.RoleRef.Name = "admin"
	roleBinding.Groups = util.NewStringSet(valerieGroups...)
	_, err = clusterAdminClient.RoleBindings("empty").Create(roleBinding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(valerieOpenshiftClient, "empty", "get", "pods", true); err != nil {
		t.Error(err)
	}

	// make sure that user groups are respected for policy
	_, err = valerieOpenshiftClient.Projects().Get("empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

}
