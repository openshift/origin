package security

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	authenticationv1 "k8s.io/api/authentication/v1"
	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	securityv1 "github.com/openshift/api/security/v1"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"

	"github.com/openshift/origin/test/extended/authorization"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-auth][Feature:SecurityContextConstraints] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("scc", admissionapi.LevelPrivileged)
	ctx := context.Background()

	g.It("TestPodUpdateSCCEnforcement [apigroup:user.openshift.io][apigroup:authorization.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		projectName := oc.Namespace()
		haroldUser := oc.CreateUser("harold-").Name
		haroldClientConfig := oc.GetClientConfigForUser(haroldUser)
		haroldKubeClient := kubernetes.NewForConfigOrDie(haroldClientConfig)
		authorization.AddUserAdminToProject(oc, projectName, haroldUser)

		RunTestPodUpdateSCCEnforcement(ctx, haroldKubeClient, oc.AdminKubeClient(), projectName, t)
	})

	g.It("TestPodUpdateSCCEnforcement with service account", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		projectName := oc.Namespace()
		sa := createServiceAccount(ctx, oc, projectName)
		createPodAdminRoleOrDie(ctx, oc, sa)
		restrictedClient, _ := createClientFromServiceAccount(oc, sa)

		RunTestPodUpdateSCCEnforcement(ctx, restrictedClient, oc.AdminKubeClient(), projectName, t)
	})
})

func RunTestPodUpdateSCCEnforcement(ctx context.Context, restrictedClient, clusterAdminKubeClientset kubernetes.Interface, namespace string, t g.GinkgoTInterface) {
	// so cluster-admin can create privileged pods, but harold cannot.  This means that harold should not be able
	// to update the privileged pods either, even if he lies about its privileged nature
	privilegedPod := getPrivilegedPod("unsafe")

	if _, err := restrictedClient.CoreV1().Pods(namespace).Create(ctx, privilegedPod, metav1.CreateOptions{}); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	actualPod, err := clusterAdminKubeClientset.CoreV1().Pods(namespace).Create(ctx, privilegedPod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actualPod.Spec.Containers[0].Image = "something-nefarious"
	if _, err := restrictedClient.CoreV1().Pods(namespace).Update(ctx, actualPod, metav1.UpdateOptions{}); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden: %v", err)
	}

	// try to connect to /exec subresource as harold
	haroldCorev1Rest := restrictedClient.CoreV1().RESTClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := &metav1.Status{}
	err = haroldCorev1Rest.Post().
		Resource("pods").
		Namespace(namespace).
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
	if _, err := restrictedClient.CoreV1().Pods(namespace).Update(context.Background(), actualPod, metav1.UpdateOptions{}); err == nil {
		t.Fatalf("missing error: %v", err)
	}
}

var _ = g.Describe("[sig-auth][Feature:SecurityContextConstraints] ", func() {
	ctx := context.Background()

	defer g.GinkgoRecover()
	// pods running as root are being started here
	oc := exutil.NewCLIWithPodSecurityLevel("scc", admissionapi.LevelPrivileged)

	g.It("TestAllowedSCCViaRBAC [apigroup:project.openshift.io][apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:security.openshift.io]", g.Label("Size:L"), func() {
		t := g.GinkgoT()

		clusterAdminKubeClientset := oc.AdminKubeClient()

		project1 := oc.Namespace()
		project2 := oc.SetupProject()
		user1 := oc.CreateUser("user1-").Name
		user2 := oc.CreateUser("user2-").Name

		// set up 2 projects for 2 users
		authorization.AddUserAdminToProject(oc, project1, user1)
		user1Config := oc.GetClientConfigForUser(user1)
		user1Client := kubernetes.NewForConfigOrDie(user1Config)
		user1SecurityClient := securityv1client.NewForConfigOrDie(user1Config)

		authorization.AddUserAdminToProject(oc, project2, user2)
		user2Config := oc.GetClientConfigForUser(user2)
		user2Client := kubernetes.NewForConfigOrDie(user2Config)
		user2SecurityClient := securityv1client.NewForConfigOrDie(user2Config)

		RunTestAllowedSCCViaRBAC(
			ctx,
			oc,
			clusterAdminKubeClientset, user1Client, user2Client,
			user1SecurityClient, user2SecurityClient,
			project1, project2,
			rbacv1.Subject{Kind: rbacv1.UserKind, APIGroup: rbacv1helpers.GroupName, Name: user1},
			rbacv1.Subject{Kind: rbacv1.UserKind, APIGroup: rbacv1helpers.GroupName, Name: user2},
			t,
		)
	})

	g.It("TestAllowedSCCViaRBAC with service account [apigroup:security.openshift.io]", g.Label("Size:L"), func() {
		t := g.GinkgoT()

		clusterAdminKubeClientset := oc.AdminKubeClient()

		newNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-namespace-2", oc.Namespace()),
			},
		}
		_, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), newNamespace, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddExplicitResourceToDelete(corev1.SchemeGroupVersion.WithResource("namespaces"), "", newNamespace.Name)

		namespace1 := oc.Namespace()
		namespace2 := newNamespace.Name

		sa1 := createServiceAccount(ctx, oc, namespace1)
		createPodAdminRoleOrDie(ctx, oc, sa1)
		createPodSecurityPolicySelfSubjectReviewsRoleBindingOrDie(ctx, oc, sa1)

		sa2 := createServiceAccount(ctx, oc, namespace2)
		createPodAdminRoleOrDie(ctx, oc, sa2)
		createPodSecurityPolicySelfSubjectReviewsRoleBindingOrDie(ctx, oc, sa2)

		// set up 2 namespaces for 2 service accounts
		sa1Client, sa1SecurityClient := createClientFromServiceAccount(oc, sa1)
		sa2Client, sa2SecurityClient := createClientFromServiceAccount(oc, sa2)

		RunTestAllowedSCCViaRBAC(
			ctx,
			oc,
			clusterAdminKubeClientset, sa1Client, sa2Client,
			sa1SecurityClient, sa2SecurityClient,
			namespace1, namespace2,
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: sa1.Namespace, Name: sa1.Name},
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: sa2.Namespace, Name: sa2.Name},
			t,
		)
	})
})

func addSubjectsToClusterRoleBindingBuilder(builder *rbacv1helpers.ClusterRoleBindingBuilder, subjects ...rbacv1.Subject) *rbacv1helpers.ClusterRoleBindingBuilder {
	for _, subject := range subjects {
		builder.ClusterRoleBinding.Subjects = append(builder.ClusterRoleBinding.Subjects, subject)
	}
	return builder
}

func addSubjectsToRoleBindingBuilder(builder *rbacv1helpers.RoleBindingBuilder, subjects ...rbacv1.Subject) *rbacv1helpers.RoleBindingBuilder {
	for _, subject := range subjects {
		builder.RoleBinding.Subjects = append(builder.RoleBinding.Subjects, subject)
	}
	return builder
}

func RunTestAllowedSCCViaRBAC(
	ctx context.Context,
	oc *exutil.CLI,
	clusterAdminKubeClientset, clientset1, clientset2 kubernetes.Interface,
	securityClientset1, securityClientset2 *securityv1client.SecurityV1Client,
	namespace1, namespace2 string,
	subject1, subject2 rbacv1.Subject,
	t g.GinkgoTInterface,
) {
	clusterRole := "all-scc-" + namespace1
	rule := rbacv1helpers.NewRule("use").Groups("security.openshift.io").Resources("securitycontextconstraints").RuleOrDie()

	// set a up cluster role that allows access to all SCCs
	_, err := clusterAdminKubeClientset.RbacV1().ClusterRoles().Create(
		ctx,
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: clusterRole},
			Rules:      []rbacv1.PolicyRule{rule},
		},
		metav1.CreateOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	oc.AddExplicitResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("clusterroles"), "", clusterRole)

	createOpts := metav1.CreateOptions{}

	// subject1 cannot make a privileged pod
	if _, err := clientset1.CoreV1().Pods(namespace1).Create(ctx, getPrivilegedPod("test1"), createOpts); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden for serviceaccount1: %v", err)
	}

	// subject2 cannot make a privileged pod
	if _, err := clientset2.CoreV1().Pods(namespace2).Create(ctx, getPrivilegedPod("test2"), createOpts); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden for serviceaccount2: %v", err)
	}

	// this should allow subject1 to make a privileged pod in namespace1
	rb := addSubjectsToRoleBindingBuilder(NewRoleBindingForClusterRole(clusterRole, namespace1), subject1).BindingOrDie()
	if _, err := clusterAdminKubeClientset.RbacV1().RoleBindings(namespace1).Create(ctx, &rb, createOpts); err != nil {
		t.Fatal(err)
	}

	// this should allow subject1 to make pods in namespace2
	rbEditUser1Project2 := addSubjectsToRoleBindingBuilder(NewRoleBindingForClusterRole("edit", namespace2), subject1).BindingOrDie()
	if _, err := clusterAdminKubeClientset.RbacV1().RoleBindings(namespace2).Create(ctx, &rbEditUser1Project2, createOpts); err != nil {
		t.Fatal(err)
	}

	// this should allow subject2 to make pods in namespace1
	rbEditUser2Project1 := addSubjectsToRoleBindingBuilder(NewRoleBindingForClusterRole("edit", namespace1), subject2).BindingOrDie()
	if _, err := clusterAdminKubeClientset.RbacV1().RoleBindings(namespace1).Create(ctx, &rbEditUser2Project1, createOpts); err != nil {
		t.Fatal(err)
	}

	// this should allow subject2 to make a privileged pod in all namespaces
	crb := addSubjectsToClusterRoleBindingBuilder(rbacv1helpers.NewClusterBinding(clusterRole), subject2).BindingOrDie()
	if _, err := clusterAdminKubeClientset.RbacV1().ClusterRoleBindings().Create(ctx, &crb, createOpts); err != nil {
		t.Fatal(err)
	}
	oc.AddExplicitResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings"), "", crb.Name)

	// wait for RBAC to catch up to subject1 role binding for SCC
	if err := exutil.WaitForAccess(clientset1, true, &kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				Namespace: namespace1,
				Verb:      rule.Verbs[0],
				Group:     rule.APIGroups[0],
				Resource:  rule.Resources[0],
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// wait for RBAC to catch up to subject1 role binding for edit
	if err := exutil.WaitForAccess(clientset1, true, &kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				Namespace: namespace2,
				Verb:      "create",
				Group:     "",
				Resource:  "pods",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// wait for RBAC to catch up to subject2 role binding
	if err := exutil.WaitForAccess(clientset2, true, &kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				Namespace: namespace1,
				Verb:      "create",
				Group:     "",
				Resource:  "pods",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// wait for RBAC to catch up to subject2 cluster role binding
	if err := exutil.WaitForAccess(clientset2, true, &kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				Namespace: namespace2,
				Verb:      rule.Verbs[0],
				Group:     rule.APIGroups[0],
				Resource:  rule.Resources[0],
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// subject1 can make a privileged pod in namespace1
	if _, err := clientset1.CoreV1().Pods(namespace1).Create(ctx, getPrivilegedPod("test3"), createOpts); err != nil {
		t.Fatalf("subject1 failed to create pod in namespace1 via local binding: %v", err)
	}

	// subject1 cannot make a privileged pod in namespace2
	if _, err := clientset1.CoreV1().Pods(namespace2).Create(ctx, getPrivilegedPod("test4"), createOpts); !isForbiddenBySCC(err) {
		t.Fatalf("missing forbidden for serviceaccount1 in namespace2: %v", err)
	}

	// subject2 can make a privileged pod in namespace1
	if _, err := clientset2.CoreV1().Pods(namespace1).Create(ctx, getPrivilegedPod("test5"), createOpts); err != nil {
		t.Fatalf("subject2 failed to create pod in namespace1 via cluster binding: %v", err)
	}

	// subject2 can make a privileged pod in namespace2
	if _, err := clientset2.CoreV1().Pods(namespace2).Create(ctx, getPrivilegedPod("test6"), createOpts); err != nil {
		t.Fatalf("subject2 failed to create pod in namespace2 via cluster binding: %v", err)
	}

	// make sure PSP self subject review works since that is based by the same SCC logic but has different wiring

	// subject1 can make a privileged pod in namespace1
	subject1PSPReview, err := securityClientset1.PodSecurityPolicySelfSubjectReviews(namespace1).Create(ctx, runAsRootPSPSSR(), createOpts)
	if err != nil {
		t.Fatal(err)
	}
	if allowedBy := subject1PSPReview.Status.AllowedBy; allowedBy == nil || allowedBy.Name != "anyuid" {
		t.Fatalf("subject1 failed PSP SSR in namespace1: %v", allowedBy)
	}

	// subject2 can make a privileged pod in namespace2
	subject2PSPReview, err := securityClientset2.PodSecurityPolicySelfSubjectReviews(namespace2).Create(ctx, runAsRootPSPSSR(), createOpts)
	if err != nil {
		t.Fatal(err)
	}
	if allowedBy := subject2PSPReview.Status.AllowedBy; allowedBy == nil || allowedBy.Name != "anyuid" {
		t.Fatalf("subject2 failed PSP SSR in namespace2: %v", allowedBy)
	}
}

var _ = g.Describe("[sig-auth][Feature:SecurityContextConstraints] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("ssc", admissionapi.LevelBaseline)

	g.It("TestPodDefaultCapabilities", g.Label("Size:M"), func() {
		g.By("Running a restricted pod and getting it's inherited capabilities")
		// This test should use image.ShellImage but this requires having a local image
		// registry, which not all deployment types have. Using the lightest publicly available
		// image containing capsh.
		pod, err := exutil.NewPodExecutor(oc, "restrictedcapsh", image.LocationFor("quay.io/redhat-developer/test-build-simples2i:1.2"))
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO: remove desiredCapabilities once restricted-v2 is the default
		// system:authenticated SCC in the cluster - in favour of alternativeDesiredCapabilities
		desiredCapabilities := "000000000000051b"
		alternativeDesiredCapabilities := "0000000000000000"

		capabilities, err := pod.Exec("cat /proc/1/status | grep CapBnd | cut -f 2")
		o.Expect(err).NotTo(o.HaveOccurred())

		capString, err := pod.Exec("capsh --decode=" + capabilities)
		o.Expect(err).NotTo(o.HaveOccurred())

		desiredCapString, err := pod.Exec("capsh --decode=" + desiredCapabilities)
		o.Expect(err).NotTo(o.HaveOccurred())

		alternativeDesiredCapString, err := pod.Exec("capsh --decode=" + alternativeDesiredCapabilities)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.Logf("comparing capabilities: %s with desired: %s or more restricitve desired: %s", capabilities, desiredCapabilities, alternativeDesiredCapabilities)
		framework.Logf("which translates to: %s compared with desired: %s or more restrictive desired %s", capString, desiredCapString, alternativeDesiredCapString)
		o.Expect(capabilities).To(o.Or(o.Equal(desiredCapabilities), o.Equal(alternativeDesiredCapabilities)))
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

func createPodAdminRoleOrDie(ctx context.Context, oc *exutil.CLI, sa *corev1.ServiceAccount) {
	framework.Logf("Creating role")
	rule := rbacv1helpers.NewRule("create", "update").Groups("").Resources("pods", "pods/exec").RuleOrDie()
	_, err := oc.AdminKubeClient().RbacV1().Roles(sa.Namespace).Create(
		ctx,
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "podadmin"},
			Rules:      []rbacv1.PolicyRule{rule},
		},
		metav1.CreateOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Creating rolebinding")
	_, err = oc.AdminKubeClient().RbacV1().RoleBindings(sa.Namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    sa.Namespace,
			GenerateName: "podadmin-",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: sa.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "podadmin",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func createServiceAccount(ctx context.Context, oc *exutil.CLI, namespace string) *corev1.ServiceAccount {
	framework.Logf("Creating ServiceAccount")
	sa, err := oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-sa-"}}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Waiting for ServiceAccount %q to be provisioned...", sa.Name)
	err = exutil.WaitForServiceAccount(oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace), sa.Name)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sa
}

func createPodSecurityPolicySelfSubjectReviewsRoleBindingOrDie(ctx context.Context, oc *exutil.CLI, sa *corev1.ServiceAccount) {
	framework.Logf("Creating podsecuritypolicyselfsubjectreviews role")
	rule := rbacv1helpers.NewRule("create").Groups("security.openshift.io").Resources("podsecuritypolicyselfsubjectreviews").RuleOrDie()
	_, err := oc.AdminKubeClient().RbacV1().Roles(sa.Namespace).Create(
		ctx,
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "pspssr"},
			Rules:      []rbacv1.PolicyRule{rule},
		},
		metav1.CreateOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Creating podsecuritypolicyselfsubjectreviews rolebinding")
	_, err = oc.AdminKubeClient().RbacV1().RoleBindings(sa.Namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    sa.Namespace,
			GenerateName: "podadmin-",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: sa.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "pspssr",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func createClientFromServiceAccount(oc *exutil.CLI, sa *corev1.ServiceAccount) (*kubernetes.Clientset, *securityv1client.SecurityV1Client) {
	// create a new token request for the service account and use it to build a client for it
	framework.Logf("Creating service account token")
	bootstrapperToken, err := oc.AdminKubeClient().CoreV1().ServiceAccounts(sa.Namespace).CreateToken(context.TODO(), sa.Name, &authenticationv1.TokenRequest{}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	saClientConfig := restclient.AnonymousClientConfig(oc.AdminConfig())
	saClientConfig.BearerToken = bootstrapperToken.Status.Token

	return kubernetes.NewForConfigOrDie(saClientConfig), securityv1client.NewForConfigOrDie(saClientConfig)
}

func NewRoleBindingForClusterRole(roleName, namespace string) *rbacv1helpers.RoleBindingBuilder {
	const GroupName = "rbac.authorization.k8s.io"
	return &rbacv1helpers.RoleBindingBuilder{
		RoleBinding: rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: namespace,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: GroupName,
				Kind:     "ClusterRole",
				Name:     roleName,
			},
		},
	}
}
