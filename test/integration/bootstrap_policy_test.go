// +build integration,!no-etcd

package integration

import (
	"io/ioutil"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
)

func TestAuthenticatedUsersAgainstOpenshiftNamespace(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
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

	if _, err := valerieOpenshiftClient.Templates(openshiftSharedResourcesNamespace).List(labels.Everything(), fields.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.Templates(kapi.NamespaceDefault).List(labels.Everything(), fields.Everything()); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageRepositories(openshiftSharedResourcesNamespace).List(labels.Everything(), fields.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageRepositories(kapi.NamespaceDefault).List(labels.Everything(), fields.Everything()); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageRepositoryTags(openshiftSharedResourcesNamespace).Get("name", "tag"); !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageRepositoryTags(kapi.NamespaceDefault).Get("name", "tag"); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOverwritePolicyCommand(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := client.Policies(masterConfig.PolicyConfig.MasterAuthorizationNamespace).Delete(authorizationapi.PolicyName); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := client.Policies(masterConfig.PolicyConfig.MasterAuthorizationNamespace).List(labels.Everything(), fields.Everything()); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}

	etcdHelper, err := etcd.NewOpenShiftEtcdHelper(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := admin.OverwriteBootstrapPolicy(etcdHelper, masterConfig.PolicyConfig.MasterAuthorizationNamespace, masterConfig.PolicyConfig.BootstrapPolicyFile, admin.CreateBootstrapPolicyFileFullCommand, true, ioutil.Discard); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := client.Policies(masterConfig.PolicyConfig.MasterAuthorizationNamespace).List(labels.Everything(), fields.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSelfSubjectAccessReviews(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
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

	// can I get a subjectaccessreview on myself even if I have no rights to do it generally
	askCanICreatePolicyBindings := &authorizationapi.SubjectAccessReview{Verb: "create", Resource: "policybindings"}
	subjectAccessReviewTest{
		clientInterface: valerieOpenshiftClient.SubjectAccessReviews("openshift"),
		review:          askCanICreatePolicyBindings,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    "denied by default",
			Namespace: "openshift",
		},
	}.run(t)

	// I shouldn't be allowed to ask whether someone else can perform an action
	askCanClusterAdminsCreateProject := &authorizationapi.SubjectAccessReview{Groups: util.NewStringSet("system:cluster-admins"), Verb: "create", Resource: "projects"}
	subjectAccessReviewTest{
		clientInterface: valerieOpenshiftClient.SubjectAccessReviews("openshift"),
		review:          askCanClusterAdminsCreateProject,
		err:             "forbidden",
	}.run(t)

}
