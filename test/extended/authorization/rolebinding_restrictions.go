package authorization

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("rolebinding-restrictions")
	g.Context("", func() {
		g.Describe("Create a rolebinding when there are no restrictions", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				ns := oc.Namespace()
				user := "alice"
				roleBindingCreate(oc, false, false, ns, user, "rb1")
			})
		})

		g.Describe("Create a rolebinding when subject is permitted by RBR", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				ns := oc.Namespace()
				users := []string{"bob"}
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(context.Background(), generateAllowUserRolebindingRestriction(ns, users), metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				roleBindingCreate(oc, false, false, ns, users[0], "rb1")
			})
		})

		g.Describe("Create a rolebinding when subject is already bound", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				users := []string{"cindy"}
				ns := oc.Namespace()
				roleBindingCreate(oc, false, false, ns, users[0], "rb1")

				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(context.Background(), generateAllowUserRolebindingRestriction(ns, users), metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				roleBindingCreate(oc, false, false, ns, users[0], "rb2")
			})
		})

		g.Describe("Create a rolebinding when subject is not already bound and is not permitted by any RBR", func() {
			g.It(fmt.Sprintf("should fail [apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				ns := oc.Namespace()
				users := []string{"dave", "eve"}
				roleBindingCreate(oc, false, false, ns, users[0], "rb1")

				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(context.Background(), generateAllowUserRolebindingRestriction(ns, users[:len(users)-1]), metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				roleBindingCreate(oc, false, true, ns, users[1], "rb2")
			})
		})

		g.Describe("Create a RBAC rolebinding when subject is not already bound and is not permitted by any RBR", func() {
			g.It(fmt.Sprintf("should fail [apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				ns := oc.Namespace()
				users := []string{"frank", "george"}
				roleBindingCreate(oc, false, false, ns, users[0], "rb1")

				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(context.Background(), generateAllowUserRolebindingRestriction(ns, users[:len(users)-1]), metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				roleBindingCreate(oc, true, true, ns, users[1], "rb2")
			})
		})

		g.Describe("Create a rolebinding that also contains system:non-existing users", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				ns := oc.Namespace()
				users := []string{"harry", "system:non-existing"}
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(context.Background(), generateAllowUserRolebindingRestriction(ns, users), metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				roleBindingCreate(oc, false, false, ns, users[0], "rb1")

				roleBindingCreate(oc, false, false, ns, "system:non-existing", "rb2")
			})
		})

		g.Describe("Rolebinding restrictions tests single project", func() {
			g.It(fmt.Sprintf("should succeed [apigroup:authorization.openshift.io]"), g.Label("Size:S"), func() {
				ns := oc.Namespace()
				users := []string{"zed", "yvette"}
				users2 := []string{"xavier", "system:non-existing"}
				// No restrictions, rolebinding should succeed
				roleBindingCreate(oc, false, false, ns, users[0], "rb1")
				// Subject bound, rolebinding restriction should succeed
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(context.Background(), generateAllowUserRolebindingRestriction(ns, users[:len(users)-1]), metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Duplicate should succeed
				roleBindingCreate(oc, false, false, ns, users[0], "rb2")

				// Subject not bound, not permitted by any RBR, rolebinding should fail
				roleBindingCreate(oc, false, true, ns, users[1], "rb3")

				// Subject not bound, not permitted by any RBR, RBAC rolebinding should fail
				roleBindingCreate(oc, true, true, ns, users[1], "rb3")

				// Create a rolebinding that also contains system:non-existing users should succeed
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(context.Background(), generateAllowUserRolebindingRestriction(ns, users2), metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				roleBindingCreate(oc, false, false, ns, users2[0], "rb4")

				roleBindingCreate(oc, false, false, ns, users2[1], "rb5")
			})
		})
	})
})

func generateAllowUserRolebindingRestriction(ns string, users []string) *authorizationv1.RoleBindingRestriction {
	var userstr string
	for _, s := range users {
		userstr = fmt.Sprint(userstr + strings.Replace(s, ":", "", -1))
	}
	return &authorizationv1.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("match-users-%s", userstr),
			Namespace: ns,
		},
		Spec: authorizationv1.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationv1.UserRestriction{
				Users: users,
			},
		},
	}
}

func generateRolebinding(ns, user, rb string) *authorizationv1.RoleBinding {
	return &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      rb,
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind:      authorizationv1.UserKind,
				Namespace: ns,
				Name:      user,
			},
		},
		RoleRef: corev1.ObjectReference{Name: "role", Namespace: ns},
	}
}

func generateRbacUserRolebinding(ns, user, rb string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      rb,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.UserKind,
				Namespace: ns,
				Name:      user,
			},
		},
		RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "role"},
	}
}

func roleBindingCreate(oc *exutil.CLI, useRBAC, shouldErr bool, ns, user, rb string) {
	var rbrErr error
	switch {
	case useRBAC:
		_, rbrErr = oc.AdminKubeClient().RbacV1().RoleBindings(ns).Create(context.Background(), generateRbacUserRolebinding(ns, user, rb), metav1.CreateOptions{})
	default:
		_, rbrErr = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(context.Background(), generateRolebinding(ns, user, rb), metav1.CreateOptions{})
	}
	if !shouldErr {
		o.Expect(rbrErr).NotTo(o.HaveOccurred())
		return
	}
	o.Expect(rbrErr).NotTo(o.BeNil())
	o.Expect(kerrors.IsForbidden(rbrErr)).To(o.BeTrue())
	expectedErrorString := fmt.Sprintf("rolebindings to User \"%s\" are not allowed in project", user)
	o.Expect(rbrErr.Error()).Should(o.ContainSubstring(expectedErrorString))
}
