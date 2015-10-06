// +build integration,etcd

package integration

import (
	"io/ioutil"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestBootstrapPolicyAuthenticatedUsersAgainstOpenshiftNamespace(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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

	if _, err := valerieOpenshiftClient.ImageStreams(openshiftSharedResourcesNamespace).List(labels.Everything(), fields.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageStreams(kapi.NamespaceDefault).List(labels.Everything(), fields.Everything()); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageStreamTags(openshiftSharedResourcesNamespace).Get("name", "tag"); !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageStreamTags(kapi.NamespaceDefault).Get("name", "tag"); err == nil || !kapierror.IsForbidden(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBootstrapPolicyOverwritePolicyCommand(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := client.ClusterPolicies().Delete(authorizationapi.PolicyName); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// after the policy is deleted, we must wait for it to be cleared from the policy cache
	err = wait.Poll(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		_, listErr := client.ClusterPolicies().List(labels.Everything(), fields.Everything())
		if listErr == nil {
			return false, nil
		}
		if !kapierror.IsForbidden(listErr) {
			t.Errorf("unexpected error: %v", listErr)
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("timeout: %v", err)
	}

	etcdClient, err := etcd.GetAndTestEtcdClient(masterConfig.EtcdClientInfo)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	etcdHelper, err := origin.NewEtcdStorage(etcdClient, masterConfig.EtcdStorageConfig.OpenShiftStorageVersion, masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := admin.OverwriteBootstrapPolicy(etcdHelper, masterConfig.PolicyConfig.BootstrapPolicyFile, admin.CreateBootstrapPolicyFileFullCommand, true, ioutil.Discard); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := client.ClusterPolicies().List(labels.Everything(), fields.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBootstrapPolicySelfSubjectAccessReviews(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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
	askCanICreatePolicyBindings := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{Verb: "create", Resource: "policybindings"},
	}
	subjectAccessReviewTest{
		localInterface: valerieOpenshiftClient.LocalSubjectAccessReviews("openshift"),
		localReview:    askCanICreatePolicyBindings,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "valerie" cannot create policybindings in project "openshift"`,
			Namespace: "openshift",
		},
	}.run(t)

	// I shouldn't be allowed to ask whether someone else can perform an action
	askCanClusterAdminsCreateProject := &authorizationapi.LocalSubjectAccessReview{
		Groups: sets.NewString("system:cluster-admins"),
		Action: authorizationapi.AuthorizationAttributes{Verb: "create", Resource: "projects"},
	}
	subjectAccessReviewTest{
		localInterface: valerieOpenshiftClient.LocalSubjectAccessReviews("openshift"),
		localReview:    askCanClusterAdminsCreateProject,
		err:            `User "valerie" cannot create localsubjectaccessreviews in project "openshift"`,
	}.run(t)

}

func TestSelfSubjectAccessReviewsNonExistingNamespace(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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

	valerieOpenshiftClient, _, _, err := testutil.GetClientForUser(valerieClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ensure that a SAR for a non-exisitng namespace gives a SAR response and not a
	// namespace doesn't exist response from admisison.
	askCanICreatePodsInNonExistingNamespace := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{Namespace: "foo", Verb: "create", Resource: "pods"},
	}
	subjectAccessReviewTest{
		description:    "ensure SAR for non-existing namespace does not leak namespace info",
		localInterface: valerieOpenshiftClient.LocalSubjectAccessReviews("foo"),
		localReview:    askCanICreatePodsInNonExistingNamespace,
		response: authorizationapi.SubjectAccessReviewResponse{
			Allowed:   false,
			Reason:    `User "valerie" cannot create pods in project "foo"`,
			Namespace: "foo",
		},
	}.run(t)
}
