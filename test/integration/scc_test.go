package integration

import (
	"strings"
	"testing"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"

	"github.com/openshift/origin/pkg/security/apis/security"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestPodUpdateSCCEnforcement(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projectName := "hammer-project"

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldKubeClient, _, err := testutil.GetClientForUser(clusterAdminClientConfig, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClientset, projectName, []string{"default"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// so cluster-admin can create privileged pods, but harold cannot.  This means that harold should not be able
	// to update the privileged pods either, even if he lies about its privileged nature
	privilegedPod := getPrivilegedPod("unsafe")

	if _, err := haroldKubeClient.Core().Pods(projectName).Create(privilegedPod); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	actualPod, err := clusterAdminKubeClientset.Core().Pods(projectName).Create(privilegedPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actualPod.Spec.Containers[0].Image = "something-nefarious"
	if _, err := haroldKubeClient.Core().Pods(projectName).Update(actualPod); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	// try to lie about the privileged nature
	actualPod.Spec.SecurityContext.HostPID = false
	if _, err := haroldKubeClient.Core().Pods(projectName).Update(actualPod); err == nil {
		t.Fatalf("missing error: %v", err)
	}
}

func TestAllowedSCCViaRBAC(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	project1 := "project1"
	project2 := "project2"

	user1 := "user1"
	user2 := "user2"

	clusterRole := "all-scc"
	rule := rbac.NewRule("use").Groups("security.openshift.io").Resources("securitycontextconstraints").RuleOrDie()

	// set a up cluster role that allows access to all SCCs
	if _, err := clusterAdminKubeClientset.Rbac().ClusterRoles().Create(
		&rbac.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: clusterRole},
			Rules:      []rbac.PolicyRule{rule},
		},
	); err != nil {
		t.Fatal(err)
	}

	// set up 2 projects for 2 users

	user1Client, user1Config, err := testserver.CreateNewProject(clusterAdminClientConfig, project1, user1)
	if err != nil {
		t.Fatal(err)
	}
	user1SecurityClient := securityclient.NewForConfigOrDie(user1Config)

	user2Client, user2Config, err := testserver.CreateNewProject(clusterAdminClientConfig, project2, user2)
	if err != nil {
		t.Fatal(err)
	}
	user2SecurityClient := securityclient.NewForConfigOrDie(user2Config)

	// make sure the SAs are ready so we can deploy pods

	if err := testserver.WaitForServiceAccounts(user1Client, project1, []string{"default"}); err != nil {
		t.Fatal(err)
	}

	if err := testserver.WaitForServiceAccounts(user2Client, project2, []string{"default"}); err != nil {
		t.Fatal(err)
	}

	// user1 cannot make a privileged pod
	if _, err := user1Client.Core().Pods(project1).Create(getPrivilegedPod("test1")); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden for user1: %v", err)
	}

	// user2 cannot make a privileged pod
	if _, err := user2Client.Core().Pods(project2).Create(getPrivilegedPod("test2")); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden for user2: %v", err)
	}

	// this should allow user1 to make a privileged pod in project1
	rb := rbac.NewRoleBindingForClusterRole(clusterRole, project1).Users(user1).BindingOrDie()
	if _, err := clusterAdminKubeClientset.Rbac().RoleBindings(project1).Create(&rb); err != nil {
		t.Fatal(err)
	}

	// this should allow user1 to make pods in project2
	rbEditUser1Project2 := rbac.NewRoleBindingForClusterRole("edit", project2).Users(user1).BindingOrDie()
	if _, err := clusterAdminKubeClientset.Rbac().RoleBindings(project2).Create(&rbEditUser1Project2); err != nil {
		t.Fatal(err)
	}

	// this should allow user2 to make pods in project1
	rbEditUser2Project1 := rbac.NewRoleBindingForClusterRole("edit", project1).Users(user2).BindingOrDie()
	if _, err := clusterAdminKubeClientset.Rbac().RoleBindings(project1).Create(&rbEditUser2Project1); err != nil {
		t.Fatal(err)
	}

	// this should allow user2 to make a privileged pod in all projects
	crb := rbac.NewClusterBinding(clusterRole).Users(user2).BindingOrDie()
	if _, err := clusterAdminKubeClientset.Rbac().ClusterRoleBindings().Create(&crb); err != nil {
		t.Fatal(err)
	}

	// wait for RBAC to catch up to user1 role binding for SCC
	if err := testutil.WaitForPolicyUpdate(user1Client.Authorization(), project1, rule.Verbs[0],
		schema.GroupResource{Group: rule.APIGroups[0], Resource: rule.Resources[0]}, true); err != nil {
		t.Fatal(err)
	}

	// wait for RBAC to catch up to user1 role binding for edit
	if err := testutil.WaitForPolicyUpdate(user1Client.Authorization(), project2, "create",
		schema.GroupResource{Resource: "pods"}, true); err != nil {
		t.Fatal(err)
	}

	// wait for RBAC to catch up to user2 role binding
	if err := testutil.WaitForPolicyUpdate(user2Client.Authorization(), project1, "create",
		schema.GroupResource{Resource: "pods"}, true); err != nil {
		t.Fatal(err)
	}

	// wait for RBAC to catch up to user2 cluster role binding
	if err := testutil.WaitForClusterPolicyUpdate(user2Client.Authorization(), rule.Verbs[0],
		schema.GroupResource{Group: rule.APIGroups[0], Resource: rule.Resources[0]}, true); err != nil {
		t.Fatal(err)
	}

	// user1 can make a privileged pod in project1
	if _, err := user1Client.Core().Pods(project1).Create(getPrivilegedPod("test3")); err != nil {
		t.Fatalf("user1 failed to create pod in project1 via local binding: %v", err)
	}

	// user1 cannot make a privileged pod in project2
	if _, err := user1Client.Core().Pods(project2).Create(getPrivilegedPod("test4")); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden for user1 in project2: %v", err)
	}

	// user2 can make a privileged pod in project1
	if _, err := user2Client.Core().Pods(project1).Create(getPrivilegedPod("test5")); err != nil {
		t.Fatalf("user2 failed to create pod in project1 via cluster binding: %v", err)
	}

	// user2 can make a privileged pod in project2
	if _, err := user2Client.Core().Pods(project2).Create(getPrivilegedPod("test6")); err != nil {
		t.Fatalf("user2 failed to create pod in project2 via cluster binding: %v", err)
	}

	// make sure PSP self subject review works since that is based by the same SCC logic but has different wiring

	// user1 can make a privileged pod in project1
	user1PSPReview, err := user1SecurityClient.PodSecurityPolicySelfSubjectReviews(project1).Create(runAsRootPSPSSR())
	if err != nil {
		t.Fatal(err)
	}
	if allowedBy := user1PSPReview.Status.AllowedBy; allowedBy == nil || allowedBy.Name != "anyuid" {
		t.Fatalf("user1 failed PSP SSR in project1: %v", allowedBy)
	}

	// user2 can make a privileged pod in project2
	user2PSPReview, err := user2SecurityClient.PodSecurityPolicySelfSubjectReviews(project2).Create(runAsRootPSPSSR())
	if err != nil {
		t.Fatal(err)
	}
	if allowedBy := user2PSPReview.Status.AllowedBy; allowedBy == nil || allowedBy.Name != "anyuid" {
		t.Fatalf("user2 failed PSP SSR in project2: %v", allowedBy)
	}
}

func isForbiddenBySCC(err error) bool {
	return kapierror.IsForbidden(err) && strings.Contains(err.Error(), "unable to validate against any security context constraint")
}

func getPrivilegedPod(name string) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{Name: "first", Image: "something-innocuous"},
			},
			SecurityContext: &kapi.PodSecurityContext{
				HostPID: true,
			},
		},
	}
}

func runAsRootPSPSSR() *security.PodSecurityPolicySelfSubjectReview {
	return &security.PodSecurityPolicySelfSubjectReview{
		Spec: security.PodSecurityPolicySelfSubjectReviewSpec{
			Template: kapi.PodTemplateSpec{
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "fake",
							Image: "fake",
							SecurityContext: &kapi.SecurityContext{
								RunAsUser: new(int64), // root
							},
						},
					},
				},
			},
		},
	}
}
