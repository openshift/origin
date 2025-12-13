package user

import (
	"context"
	"fmt"
	"reflect"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	userv1 "github.com/openshift/api/user/v1"
	projectv1typedclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:UserAPI]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("user-api")

	g.It("users can manipulate groups [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:project.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		clusterAdminUserClient := oc.AdminUserClient().UserV1()

		valerieName := oc.CreateUser("valerie-").Name

		g.By("make sure we don't get back system groups", func() {
			// make sure we don't get back system groups
			userValerie, err := clusterAdminUserClient.Users().Get(context.Background(), valerieName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(userValerie.Groups) != 0 {
				t.Errorf("unexpected groups: %v", userValerie.Groups)
			}
		})

		g.By("make sure that user/~ returns groups for unbacked users", func() {
			// Compatible with some setups use system:cluster-admins instead of system:masters
			allowedGroups := [][]string{
				{"system:authenticated", "system:masters"},
				{"system:authenticated", "system:cluster-admins"},
			}

			clusterAdminUser, err := clusterAdminUserClient.Users().Get(context.Background(), "~", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			matched := false
			for _, expectedGroups := range allowedGroups {
				if reflect.DeepEqual(clusterAdminUser.Groups, expectedGroups) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("unexpected groups returned for user/~: got %v, expected one of %v", clusterAdminUser.Groups, allowedGroups)
			}
		})

		theGroup := &userv1.Group{}
		theGroup.Name = "theGroup-" + oc.Namespace()
		theGroup.Users = append(theGroup.Users, valerieName)
		_, err := clusterAdminUserClient.Groups().Create(context.Background(), theGroup, metav1.CreateOptions{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		oc.AddResourceToDelete(userv1.GroupVersion.WithResource("groups"), theGroup)

		g.By("make sure that user/~ returns system groups for backed users when it merges", func() {
			// make sure that user/~ returns system groups for backed users when it merges
			expectedValerieGroups := []string{"system:authenticated", "system:authenticated:oauth", theGroup.Name}
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			var lastErr error
			if err := wait.PollImmediate(100*time.Millisecond, wait.ForeverTestTimeout, func() (done bool, err error) {
				secondValerie, err := userv1typedclient.NewForConfigOrDie(valerieConfig).Users().Get(context.Background(), "~", metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(secondValerie.Groups, expectedValerieGroups) {
					lastErr = fmt.Errorf("expected %v, got %v", expectedValerieGroups, secondValerie.Groups)
					return false, nil
				}
				return true, nil
			}); err != nil {
				if lastErr != nil {
					t.Error(lastErr)
				} else {
					t.Error(err)
				}
			}
		})

		g.By("confirm no access to the project", func() {
			// separate client here to avoid bad caching
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(context.Background(), oc.Namespace(), metav1.GetOptions{})
			if err == nil {
				t.Fatalf("expected error")
			}
		})

		g.By("adding the binding", func() {
			roleBinding := &authorizationv1.RoleBinding{}
			roleBinding.Name = "admins"
			roleBinding.RoleRef.Name = "admin"
			roleBinding.Subjects = []corev1.ObjectReference{
				{Kind: "Group", Name: theGroup.Name},
			}
			_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(oc.Namespace()).Create(context.Background(), roleBinding, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			err = oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
				Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
						Namespace: oc.Namespace(),
						Verb:      "get",
						Group:     "",
						Resource:  "pods",
					},
				},
			}, valerieName)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("make sure that user groups are respected for policy", func() {
			// make sure that user groups are respected for policy
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(context.Background(), oc.Namespace(), metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	g.It("groups should work [apigroup:user.openshift.io][apigroup:project.openshift.io][apigroup:authorization.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()
		clusterAdminUserClient := oc.AdminUserClient().UserV1()

		victorName := oc.CreateUser("victor-").Name
		valerieName := oc.CreateUser("valerie-").Name
		valerieConfig := oc.GetClientConfigForUser(valerieName)

		g.By("creating the group")
		theGroup := &userv1.Group{}
		theGroup.Name = "thegroup-" + oc.Namespace()
		theGroup.Users = append(theGroup.Users, valerieName, victorName)
		_, err := clusterAdminUserClient.Groups().Create(context.Background(), theGroup, metav1.CreateOptions{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		oc.AddResourceToDelete(userv1.GroupVersion.WithResource("groups"), theGroup)

		g.By("confirm no access to the project", func() {
			// separate client here to avoid bad caching
			valerieConfig := oc.GetClientConfigForUser(valerieName)
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(context.Background(), oc.Namespace(), metav1.GetOptions{})
			if err == nil {
				t.Fatalf("expected error")
			}
		})

		g.By("adding the binding", func() {
			roleBinding := &authorizationv1.RoleBinding{}
			roleBinding.Name = "admins"
			roleBinding.RoleRef.Name = "admin"
			roleBinding.Subjects = []corev1.ObjectReference{
				{Kind: "Group", Name: theGroup.Name},
			}
			_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(oc.Namespace()).Create(context.Background(), roleBinding, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			err = oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
				Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
						Namespace: oc.Namespace(),
						Verb:      "list",
						Group:     "",
						Resource:  "pods",
					},
				},
			}, valerieName)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("checking access", func() {
			// make sure that user groups are respected for policy
			_, err = projectv1typedclient.NewForConfigOrDie(valerieConfig).Projects().Get(context.Background(), oc.Namespace(), metav1.GetOptions{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			victorConfig := oc.GetClientConfigForUser(victorName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, err = projectv1typedclient.NewForConfigOrDie(victorConfig).Projects().Get(context.Background(), oc.Namespace(), metav1.GetOptions{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	})
})
