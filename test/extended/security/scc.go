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
		// Skip on Microshift clusters
		isHyperShift, err := exutil.IsHypershift(context.TODO(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("Skip case as control plane pods are not supported on HyperShift cluster")
		}

	})

	g.It("[CNTRLPLANE-1544] OCP-85221 Check the pods with uid, gid, hostUsers, annotations parameters are correctly set", func() {
		// Define namespaces that has annotations present, namespace: deploynames
		controlPlaneNamespacesWithDeployments := map[string][]string{
			"openshift-kube-controller-manager-operator": {"kube-controller-manager-operator"},
			//"openshift-cloud-credential-operator":        []string{"pod-identity-webhook", "cloud-credential-operator"},
			//"openshift-kube-apiserver-operator":          []string{"kube-apiserver-operator"},
			"openshift-kube-scheduler-operator": {"openshift-kube-scheduler-operator"},
		}

		for namespace, deployNames := range controlPlaneNamespacesWithDeployments {
			for _, deployName := range deployNames {
				annotationsNs := []string{"openshift-kube-scheduler-operator", "openshift-kube-controller-manager-operator"}
				// Check if namespace is NOT in the annotationsNs array
				skipAnnotationCheck := false
				for _, ns := range annotationsNs {
					if namespace == ns {
						skipAnnotationCheck = true
						break
					}
				}
				commonTestSteps(ctx, oc, namespace, deployName, 1000, 1000, 1000, skipAnnotationCheck)
			}
		}
	})

	g.It("[CNTRLPLANE-1544] OCP-85242  Test the deployment is up and running with parameters set uid,gid,restricted-v3 annotations in new namespace", func() {
		namespace := "openshift-kube-controller-manager-operator"
		// namespace = oc.Namespace()
		deployName := "deployment-uid-gid"
		skipAnnotationCheck := false

		g.By("Creating the deployment")
		deployment := createDeploymentFromYAML(namespace, deployName, 1010, 1020, 1000)
		_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddExplicitResourceToDelete(appsv1.SchemeGroupVersion.WithResource("deployments"), namespace, deployName)

		commonTestSteps(ctx, oc, namespace, deployName, 1010, 1020, 1000, skipAnnotationCheck)
	})

	g.It("[CNTRLPLANE-1544] OCP-85303 Test the deployment with invalid security context values are not allowed", func() {
		// namespace := oc.Namespace()
		namespace := "openshift-kube-controller-manager-operator"

		g.By("Testing deployment with runAsUser: 65536 (should fail)")
		deployName1 := "deployment-invalid-user-65536"
		deployment := createDeploymentFromYAML(namespace, deployName1, 65536, 1000, 1000)
		_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddExplicitResourceToDelete(appsv1.SchemeGroupVersion.WithResource("deployments"), namespace, deployName1)

		message := "container create failed: setresuid to `65536`: Invalid argument"
		commonTestStepsInvalidGroup(oc, deployment, message)

		g.By("Testing deployment with runAsGroup: 65536 (should fail)")
		deployName2 := "deployment-invalid-group-65536"
		deployment = createDeploymentFromYAML(namespace, deployName2, 1000, 65536, 1000)
		_, err = oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddExplicitResourceToDelete(appsv1.SchemeGroupVersion.WithResource("deployments"), namespace, deployName2)

		message = "container create failed: setgroups: Invalid argument"
		commonTestStepsInvalidGroup(oc, deployment, message)
	})
})

func createDeploymentFromYAML(namespace, deployName string, runAsUser, runAsGroup, fsGroup int64) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: namespace,
			Annotations: map[string]string{
				"openshift.io/required-scc": "restricted-v3",
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
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:    ptr.To(runAsUser),
						RunAsGroup:   ptr.To(runAsGroup),
						FSGroup:      ptr.To(fsGroup),
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					HostUsers: ptr.To(false),
					Containers: []corev1.Container{
						{
							Name:    "test-container",
							Image:   "ubuntu",
							Command: []string{"/bin/bash", "-c", "sleep 3600"},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

func commonTestSteps(ctx context.Context, oc *exutil.CLI, namespace, deployName string, runAsUser, runAsGroup, fsGroup int64, skipAnnotationCheck bool) {
	g.By("Checking the deployment is ready")
	err := exutil.WaitForDeploymentReady(oc, deployName, namespace, -1)
	o.Expect(err).NotTo(o.HaveOccurred())

	deploy, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(ctx, deployName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	// If skipAnnotationCheck is true, skip the check for the deployment annotations
	if !skipAnnotationCheck {
		g.By("Checking the deployment annotations are set to restricted-v3")
		o.Expect(deploy.ObjectMeta.Annotations["openshift.io/required-scc"]).To(o.Equal("restricted-v3"))
	}

	g.By(fmt.Sprintf("Checking the deployment has runAsUser set to %d", runAsUser))
	o.Expect(deploy.Spec.Template.Spec.SecurityContext.RunAsUser).To(o.Equal(ptr.To(int64(runAsUser))))

	g.By(fmt.Sprintf("Checking the deployment has runAsGroup set to %d", runAsGroup))
	o.Expect(deploy.Spec.Template.Spec.SecurityContext.RunAsGroup).To(o.Equal(ptr.To(int64(runAsGroup))))

	g.By(fmt.Sprintf("Checking the deployment has fsGroup set to %d", fsGroup))
	o.Expect(deploy.Spec.Template.Spec.SecurityContext.FSGroup).To(o.Equal(ptr.To(int64(fsGroup))))

	g.By("Checking the deployment has hostUsers set to false")
	o.Expect(deploy.Spec.Template.Spec.HostUsers).To(o.Equal(ptr.To(false)))

	g.By("Checking the pods have the correct uid, gid, groups")
	// Get label selector from deployment
	labelSelector := ""
	for key, value := range deploy.Spec.Selector.MatchLabels {
		labelSelector = fmt.Sprintf("%s=%s", key, value)
		break
	}
	pods, err := exutil.GetDeploymentPods(oc, deployName, namespace, labelSelector)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pods.Items).NotTo(o.BeEmpty())
	podName := pods.Items[0].Name
	out, err := oc.AsAdmin().Run("exec").Args(podName, "--namespace="+namespace, "--", "/bin/bash", "-c", "id").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("uid=%d(%d) gid=%d(%d) groups=%d(%d)", runAsUser, runAsUser, runAsGroup, runAsGroup, runAsGroup, runAsGroup)))
}

func commonTestStepsInvalidGroup(oc *exutil.CLI, deployment *appsv1.Deployment, message string) {
	var containerStatus corev1.ContainerStatus

	g.By(fmt.Sprintf("Checking the deployment should not be ready: %v", deployment.Name))
	// Get label selector from deployment
	labelSelector := ""
	for key, value := range deployment.Spec.Selector.MatchLabels {
		labelSelector = fmt.Sprintf("%s=%s", key, value)
		break
	}

	g.By("Waiting for container to reach CreateContainerError state")
	o.Eventually(func() string {
		pods, err := exutil.GetDeploymentPods(oc, deployment.Name, deployment.Namespace, labelSelector)
		if err != nil || len(pods.Items) == 0 {
			return ""
		}
		pod := &pods.Items[0]
		if len(pod.Status.ContainerStatuses) == 0 {
			return ""
		}
		containerStatus = pod.Status.ContainerStatuses[0]
		if containerStatus.State.Waiting != nil {
			return containerStatus.State.Waiting.Reason
		}
		return ""
	}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(o.Equal("CreateContainerError"))
	o.Expect(containerStatus.State.Waiting.Message).To(o.ContainSubstring(message))
}
