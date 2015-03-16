// +build integration,!no-etcd

package integration

import (
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/openshift/origin/test/util"
)

func TestAuthenticatedUsersAgainstOpenshiftNamespace(t *testing.T) {
	_, clusterAdminKubeConfig, err := util.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := util.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
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

	openshiftSharedResourcesNamespace := "openshift"

	if _, err := valerieOpenshiftClient.Templates(openshiftSharedResourcesNamespace).List(labels.Everything(), labels.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.Templates(kapi.NamespaceDefault).List(labels.Everything(), labels.Everything()); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageRepositories(openshiftSharedResourcesNamespace).List(labels.Everything(), labels.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageRepositories(kapi.NamespaceDefault).List(labels.Everything(), labels.Everything()); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageRepositoryTags(openshiftSharedResourcesNamespace).Get("name", "tag"); !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageRepositoryTags(kapi.NamespaceDefault).Get("name", "tag"); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOverwritePolicyCommand(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := util.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client, err := util.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := client.Policies(masterConfig.PolicyConfig.MasterAuthorizationNamespace).Delete(authorizationapi.PolicyName); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := client.Policies(masterConfig.PolicyConfig.MasterAuthorizationNamespace).List(labels.Everything(), labels.Everything()); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("unexpected error: %v", err)
	}

	etcdHelper, err := etcd.NewOpenShiftEtcdHelper(masterConfig.EtcdClientInfo.URL)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := admin.OverwriteBootstrapPolicy(etcdHelper, masterConfig.PolicyConfig.MasterAuthorizationNamespace, masterConfig.PolicyConfig.BootstrapPolicyFile); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := client.Policies(masterConfig.PolicyConfig.MasterAuthorizationNamespace).List(labels.Everything(), labels.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
