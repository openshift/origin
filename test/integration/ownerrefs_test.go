package integration

import (
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestOwnerRefRestriction(t *testing.T) {
	// functionality of the plugin has a unit test, we just need to make sure its called.
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	originClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = originClient.ClusterRoles().Create(&authorizationapi.ClusterRole{
		ObjectMeta: kapi.ObjectMeta{
			Name: "create-svc",
		},
		Rules: []authorizationapi.PolicyRule{
			authorizationapi.NewRule("create").Groups(kapi.GroupName).Resources("services").RuleOrDie(),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := testserver.CreateNewProject(originClient, *clientConfig, "foo", "admin-user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, creatorClient, _, err := testutil.GetClientForUser(*clientConfig, "creator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = originClient.RoleBindings("foo").Create(&authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name: "create-svc",
		},
		RoleRef:  kapi.ObjectReference{Name: "create-svc"},
		Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "creator"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(originClient, "foo", "create", kapi.Resource("services"), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = creatorClient.Services("foo").Create(&kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "my-service",
			OwnerReferences: []kapi.OwnerReference{{}},
		},
	})
	if err == nil {
		t.Fatalf("missing err")
	}
	if !kapierrors.IsForbidden(err) || !strings.Contains(err.Error(), "cannot set an ownerRef on a resource you can't delete") {
		t.Fatalf("expecting cannot set an ownerRef on a resource you can't delete, got %v", err)
	}
}
