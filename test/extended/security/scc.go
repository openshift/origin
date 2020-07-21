package security

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo"
	securityv1 "github.com/openshift/api/security/v1"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	"github.com/openshift/origin/test/extended/authorization"
	exutil "github.com/openshift/origin/test/extended/util"
	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
)

var _ = g.Describe("[sig-auth][Feature:SecurityContextConstraints] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("scc")
	ctx := context.Background()

	g.It("TestPodUpdateSCCEnforcement", func() {
		t := g.GinkgoT()

		clusterAdminKubeClientset := oc.AdminKubeClient()

		projectName := oc.Namespace()
		haroldUser := oc.CreateUser("harold-").Name
		haroldClientConfig := oc.GetClientConfigForUser(haroldUser)
		haroldKubeClient := kubernetes.NewForConfigOrDie(haroldClientConfig)
		authorization.AddUserAdminToProject(oc, projectName, haroldUser)

		// so cluster-admin can create privileged pods, but harold cannot.  This means that harold should not be able
		// to update the privileged pods either, even if he lies about its privileged nature
		privilegedPod := getPrivilegedPod("unsafe")

		if _, err := haroldKubeClient.CoreV1().Pods(projectName).Create(ctx, privilegedPod, metav1.CreateOptions{}); !isForbiddenBySCC(err) {
			t.Fatalf("missing forbidden: %v", err)
		}

		actualPod, err := clusterAdminKubeClientset.CoreV1().Pods(projectName).Create(ctx, privilegedPod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		actualPod.Spec.Containers[0].Image = "something-nefarious"
		if _, err := haroldKubeClient.CoreV1().Pods(projectName).Update(ctx, actualPod, metav1.UpdateOptions{}); !isForbiddenBySCC(err) {
			t.Fatalf("missing forbidden: %v", err)
		}

		// try to connect to /exec subresource as harold
		haroldCorev1Rest := haroldKubeClient.CoreV1().RESTClient()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result := &metav1.Status{}
		err = haroldCorev1Rest.Post().
			Resource("pods").
			Namespace(projectName).
			Name(actualPod.Name).
			SubResource("exec").
			Param("container", "first").
			Do(ctx).
			Into(result)
		if !isForbiddenBySCCExecRestrictions(err) {
			t.Fatalf("missing forbidden by SCCExecRestrictions: %v", err)
		}

		// try to lie about the privileged nature
		actualPod.Spec.HostPID = false
		if _, err := haroldKubeClient.CoreV1().Pods(projectName).Update(context.Background(), actualPod, metav1.UpdateOptions{}); err == nil {
			t.Fatalf("missing error: %v", err)
		}
	})
})

var _ = g.Describe("[sig-auth][Feature:SecurityContextConstraints] ", func() {
	ctx := context.Background()

	defer g.GinkgoRecover()
	oc := exutil.NewCLI("scc")

	g.It("TestAllowedSCCViaRBAC", func() {
		t := g.GinkgoT()

		clusterAdminKubeClientset := oc.AdminKubeClient()

		project1 := oc.Namespace()
		project2 := oc.CreateProject()
		user1 := oc.CreateUser("user1-").Name
		user2 := oc.CreateUser("user2-").Name

		clusterRole := "all-scc-" + oc.Namespace()
		rule := rbacv1helpers.NewRule("use").Groups("security.openshift.io").Resources("securitycontextconstraints").RuleOrDie()

		// set a up cluster role that allows access to all SCCs
		if _, err := clusterAdminKubeClientset.RbacV1().ClusterRoles().Create(
			ctx,
			&rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: clusterRole},
				Rules:      []rbacv1.PolicyRule{rule},
			},
			metav1.CreateOptions{},
		); err != nil {
			t.Fatal(err)
		}
		oc.AddExplicitResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("clusterroles"), "", clusterRole)

		// set up 2 projects for 2 users

		authorization.AddUserAdminToProject(oc, project1, user1)
		user1Config := oc.GetClientConfigForUser(user1)
		user1Client := kubernetes.NewForConfigOrDie(user1Config)
		user1SecurityClient := securityv1client.NewForConfigOrDie(user1Config)

		authorization.AddUserAdminToProject(oc, project2, user2)
		user2Config := oc.GetClientConfigForUser(user2)
		user2Client := kubernetes.NewForConfigOrDie(user2Config)
		user2SecurityClient := securityv1client.NewForConfigOrDie(user2Config)

		createOpts := metav1.CreateOptions{}

		// user1 cannot make a privileged pod
		if _, err := user1Client.CoreV1().Pods(project1).Create(ctx, getPrivilegedPod("test1"), createOpts); !isForbiddenBySCC(err) {
			t.Fatalf("missing forbidden for user1: %v", err)
		}

		// user2 cannot make a privileged pod
		if _, err := user2Client.CoreV1().Pods(project2).Create(ctx, getPrivilegedPod("test2"), createOpts); !isForbiddenBySCC(err) {
			t.Fatalf("missing forbidden for user2: %v", err)
		}

		// this should allow user1 to make a privileged pod in project1
		rb := rbacv1helpers.NewRoleBindingForClusterRole(clusterRole, project1).Users(user1).BindingOrDie()
		if _, err := clusterAdminKubeClientset.RbacV1().RoleBindings(project1).Create(ctx, &rb, createOpts); err != nil {
			t.Fatal(err)
		}

		// this should allow user1 to make pods in project2
		rbEditUser1Project2 := rbacv1helpers.NewRoleBindingForClusterRole("edit", project2).Users(user1).BindingOrDie()
		if _, err := clusterAdminKubeClientset.RbacV1().RoleBindings(project2).Create(ctx, &rbEditUser1Project2, createOpts); err != nil {
			t.Fatal(err)
		}

		// this should allow user2 to make pods in project1
		rbEditUser2Project1 := rbacv1helpers.NewRoleBindingForClusterRole("edit", project1).Users(user2).BindingOrDie()
		if _, err := clusterAdminKubeClientset.RbacV1().RoleBindings(project1).Create(ctx, &rbEditUser2Project1, createOpts); err != nil {
			t.Fatal(err)
		}

		// this should allow user2 to make a privileged pod in all projects
		crb := rbacv1helpers.NewClusterBinding(clusterRole).Users(user2).BindingOrDie()
		if _, err := clusterAdminKubeClientset.RbacV1().ClusterRoleBindings().Create(ctx, &crb, createOpts); err != nil {
			t.Fatal(err)
		}
		oc.AddExplicitResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings"), "", crb.Name)

		// wait for RBAC to catch up to user1 role binding for SCC
		if err := oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
			Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
					Namespace: project1,
					Verb:      rule.Verbs[0],
					Group:     rule.APIGroups[0],
					Resource:  rule.Resources[0],
				},
			},
		}, user1); err != nil {
			t.Fatal(err)
		}

		// wait for RBAC to catch up to user1 role binding for edit
		if err := oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
			Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
					Namespace: project2,
					Verb:      "create",
					Group:     "",
					Resource:  "pods",
				},
			},
		}, user1); err != nil {
			t.Fatal(err)
		}

		// wait for RBAC to catch up to user2 role binding
		if err := oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
			Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
					Namespace: project1,
					Verb:      "create",
					Group:     "",
					Resource:  "pods",
				},
			},
		}, user2); err != nil {
			t.Fatal(err)
		}

		// wait for RBAC to catch up to user2 cluster role binding
		if err := oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
			Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
					Namespace: project2,
					Verb:      rule.Verbs[0],
					Group:     rule.APIGroups[0],
					Resource:  rule.Resources[0],
				},
			},
		}, user2); err != nil {
			t.Fatal(err)
		}

		// user1 can make a privileged pod in project1
		if _, err := user1Client.CoreV1().Pods(project1).Create(ctx, getPrivilegedPod("test3"), createOpts); err != nil {
			t.Fatalf("user1 failed to create pod in project1 via local binding: %v", err)
		}

		// user1 cannot make a privileged pod in project2
		if _, err := user1Client.CoreV1().Pods(project2).Create(ctx, getPrivilegedPod("test4"), createOpts); !isForbiddenBySCC(err) {
			t.Fatalf("missing forbidden for user1 in project2: %v", err)
		}

		// user2 can make a privileged pod in project1
		if _, err := user2Client.CoreV1().Pods(project1).Create(ctx, getPrivilegedPod("test5"), createOpts); err != nil {
			t.Fatalf("user2 failed to create pod in project1 via cluster binding: %v", err)
		}

		// user2 can make a privileged pod in project2
		if _, err := user2Client.CoreV1().Pods(project2).Create(ctx, getPrivilegedPod("test6"), createOpts); err != nil {
			t.Fatalf("user2 failed to create pod in project2 via cluster binding: %v", err)
		}

		// make sure PSP self subject review works since that is based by the same SCC logic but has different wiring

		// user1 can make a privileged pod in project1
		user1PSPReview, err := user1SecurityClient.PodSecurityPolicySelfSubjectReviews(project1).Create(ctx, runAsRootPSPSSR(), createOpts)
		if err != nil {
			t.Fatal(err)
		}
		if allowedBy := user1PSPReview.Status.AllowedBy; allowedBy == nil || allowedBy.Name != "anyuid" {
			t.Fatalf("user1 failed PSP SSR in project1: %v", allowedBy)
		}

		// user2 can make a privileged pod in project2
		user2PSPReview, err := user2SecurityClient.PodSecurityPolicySelfSubjectReviews(project2).Create(ctx, runAsRootPSPSSR(), createOpts)
		if err != nil {
			t.Fatal(err)
		}
		if allowedBy := user2PSPReview.Status.AllowedBy; allowedBy == nil || allowedBy.Name != "anyuid" {
			t.Fatalf("user2 failed PSP SSR in project2: %v", allowedBy)
		}
	})
})

func isForbiddenBySCC(err error) bool {
	return kapierror.IsForbidden(err) && strings.Contains(err.Error(), "unable to validate against any security context constraint")
}

func isForbiddenBySCCExecRestrictions(err error) bool {
	return kapierror.IsForbidden(err) && strings.Contains(err.Error(), "pod's security context exceeds your permissions")
}

func getPrivilegedPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"e2e.openshift.io/unschedulable": "should-not-run",
			},
			Containers: []corev1.Container{
				{Name: "first", Image: "something-innocuous"},
			},
			HostPID: true,
		},
	}
}

func runAsRootPSPSSR() *securityv1.PodSecurityPolicySelfSubjectReview {
	return &securityv1.PodSecurityPolicySelfSubjectReview{
		Spec: securityv1.PodSecurityPolicySelfSubjectReviewSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "fake",
							Image: "fake",
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: new(int64), // root
							},
						},
					},
				},
			},
		},
	}
}
