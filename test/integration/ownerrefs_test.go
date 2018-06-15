package integration

import (
	"strings"
	"testing"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestOwnerRefRestriction(t *testing.T) {
	// functionality of the plugin has a unit test, we just need to make sure its called.
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(clientConfig).Authorization()

	_, err = clusterAdminAuthorizationClient.ClusterRoles().Create(&authorizationapi.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "create-svc",
		},
		Rules: []authorizationapi.PolicyRule{
			authorizationapi.NewRule("create", "update").Groups(kapi.GroupName).Resources("services").RuleOrDie(),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := testserver.CreateNewProject(clientConfig, "foo", "admin-user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	creatorClient, _, err := testutil.GetClientForUser(clientConfig, "creator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = clusterAdminAuthorizationClient.RoleBindings("foo").Create(&authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "create-svc",
		},
		RoleRef:  kapi.ObjectReference{Name: "create-svc"},
		Subjects: []kapi.ObjectReference{{Kind: authorizationapi.UserKind, Name: "creator"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(creatorClient.Authorization(), "foo", "create", kapi.Resource("services"), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actual, err := creatorClient.Core().Services("foo").Create(&kapi.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-service",
		},
		Spec: kapi.ServiceSpec{
			Ports: []kapi.ServicePort{
				{Port: 80},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	actual.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "foo",
		Kind:       "bar",
		Name:       "baz",
		UID:        types.UID("baq"),
	}}
	actual, err = creatorClient.Core().Services("foo").Update(actual)
	if err == nil {
		t.Fatalf("missing error")
	}
	if !kapierrors.IsForbidden(err) || !strings.Contains(err.Error(), "cannot set an ownerRef on a resource you can't delete") {
		t.Fatalf("expecting cannot set an ownerRef on a resource you can't delete, got %v", err)
	}
}
