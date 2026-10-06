package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
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
	"k8s.io/utils/ptr"

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

	g.It("TestPodUpdateSCCEnforcement [apigroup:user.openshift.io][apigroup:authorization.openshift.io]", func() {
		t := g.GinkgoT()

		projectName := oc.Namespace()
		haroldUser := oc.CreateUser("harold-").Name
		haroldClientConfig := oc.GetClientConfigForUser(haroldUser)
		haroldKubeClient := kubernetes.NewForConfigOrDie(haroldClientConfig)
		authorization.AddUserAdminToProject(oc, projectName, haroldUser)

		RunTestPodUpdateSCCEnforcement(ctx, haroldKubeClient, oc.AdminKubeClient(), projectName, t)
	})

	g.It("TestPodUpdateSCCEnforcement with service account", func() {
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

	g.It("TestAllowedSCCViaRBAC [apigroup:project.openshift.io][apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:security.openshift.io]", func() {
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

	g.It("TestAllowedSCCViaRBAC with service account [apigroup:security.openshift.io]", func() {
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

	g.It("TestPodDefaultCapabilities", func() {
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

var _ = g.Describe("[sig-auth][Feature:SecurityContextConstraints] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("scc-userns", admissionapi.LevelPrivileged)
	ctx := context.Background()

	g.BeforeEach(func() {
		// Skip on Hypershift clusters
		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("Skip case as control plane pods are not supported on HyperShift cluster")
		}

		// Skip on Microshift clusters
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skip case as control plane pods are not supported on MicroShift cluster")
		}
	})

	g.It("[CNTRLPLANE-1544] OCP-85221 Verify control plane deployments have valid user namespace security context", func() {
		config := getControlPlaneConfig()

		for namespace, deployNames := range config.deployments {
			for _, deployName := range deployNames {
				// Fetch the deployment object from the cluster
				deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(ctx, deployName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Check if namespace should skip annotation check
				skipAnnotationCheck := config.skipAnnotations[namespace]

				// Check if namespace has runAsUser/runAsGroup configured
				hasSecurityContext := config.hasSecurityContext[namespace]

				if hasSecurityContext {
					// Deployments with runAsUser/runAsGroup present - verify specific values
					commonTestStepsValidGroups(ctx, oc, deployment, defaultUID, defaultGID, defaultFSGroup, skipAnnotationCheck)
				} else {
					// Deployments without runAsUser/runAsGroup - skip these checks
					framework.Logf("Skipping runAsUser/runAsGroup checks for %s/%s (not yet configured)", namespace, deployName)
					commonTestStepsValidGroups(ctx, oc, deployment, unsetIDSentinel, unsetIDSentinel, unsetIDSentinel, skipAnnotationCheck)
				}
			}
		}
	})

	g.It("[CNTRLPLANE-1544] OCP-85242  Test deployment with hostUsers: false and restricted-v3 annotations is up and running in user namespace", func() {
		namespace := oc.Namespace()
		deployName := "deployment-hostusers-false"
		skipAnnotationCheck := false

		g.By("Creating the deployment with hostUsers: false and restricted-v3 annotations")
		deployment := createDeploymentWithContainerSecurityContext(
			namespace, deployName,
			testUID, testGID, defaultFSGroup,
			unsetIDSentinel, unsetIDSentinel,
			false,
		)
		_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddExplicitResourceToDelete(appsv1.SchemeGroupVersion.WithResource("deployments"), namespace, deployName)

		commonTestStepsValidGroups(ctx, oc, deployment, testUID, testGID, defaultFSGroup, skipAnnotationCheck)
	})

	g.It("[CNTRLPLANE-1544] OCP-85927 Test deployment with hostUsers: true and restricted-v3 annotation fails with expected error in user namespace", func() {
		namespace := oc.Namespace()
		deployName := "deployment-hostusers-true"

		g.By("Creating deployment with hostUsers: true and restricted-v3 annotation (should fail)")
		// Create deployment with hostUsers: true - this is invalid for restricted-v3 SCC
		deployment := createDeploymentWithContainerSecurityContext(
			namespace, deployName,
			defaultUID, defaultGID, defaultFSGroup,
			unsetIDSentinel, unsetIDSentinel,
			true,
		)
		_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddExplicitResourceToDelete(appsv1.SchemeGroupVersion.WithResource("deployments"), namespace, deployName)

		expectedError := "provider restricted-v3: .spec.securityContext.hostUsers: Invalid value: true: Host Users must be set to false"
		commonTestStepsInvalidGroup(oc, deployment, expectedError)
	})

	g.It("[CNTRLPLANE-1544] OCP-85303 Test the deployment with invalid security context values are not allowed in user namespaces", func() {
		namespace := oc.Namespace()

		// Table-driven test for invalid security context values
		// SCC admission controller should reject these values at ReplicaSet level
		testCases := getInvalidSecurityContextTestCases()

		for _, tc := range testCases {
			g.By(fmt.Sprintf("Testing deployment with %s (should fail: %s)", tc.deployName, tc.description))
			deployment := createDeploymentWithContainerSecurityContext(
				namespace, tc.deployName,
				tc.runAsUser, tc.runAsGroup, tc.fsGroup,
				unsetIDSentinel, unsetIDSentinel,
				false,
			)
			_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.AddExplicitResourceToDelete(appsv1.SchemeGroupVersion.WithResource("deployments"), namespace, tc.deployName)

			commonTestStepsInvalidGroup(oc, deployment, tc.expectedError)
		}
	})

	g.It("[CNTRLPLANE-1544] OCP-85928 Test container-level security context overrides pod-level values correctly", func() {
		namespace := oc.Namespace()
		deployName := "deployment-container-override"

		g.By("Testing deployment with container-level runAsUser and runAsGroup overriding pod-level")
		// Create deployment with pod-level: uid=defaultUID, gid=defaultGID
		// Container-level: uid=containerUID, gid=containerGID (should override)
		deployment := createDeploymentWithContainerSecurityContext(
			namespace, deployName,
			defaultUID, defaultGID, defaultFSGroup,
			containerUID, containerGID,
			false,
		)
		_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddExplicitResourceToDelete(appsv1.SchemeGroupVersion.WithResource("deployments"), namespace, deployName)

		// Use commonTestStepsValidGroups to verify container runs with container-level values
		// Skip deployment validation since we're testing container-level override of pod-level values
		// Pass container-level values that the pod should actually run as
		commonTestStepsValidGroups(ctx, oc, deployment, containerUID, containerGID, defaultFSGroup, true)
	})
})

const (
	// Security context constraint constants
	sccRestrictedV3 = "restricted-v3"
	sccAnnotation   = "openshift.io/required-scc"

	// User namespace ID ranges
	defaultUID      = 1000
	defaultGID      = 1000
	defaultFSGroup  = 1000
	testUID         = 1010
	testGID         = 1020
	containerUID    = 5000
	containerGID    = 6000
	invalidIDAbove  = 65535
	invalidIDBelow  = 999
	minValidID      = 1000
	maxValidID      = 65534
	unsetIDSentinel = -1

	// Timeout and polling intervals
	deploymentTimeout = 2 * time.Minute
	pollingInterval   = 5 * time.Second
	debugTimeout      = 60 * time.Second
	debugPolling      = 10 * time.Second

	// Debug output line indices
	debugOutputPIDLine = 2
	debugOutputMapLine = 2
)

// controlPlaneConfig defines configuration for control plane namespace testing
type controlPlaneConfig struct {
	deployments        map[string][]string
	skipAnnotations    map[string]bool
	hasSecurityContext map[string]bool
}

// getControlPlaneConfig returns the configuration for control plane namespace testing
func getControlPlaneConfig() controlPlaneConfig {
	return controlPlaneConfig{
		deployments: map[string][]string{
			// Pods with runAsUser/runAsGroup already configured
			"openshift-kube-controller-manager-operator": {"kube-controller-manager-operator"},
			"openshift-kube-apiserver-operator":          {"kube-apiserver-operator"},
			"openshift-kube-scheduler-operator":          {"openshift-kube-scheduler-operator"},

			// Pods without runAsUser/runAsGroup (will be added in future)
			"openshift-cloud-credential-operator":       {"pod-identity-webhook", "cloud-credential-operator"},
			"openshift-cloud-network-config-controller": {"cloud-network-config-controller"},
			"openshift-controller-manager-operator":     {"openshift-controller-manager-operator"},
			"openshift-controller-manager":              {"controller-manager"},
			"openshift-route-controller-manager":        {"route-controller-manager"},
			"openshift-service-ca-operator":             {"service-ca-operator"},
			"openshift-service-ca":                      {"service-ca"},
		},
		skipAnnotations: map[string]bool{
			// Skip annotation check (legacy deployments or deployments without explicit annotation)
			"openshift-kube-controller-manager-operator": true,
			"openshift-kube-apiserver-operator":          true,
			"openshift-kube-scheduler-operator":          true,
			"openshift-service-ca":                       true,

			// Require annotation check (newer deployments with explicit restricted-v3 annotation)
			"openshift-cloud-credential-operator":       false,
			"openshift-cloud-network-config-controller": false,
			"openshift-controller-manager-operator":     false,
			"openshift-controller-manager":              false,
			"openshift-route-controller-manager":        false,
			"openshift-service-ca-operator":             false,
		},
		hasSecurityContext: map[string]bool{
			"openshift-kube-controller-manager-operator": true, // Has runAsUser/runAsGroup
			"openshift-kube-apiserver-operator":          true, // Has runAsUser/runAsGroup
			"openshift-kube-scheduler-operator":          true, // Has runAsUser/runAsGroup
			// All others: false (don't have runAsUser/runAsGroup yet)
		},
	}
}

// invalidSecurityContextTestCase defines a test case for invalid security context validation
type invalidSecurityContextTestCase struct {
	deployName    string
	runAsUser     int64
	runAsGroup    int64
	fsGroup       int64
	expectedError string
	description   string
}

// getInvalidSecurityContextTestCases returns test cases for invalid security context values
func getInvalidSecurityContextTestCases() []invalidSecurityContextTestCase {
	return []invalidSecurityContextTestCase{
		{
			deployName:    "deployment-invalid-user-65535",
			runAsUser:     invalidIDAbove,
			runAsGroup:    defaultGID,
			fsGroup:       defaultFSGroup,
			expectedError: fmt.Sprintf("Invalid value: %d: must be in the ranges: [%d, %d]'", invalidIDAbove, minValidID, maxValidID),
			description:   "runAsUser value 65535 is out of allowed range",
		},
		{
			deployName:    "deployment-invalid-group-65535",
			runAsUser:     defaultUID,
			runAsGroup:    invalidIDAbove,
			fsGroup:       defaultFSGroup,
			expectedError: "unable to validate against any security context constraint",
			description:   "runAsGroup value 65535 is out of allowed range",
		},
		{
			deployName:    "deployment-invalid-user-999",
			runAsUser:     invalidIDBelow,
			runAsGroup:    defaultGID,
			fsGroup:       defaultFSGroup,
			expectedError: fmt.Sprintf("Invalid value: %d: must be in the ranges: [%d, %d]'", invalidIDBelow, minValidID, maxValidID),
			description:   "runAsUser value 999 is below minimum allowed",
		},
		{
			deployName:    "deployment-invalid-group-999",
			runAsUser:     invalidIDBelow,
			runAsGroup:    defaultGID,
			fsGroup:       defaultFSGroup,
			expectedError: fmt.Sprintf("Invalid value: %d: must be in the ranges: [%d, %d]'", invalidIDBelow, minValidID, maxValidID),
			description:   "runAsGroup value 999 is below minimum allowed",
		},
	}
}

// createDeploymentWithContainerSecurityContext creates a deployment with optional security context values.
// Parameters:
//   - namespace: the namespace where the deployment will be created
//   - deployName: the name of the deployment
//   - podRunAsUser: pod-level runAsUser value, use unsetIDSentinel (-1) if not needed
//   - podRunAsGroup: pod-level runAsGroup value, use unsetIDSentinel (-1) if not needed
//   - podFSGroup: pod-level fsGroup value, use unsetIDSentinel (-1) if not needed
//   - containerRunAsUser: container-level runAsUser value (overrides pod-level), use unsetIDSentinel (-1) if not needed
//   - containerRunAsGroup: container-level runAsGroup value (overrides pod-level), use unsetIDSentinel (-1) if not needed
//   - hostUsers: whether user namespaces are enabled (false) or disabled (true)
func createDeploymentWithContainerSecurityContext(
	namespace, deployName string,
	podRunAsUser, podRunAsGroup, podFSGroup int64,
	containerRunAsUser, containerRunAsGroup int64,
	hostUsers bool,
) *appsv1.Deployment {
	// Convert int64 parameters to *int64 (use unsetIDSentinel as indicator for nil)
	var podRunAsUserPtr, podRunAsGroupPtr, podFSGroupPtr *int64
	var containerRunAsUserPtr, containerRunAsGroupPtr *int64

	if podRunAsUser >= 0 {
		podRunAsUserPtr = ptr.To(podRunAsUser)
	}
	if podRunAsGroup >= 0 {
		podRunAsGroupPtr = ptr.To(podRunAsGroup)
	}
	if podFSGroup >= 0 {
		podFSGroupPtr = ptr.To(podFSGroup)
	}
	if containerRunAsUser >= 0 {
		containerRunAsUserPtr = ptr.To(containerRunAsUser)
	}
	if containerRunAsGroup >= 0 {
		containerRunAsGroupPtr = ptr.To(containerRunAsGroup)
	}

	// Build container security context with restricted defaults
	containerSecurityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	// Override with container-level security settings if provided
	if containerRunAsUserPtr != nil {
		containerSecurityContext.RunAsUser = containerRunAsUserPtr
	}
	if containerRunAsGroupPtr != nil {
		containerSecurityContext.RunAsGroup = containerRunAsGroupPtr
	}

	// Build pod security context with required baseline fields
	podSecurityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	// Add optional pod-level security settings
	if podRunAsUserPtr != nil {
		podSecurityContext.RunAsUser = podRunAsUserPtr
	}
	if podRunAsGroupPtr != nil {
		podSecurityContext.RunAsGroup = podRunAsGroupPtr
	}
	if podFSGroupPtr != nil {
		podSecurityContext.FSGroup = podFSGroupPtr
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: namespace,
			Annotations: map[string]string{
				sccAnnotation: sccRestrictedV3,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": deployName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": deployName},
				},
				Spec: corev1.PodSpec{
					SecurityContext: podSecurityContext,
					HostUsers:       ptr.To(hostUsers),
					Containers: []corev1.Container{
						{
							Name:            "test-container",
							Image:           image.ShellImage(),
							Command:         []string{"/bin/bash", "-c", "id && sleep 3600"},
							SecurityContext: containerSecurityContext,
						},
					},
				},
			},
		},
	}
}

// extractPIDFromDebugOutput extracts the container PID from debug command output.
// Debug output format: lines 0-1 contain debug pod metadata, line 2 contains the actual PID value.
func extractPIDFromDebugOutput(output string) (string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= debugOutputPIDLine {
		return "", fmt.Errorf("insufficient lines in debug output: got %d lines, expected at least %d", len(lines), debugOutputPIDLine+1)
	}
	pid := strings.TrimSpace(lines[debugOutputPIDLine])
	if pid == "" || pid == "null" {
		return "", fmt.Errorf("invalid PID value: %q", pid)
	}
	return pid, nil
}

// parseIDMapOutput parses uid_map or gid_map output and extracts the outside ID.
// Map output format: lines 0-1 contain debug pod metadata, line 2 contains the mapping data.
func parseIDMapOutput(output, mapType string) (string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= debugOutputMapLine {
		return "", fmt.Errorf("insufficient lines in %s output: got %d lines, expected at least %d", mapType, len(lines), debugOutputMapLine+1)
	}
	fields := strings.Fields(lines[debugOutputMapLine])
	if len(fields) < 2 {
		return "", fmt.Errorf("insufficient fields in %s: got %d fields, expected at least 2", mapType, len(fields))
	}
	// Return outside ID (second field in the mapping: inside_id outside_id length)
	return fields[1], nil
}

// verifyIDMapping verifies ID mapping (uid_map or gid_map) for user namespace.
// The outside ID should be different from the configured ID due to user namespace remapping.
// Parameters:
//   - oc: the OpenShift CLI client
//   - nodeName: the name of the node where the container is running
//   - pid: the process ID of the container
//   - mapType: the type of mapping file to check ("uid_map" or "gid_map")
//   - idName: descriptive name for logging purposes ("UID" or "GID")
//   - expectedID: the ID that was configured (runAsUser or runAsGroup), nil if not configured
func verifyIDMapping(oc *exutil.CLI, nodeName, pid, mapType, idName string, expectedID *int64) {
	g.By(fmt.Sprintf("Checking %s for user namespace mapping", mapType))

	// Read the ID mapping file from the container's proc filesystem
	mapCmd := fmt.Sprintf("chroot /host cat /proc/%s/%s", pid, mapType)
	mapOut, err := oc.AsAdmin().Run("debug").Args("node/"+nodeName, "--", "bash", "-c", mapCmd).Output()
	if err != nil {
		framework.Logf("Warning: Could not read %s: %v", mapType, err)
		framework.Logf("Skipping %s verification (non-critical for this test)", mapType)
		return
	}

	framework.Logf("%s content: %s", mapType, mapOut)

	outsideID, err := parseIDMapOutput(mapOut, mapType)
	if err != nil {
		framework.Logf("Warning: %v", err)
		framework.Logf("Skipping %s verification (non-critical for this test)", mapType)
		return
	}

	// Verify user namespace remapping is working
	// The outside ID should be different from the configured ID due to user namespace mapping
	if expectedID != nil {
		o.Expect(outsideID).NotTo(o.Equal(fmt.Sprintf("%d", *expectedID)),
			fmt.Sprintf("User namespace should remap %s %d to a different outside ID", idName, *expectedID))
		framework.Logf("✓ User namespace mapping verified: outside %s from %s is %s (different from configured %d)",
			idName, mapType, outsideID, *expectedID)
	} else {
		framework.Logf("%s found with outside %s: %s (not configured, skipping comparison)", mapType, idName, outsideID)
	}
}

// verifyUserNamespaceMapping verifies that user namespace mapping is working correctly
// by checking both uid_map and gid_map for the container.
// Parameters:
//   - oc: the OpenShift CLI client
//   - nodeName: the name of the node where the container is running
//   - containerID: the ID of the container to inspect
//   - runAsUser: the configured runAsUser value, nil if not configured
//   - runAsGroup: the configured runAsGroup value, nil if not configured
func verifyUserNamespaceMapping(oc *exutil.CLI, nodeName, containerID string, runAsUser, runAsGroup *int64) {
	g.By(fmt.Sprintf("Creating debug pod for node %s to inspect container and check ID mappings", nodeName))

	// Get the container PID using crictl inspect
	debugCmd := fmt.Sprintf("chroot /host crictl inspect %s | jq -r '.info.pid'", containerID)
	var pidOut string
	o.Eventually(func() string {
		out, _ := oc.AsAdmin().Run("debug").Args("node/"+nodeName, "--", "bash", "-c", debugCmd).Output()
		pidOut = out
		return out
	}).WithTimeout(debugTimeout).WithPolling(debugPolling).Should(o.Not(o.BeEmpty()),
		"Failed to retrieve container PID")

	pid, err := extractPIDFromDebugOutput(pidOut)
	if err != nil {
		framework.Logf("Warning: %v", err)
		framework.Logf("Skipping ID mapping verification (non-critical for this test)")
		return
	}

	framework.Logf("Extracted container PID: %s", pid)

	// Verify uid_map and gid_map for user namespace
	verifyIDMapping(oc, nodeName, pid, "uid_map", "UID", runAsUser)
	verifyIDMapping(oc, nodeName, pid, "gid_map", "GID", runAsGroup)
}

// verifyPodUIDGID verifies that the pod is running with the expected UID and GID.
// Executes the 'id' command inside the pod and validates the output.
// Parameters:
//   - oc: the OpenShift CLI client
//   - namespace: the namespace where the pod is running
//   - podName: the name of the pod to verify
//   - runAsUser: the expected UID value
//   - runAsGroup: the expected GID value
func verifyPodUIDGID(oc *exutil.CLI, namespace, podName string, runAsUser, runAsGroup *int64) {
	g.By("Checking the pods have the correct uid, gid, groups")

	// Execute 'id' command inside the pod to get UID/GID information
	out, err := oc.AsAdmin().Run("exec").Args(podName, "--namespace="+namespace, "--", "/bin/bash", "-c", "id").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify the output contains the expected UID and GID values
	expectedOutput := fmt.Sprintf("uid=%d(%d) gid=%d(%d) groups=%d(%d)",
		*runAsUser, *runAsUser, *runAsGroup, *runAsGroup, *runAsGroup, *runAsGroup)
	o.Expect(out).To(o.ContainSubstring(expectedOutput),
		fmt.Sprintf("Pod should be running with UID %d and GID %d", *runAsUser, *runAsGroup))
}

// validateSecurityContextField validates a specific security context field (runAsUser or runAsGroup).
// It checks container-level securityContext first (which overrides pod-level), then falls back to pod-level.
// Parameters:
//   - deploy: the deployment object to validate
//   - namespace: the namespace of the deployment (for logging)
//   - deployName: the name of the deployment (for logging)
//   - expectedValue: expected value for the field
//   - fieldName: name of the field being validated ("runAsUser" or "runAsGroup")
//   - getContainerValue: function to get the value from container securityContext
//   - getPodValue: function to get the value from pod securityContext
func validateSecurityContextField(deploy *appsv1.Deployment, namespace, deployName string, expectedValue *int64, fieldName string, getContainerValue func(*corev1.SecurityContext) *int64, getPodValue func(*corev1.PodSecurityContext) *int64) {
	if expectedValue == nil {
		framework.Logf("Skipping %s check for %s/%s (not configured)", fieldName, namespace, deployName)
		return
	}

	g.By(fmt.Sprintf("Checking the deployment has %s set to %d", fieldName, *expectedValue))

	// Check if containers have their own securityContext with the field
	// Container-level securityContext overrides pod-level securityContext
	if len(deploy.Spec.Template.Spec.Containers) > 0 &&
		deploy.Spec.Template.Spec.Containers[0].SecurityContext != nil {
		containerValue := getContainerValue(deploy.Spec.Template.Spec.Containers[0].SecurityContext)
		if containerValue != nil {
			o.Expect(containerValue).To(o.Equal(expectedValue),
				fmt.Sprintf("Deployment %s/%s container should have %s %d", namespace, deployName, fieldName, *expectedValue))
			return
		}
	}

	// No container-level field, check pod-level
	podValue := getPodValue(deploy.Spec.Template.Spec.SecurityContext)
	o.Expect(podValue).To(o.Equal(expectedValue),
		fmt.Sprintf("Deployment %s/%s should have %s %d", namespace, deployName, fieldName, *expectedValue))
}

// validateDeploymentSecurityContext validates the deployment's security context settings.
// This includes checking SCC annotations and verifying UID/GID/fsGroup configurations.
// Parameters:
//   - deploy: the deployment object to validate
//   - namespace: the namespace of the deployment (for logging)
//   - deployName: the name of the deployment (for logging)
//   - runAsUser: expected runAsUser value, nil if not required
//   - runAsGroup: expected runAsGroup value, nil if not required
//   - fsGroup: expected fsGroup value, nil if not required
//   - skipAnnotationCheck: if true, skip checking the SCC annotation
func validateDeploymentSecurityContext(deploy *appsv1.Deployment, namespace, deployName string, runAsUser, runAsGroup, fsGroup *int64, skipAnnotationCheck bool) {
	// Verify SCC annotation if required
	if !skipAnnotationCheck {
		g.By("Checking the deployment annotations are set to restricted-v3")
		o.Expect(deploy.ObjectMeta.Annotations[sccAnnotation]).To(o.Equal(sccRestrictedV3),
			fmt.Sprintf("Deployment %s/%s should have %s annotation set to %s", namespace, deployName, sccAnnotation, sccRestrictedV3))
	}

	// Validate security context fields if they are configured
	// Some control plane deployments may not have these set yet
	validateSecurityContextField(deploy, namespace, deployName, runAsUser, "runAsUser",
		func(sc *corev1.SecurityContext) *int64 { return sc.RunAsUser },
		func(psc *corev1.PodSecurityContext) *int64 { return psc.RunAsUser })

	validateSecurityContextField(deploy, namespace, deployName, runAsGroup, "runAsGroup",
		func(sc *corev1.SecurityContext) *int64 { return sc.RunAsGroup },
		func(psc *corev1.PodSecurityContext) *int64 { return psc.RunAsGroup })

	if fsGroup != nil {
		g.By(fmt.Sprintf("Checking the deployment has fsGroup set to %d", *fsGroup))
		o.Expect(deploy.Spec.Template.Spec.SecurityContext.FSGroup).To(o.Equal(fsGroup),
			fmt.Sprintf("Deployment %s/%s should have fsGroup %d", namespace, deployName, *fsGroup))
	} else {
		framework.Logf("Skipping fsGroup check for %s/%s (not configured)", namespace, deployName)
	}

	// hostUsers must be set to false for user namespaces to be enabled
	g.By("Checking the deployment has hostUsers set to false")
	o.Expect(deploy.Spec.Template.Spec.HostUsers).To(o.Equal(ptr.To(false)),
		fmt.Sprintf("Deployment %s/%s should have hostUsers set to false for user namespace support", namespace, deployName))
}

// commonTestStepsValidGroups performs common validation steps for deployments with valid security contexts.
// This includes verifying deployment readiness, security context settings, and user namespace mappings.
// Parameters:
//   - ctx: context for the operation
//   - oc: the OpenShift CLI client
//   - deployment: the deployment object to validate
//   - runAsUser: expected runAsUser value, use unsetIDSentinel (-1) if not configured
//   - runAsGroup: expected runAsGroup value, use unsetIDSentinel (-1) if not configured
//   - fsGroup: expected fsGroup value, use unsetIDSentinel (-1) if not configured
//   - skipAnnotationCheck: if true, skip checking the SCC annotation
func commonTestStepsValidGroups(ctx context.Context, oc *exutil.CLI, deployment *appsv1.Deployment, runAsUser, runAsGroup, fsGroup int64, skipAnnotationCheck bool) {
	// Convert int64 parameters to *int64 (use unsetIDSentinel as indicator for nil)
	var runAsUserPtr, runAsGroupPtr, fsGroupPtr *int64
	if runAsUser >= 0 {
		runAsUserPtr = ptr.To(runAsUser)
	}
	if runAsGroup >= 0 {
		runAsGroupPtr = ptr.To(runAsGroup)
	}
	if fsGroup >= 0 {
		fsGroupPtr = ptr.To(fsGroup)
	}

	namespace := deployment.Namespace
	deployName := deployment.Name

	// Wait for deployment to become ready
	g.By("Checking the deployment is ready")
	err := exutil.WaitForDeploymentReady(oc, deployName, namespace, -1)
	o.Expect(err).NotTo(o.HaveOccurred(), "Deployment should become ready")

	// Fetch latest deployment state and validate its security context
	deploy, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(ctx, deployName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	validateDeploymentSecurityContext(deploy, namespace, deployName, runAsUserPtr, runAsGroupPtr, fsGroupPtr, skipAnnotationCheck)

	// Build label selector from deployment's match labels
	labelPairs := make([]string, 0, len(deploy.Spec.Selector.MatchLabels))
	for key, value := range deploy.Spec.Selector.MatchLabels {
		labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", key, value))
	}
	labelSelector := strings.Join(labelPairs, ",")

	// Retrieve pods created by this deployment
	pods, err := exutil.GetDeploymentPods(oc, deployName, namespace, labelSelector)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pods.Items).NotTo(o.BeEmpty(), "Deployment should have created at least one pod")

	pod := &pods.Items[0]
	podName := pod.Name
	nodeName := pod.Spec.NodeName
	o.Expect(nodeName).NotTo(o.BeEmpty(), "Pod should be scheduled on a node")
	o.Expect(podName).NotTo(o.BeEmpty(), "Pod name should not be empty")

	// Verify pod is running with the correct UID/GID if configured
	if runAsUserPtr != nil && runAsGroupPtr != nil {
		verifyPodUIDGID(oc, namespace, podName, runAsUserPtr, runAsGroupPtr)
	} else {
		framework.Logf("Skipping pod UID/GID verification for %s/%s (security context not configured)", namespace, deployName)
	}

	// Verify user namespace ID mapping if container is available
	g.By("Getting container ID and node information for ID mapping verification")
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].ContainerID != "" {
		containerID := strings.TrimPrefix(pod.Status.ContainerStatuses[0].ContainerID, "cri-o://")
		verifyUserNamespaceMapping(oc, nodeName, containerID, runAsUserPtr, runAsGroupPtr)
	} else {
		framework.Logf("Container ID not available (expected for CreateContainerError state)")
	}
}

// commonTestStepsInvalidGroup validates that deployments with invalid security contexts fail as expected.
// When a deployment has invalid security context values, the SCC admission controller
// rejects pod creation at the ReplicaSet level, so no pods are created.
// The error appears in the Deployment's status conditions under ReplicaFailure.
// Parameters:
//   - oc: the OpenShift CLI client
//   - deployment: the deployment object expected to fail
//   - expectedErrorMessage: the expected error message substring to verify
func commonTestStepsInvalidGroup(oc *exutil.CLI, deployment *appsv1.Deployment, expectedErrorMessage string) {
	ctx := context.Background()

	g.By(fmt.Sprintf("Verifying deployment %s should fail with expected error", deployment.Name))

	// Wait for deployment to reach ReplicaFailure status
	g.By("Waiting for deployment to reach ReplicaFailure status with FailedCreate reason")
	var replicaFailureMessage string
	o.Eventually(func() bool {
		deploy, err := oc.AdminKubeClient().AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error getting deployment: %v", err)
			return false
		}

		// Look for ReplicaFailure condition with FailedCreate reason
		for _, condition := range deploy.Status.Conditions {
			if condition.Type == appsv1.DeploymentReplicaFailure &&
				condition.Status == corev1.ConditionTrue &&
				condition.Reason == "FailedCreate" {
				replicaFailureMessage = condition.Message
				framework.Logf("Found ReplicaFailure condition with message: %s", replicaFailureMessage)
				return true
			}
		}
		return false
	}).WithTimeout(deploymentTimeout).WithPolling(pollingInterval).Should(o.BeTrue(),
		"Expected deployment to have ReplicaFailure condition with FailedCreate reason")

	// Verify the failure message contains the expected error
	g.By(fmt.Sprintf("Verifying the failure message contains expected text: %s", expectedErrorMessage))
	o.Expect(replicaFailureMessage).To(o.ContainSubstring(expectedErrorMessage),
		fmt.Sprintf("Expected failure message to contain %q, but got: %s", expectedErrorMessage, replicaFailureMessage))
}
