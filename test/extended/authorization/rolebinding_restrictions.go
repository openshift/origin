package authorization

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature: RoleBinding Restrictions] RoleBindingRestrictions should be functional", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("rolebinding-restrictions", exutil.KubeConfigPath())
	g.Context("", func() {
		g.Describe("Create a rolebinding when there are no restrictions", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				ns := oc.Namespace()
				user := "alice"
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebinding(ns, user, "rb1"))
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})

		g.Describe("Create a rolebinding when subject is permitted by RBR", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				ns := oc.Namespace()
				user := "bob"
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(generateAllowUserRolebindingRestriction(ns, user))
				o.Expect(err).NotTo(o.HaveOccurred())

				testRoleBindingCreate(oc, false, false, ns, user, "rb1")
			})
		})

		g.Describe("Create a rolebinding when subject is already bound", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				user := "cindy"
				ns := oc.Namespace()
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebinding(ns, user, "rb1"))
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(generateAllowUserRolebindingRestriction(ns, user))
				o.Expect(err).NotTo(o.HaveOccurred())

				testRoleBindingCreate(oc, false, false, ns, user, "rb2")
			})
		})

		g.Describe("Create a rolebinding when subject is not already bound and is not permitted by any RBR", func() {
			g.It(fmt.Sprintf("should fail"), func() {
				ns := oc.Namespace()
				user1 := "dave"
				user2 := "eve"
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebinding(ns, user1, "rb1"))
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(generateAllowUserRolebindingRestriction(ns, user1))
				o.Expect(err).NotTo(o.HaveOccurred())

				testRoleBindingCreate(oc, false, true, ns, user2, "rb2")
			})
		})

		g.Describe("Create a RBAC rolebinding when subject is not already bound and is not permitted by any RBR", func() {
			g.It(fmt.Sprintf("should fail"), func() {
				ns := oc.Namespace()
				user1 := "frank"
				user2 := "george"
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebinding(ns, user1, "rb1"))
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(generateAllowUserRolebindingRestriction(ns, user1))
				o.Expect(err).NotTo(o.HaveOccurred())

				testRoleBindingCreate(oc, true, true, ns, user2, "rb2")
			})
		})

		g.Describe("Create a rolebinding that also contains system:non-existing users", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				ns := oc.Namespace()
				user := "harry"
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(generateRBRnonExist(ns, user))
				o.Expect(err).NotTo(o.HaveOccurred())

				testRoleBindingCreate(oc, false, false, ns, user, "rb1")

				// we know the RBR cache is in sync now so this should never flake
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebindingNonExisting(ns, "rb2"))
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})

		g.Describe("Rolebinding restrictions tests single project", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				ns := oc.Namespace()
				user1 := "zed"
				user2 := "yvette"
				user3 := "xavier"
				// No restrictions, rolebinding should succeed
				_, err := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebinding(ns, user1, "rb1"))
				o.Expect(err).NotTo(o.HaveOccurred())
				// Subject bound, rolebinding restriction should succeed
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(generateAllowUserRolebindingRestriction(ns, user1))
				o.Expect(err).NotTo(o.HaveOccurred())

				// Duplicate should succeed
				testRoleBindingCreate(oc, false, false, ns, user1, "rb2")

				// Subject not bound, not permitted by any RBR, rolebinding should fail
				testRoleBindingCreate(oc, false, true, ns, user2, "rb3")

				// Subject not bound, not permitted by any RBR, RBAC rolebinding should fail
				testRoleBindingCreate(oc, true, true, ns, user2, "rb3")

				// Create a rolebinding that also contains system:non-existing users should succeed
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindingRestrictions(ns).Create(generateRBRnonExist(ns, user3))
				o.Expect(err).NotTo(o.HaveOccurred())

				testRoleBindingCreate(oc, false, false, ns, user3, "rb4")

				// we know the RBR cache is in sync now so this should never flake
				_, err = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebindingNonExisting(ns, "rb5"))
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})

func generateAllowUserRolebindingRestriction(ns, user string) *authorizationv1.RoleBindingRestriction {
	return &authorizationv1.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("match-users-%s", user),
			Namespace: ns,
		},
		Spec: authorizationv1.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationv1.UserRestriction{
				Users: []string{user},
			},
		},
	}
}

func generateRBRnonExist(ns, user string) *authorizationv1.RoleBindingRestriction {
	return &authorizationv1.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("match-users-%s-and-non-existing", user),
			Namespace: ns,
		},
		Spec: authorizationv1.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationv1.UserRestriction{
				Users: []string{user, "system:non-existing"},
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
				Kind:      authorizationapi.UserKind,
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

func generateRolebindingNonExisting(ns, rb string) *authorizationv1.RoleBinding {
	return &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      rb,
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind:      authorizationapi.UserKind,
				Namespace: ns,
				Name:      "system:non-existing",
			},
		},
		RoleRef: corev1.ObjectReference{Name: "role", Namespace: ns},
	}
}

func testRoleBindingCreate(oc *exutil.CLI, useRBAC, shouldErr bool, ns, user, rb string) {
	var (
		counter int
		rbrErr  error
	)

	timeoutErr := wait.PollImmediate(time.Second, wait.ForeverTestTimeout, func() (done bool, _ error) {
		cleanupErr := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Delete(rb, nil)
		if cleanupErr != nil && !kerrors.IsNotFound(cleanupErr) {
			o.Expect(cleanupErr).NotTo(o.HaveOccurred())
		}

		counter++

		if useRBAC {
			_, rbrErr = oc.AdminKubeClient().RbacV1().RoleBindings(ns).Create(generateRbacUserRolebinding(ns, user, rb))
		} else {
			_, rbrErr = oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(ns).Create(generateRolebinding(ns, user, rb))
		}

		if !shouldErr {
			return rbrErr == nil && counter > 5, nil
		}

		if rbrErr != nil {
			o.Expect(kerrors.IsForbidden(rbrErr)).To(o.BeTrue())
			expectedErrorString := fmt.Sprintf("rolebindings to User \"%s\" are not allowed in project", user)
			o.Expect(rbrErr.Error()).Should(o.ContainSubstring(expectedErrorString))
			return true, nil
		}

		return false, nil
	})

	if !shouldErr {
		o.Expect(rbrErr).NotTo(o.HaveOccurred())
	}
	o.Expect(timeoutErr).NotTo(o.HaveOccurred())
}
