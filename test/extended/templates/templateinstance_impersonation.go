package templates

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	templatev1 "github.com/openshift/api/template/v1"
	userv1 "github.com/openshift/api/user/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// 1. Check that users can't create or update templateinstances unless they are,
// or can impersonate, the requester.
// 2. Check that templateinstancespecs, particularly including
// requester.username and groups, are immutable.
var _ = g.Describe("[sig-devex][Feature:Templates] templateinstance impersonation tests [apigroup:user.openshift.io][apigroup:authorization.openshift.io]", func() {
	defer g.GinkgoRecover()

	var (
		cli = exutil.NewCLI("templates")

		adminuser              *userv1.User // project admin, but can't impersonate anyone
		impersonateuser        *userv1.User // project edit, and can impersonate edituser1
		impersonatebygroupuser *userv1.User
		impersonategroup       *userv1.Group
		edituser1              *userv1.User // project edit, can be impersonated by impersonateuser
		edituser2              *userv1.User // project edit
		viewuser               *userv1.User // project view

		dummytemplateinstance *templatev1.TemplateInstance

		dummycondition = templatev1.TemplateInstanceCondition{
			Type:   templatev1.TemplateInstanceConditionType("dummy"),
			Status: corev1.ConditionTrue,
		}

		tests []struct {
			user                      *userv1.User
			expectCreateSuccess       bool
			expectDeleteSuccess       bool
			hasUpdatePermission       bool
			hasUpdateStatusPermission bool
		}
	)

	g.BeforeEach(func() {
		adminuser = createUser(cli, "adminuser", "admin")
		impersonateuser = createUser(cli, "impersonateuser", "edit")
		impersonatebygroupuser = createUser(cli, "impersonatebygroupuser", "edit")
		edituser1 = createUser(cli, "edituser1", "edit")
		edituser2 = createUser(cli, "edituser2", "edit")
		viewuser = createUser(cli, "viewuser", "view")

		impersonategroup = createGroup(cli, "impersonategroup", "edit")
		addUserToGroup(cli, impersonatebygroupuser.Name, impersonategroup.Name)

		// additional plumbing to enable impersonateuser to impersonate edituser1
		role, err := cli.AdminAuthorizationClient().AuthorizationV1().Roles(cli.Namespace()).Create(context.Background(), &authorizationv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "impersonater",
			},
			Rules: []authorizationv1.PolicyRule{
				{
					Verbs:     []string{"assign"},
					APIGroups: []string{"template.openshift.io"},
					Resources: []string{"templateinstances"},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.AdminAuthorizationClient().AuthorizationV1().RoleBindings(cli.Namespace()).Create(context.Background(), &authorizationv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "impersonater-binding",
			},
			RoleRef: corev1.ObjectReference{
				Name:      role.Name,
				Namespace: cli.Namespace(),
			},
			Subjects: []corev1.ObjectReference{
				{
					Kind: authorizationv1.UserKind,
					Name: impersonateuser.Name,
				},
				{
					Kind: authorizationv1.GroupKind,
					Name: impersonategroup.Name,
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// I think we get flakes when the group cache hasn't yet noticed the
		// new group membership made above.  Wait until all it looks like
		// all the users above have access to the namespace as expected.
		err = wait.PollImmediate(time.Second, 30*time.Second, func() (done bool, err error) {
			for _, user := range []*userv1.User{adminuser, impersonateuser, impersonatebygroupuser, edituser1, edituser2, viewuser} {
				cli.ChangeUser(user.Name)
				sar, err := cli.AuthorizationClient().AuthorizationV1().LocalSubjectAccessReviews(cli.Namespace()).Create(context.Background(), &authorizationv1.LocalSubjectAccessReview{
					Action: authorizationv1.Action{
						Verb:     "get",
						Resource: "pods",
					},
				}, metav1.CreateOptions{})
				if err != nil {
					return false, err
				}
				if !sar.Allowed {
					return false, nil
				}
			}

			cli.ChangeUser(impersonatebygroupuser.Name)
			sar, err := cli.AuthorizationClient().AuthorizationV1().LocalSubjectAccessReviews(cli.Namespace()).Create(context.Background(), &authorizationv1.LocalSubjectAccessReview{
				Action: authorizationv1.Action{
					Verb:     "assign",
					Group:    templatev1.GroupName,
					Resource: "templateinstances",
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return false, err
			}
			if !sar.Allowed {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		dummytemplateinstance = &templatev1.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: templatev1.TemplateInstanceSpec{
				Template: templatev1.Template{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "template",
						Namespace: "dummy",
					},
				},
				// all the tests work with a templateinstance which is set up to
				// impersonate edituser1
				Requester: &templatev1.TemplateInstanceRequester{
					Username: edituser1.Name,
				},
			},
		}

		tests = []struct {
			user                      *userv1.User
			expectCreateSuccess       bool
			expectDeleteSuccess       bool
			hasUpdatePermission       bool
			hasUpdateStatusPermission bool
		}{
			{
				user:                      nil,  // system-admin
				expectCreateSuccess:       true, // can impersonate anyone
				expectDeleteSuccess:       true,
				hasUpdatePermission:       true,
				hasUpdateStatusPermission: true,
			},
			{
				user:                      adminuser,
				expectCreateSuccess:       false, // cannot impersonate edituser1
				expectDeleteSuccess:       true,
				hasUpdatePermission:       true,
				hasUpdateStatusPermission: false,
			},
			{
				user:                      impersonateuser,
				expectCreateSuccess:       true, // can impersonate edituser1
				expectDeleteSuccess:       true,
				hasUpdatePermission:       true,
				hasUpdateStatusPermission: false,
			},
			{
				user:                      impersonatebygroupuser,
				expectCreateSuccess:       true, // can impersonate edituser1
				expectDeleteSuccess:       true,
				hasUpdatePermission:       true,
				hasUpdateStatusPermission: false,
			},
			{
				user:                      edituser1,
				expectCreateSuccess:       true, // is edituser1
				expectDeleteSuccess:       true,
				hasUpdatePermission:       true,
				hasUpdateStatusPermission: false,
			},
			{
				user:                      edituser2,
				expectCreateSuccess:       false, // cannot impersonate edituser1
				expectDeleteSuccess:       true,
				hasUpdatePermission:       true,
				hasUpdateStatusPermission: false,
			},
			{
				user:                      viewuser,
				expectCreateSuccess:       false, // cannot create things and cannot impersonate edituser1
				expectDeleteSuccess:       false,
				hasUpdatePermission:       false,
				hasUpdateStatusPermission: false,
			},
		}
	})

	g.AfterEach(func() {
		deleteUser(cli, adminuser)
		deleteUser(cli, impersonateuser)
		deleteUser(cli, edituser1)
		deleteUser(cli, edituser2)
		deleteUser(cli, viewuser)
		deleteUser(cli, impersonatebygroupuser)
		deleteGroup(cli, impersonategroup)
	})

	g.It("should pass impersonation creation tests [apigroup:template.openshift.io]", g.Label("Size:L"), func() {
		// check who can create TemplateInstances (anyone with project write access
		// AND is/can impersonate spec.requester.username)
		for _, test := range tests {
			setUser(cli, test.user)

			templateinstancecopy := dummytemplateinstance.DeepCopy()
			templateinstance, err := cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(context.Background(), templateinstancecopy, metav1.CreateOptions{})

			if !test.expectCreateSuccess {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())

				err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Delete(context.Background(), templateinstance.Name, metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})

	g.It("should pass impersonation update tests [apigroup:template.openshift.io]", g.Label("Size:L"), func() {
		// check who can update TemplateInstances.  Via Update(), spec updates
		// should be rejected (with the exception of spec.metadata fields used
		// by the garbage collector, not tested here).  Status updates should be
		// silently ignored.  Via UpdateStatus(), spec updates should be
		// silently ignored.  Status should only be updatable by a user with
		// update access to that endpoint.  In practice this is intended only to
		// be the templateinstance controller and system:admin.
		for _, test := range tests {
			var templateinstancecopy *templatev1.TemplateInstance
			setUser(cli, test.user)

			templateinstance, err := cli.AdminTemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(context.Background(), dummytemplateinstance, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// ensure spec (particularly including spec.requester.username and groups) are
			// immutable via Update()
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Spec.Requester.Username = edituser2.Name

				_, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Update(context.Background(), templateinstancecopy, metav1.UpdateOptions{})
				return err
			})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())

			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Spec.Requester.Groups = append(templateinstancecopy.Spec.Requester.Groups, "foo")

				_, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Update(context.Background(), templateinstancecopy, metav1.UpdateOptions{})
				return err
			})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())

			// ensure status changes are ignored via Update()
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Status.Conditions = append(templateinstancecopy.Status.Conditions, dummycondition)

				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Update(context.Background(), templateinstancecopy, metav1.UpdateOptions{})
				return err
			})
			if !test.hasUpdatePermission {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(templateinstancecopy.Status.Conditions).NotTo(o.ContainElement(dummycondition))
			}

			// ensure spec changes are ignored via UpdateStatus()
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Spec.Requester.Username = edituser2.Name

				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).UpdateStatus(context.Background(), templateinstancecopy, metav1.UpdateOptions{})
				return err
			})
			if !test.hasUpdateStatusPermission {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(templateinstancecopy.Spec).To(o.Equal(dummytemplateinstance.Spec))
			}

			// ensure status changes are allowed via UpdateStatus()
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Status.Conditions = []templatev1.TemplateInstanceCondition{dummycondition}

				templateinstancecopy, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).UpdateStatus(context.Background(), templateinstancecopy, metav1.UpdateOptions{})
				return err
			})
			if !test.hasUpdateStatusPermission {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(templateinstancecopy.Status.Conditions).To(o.ContainElement(dummycondition))
			}

			err = cli.AdminTemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Delete(context.Background(), templateinstance.Name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.It("should pass impersonation deletion tests [apigroup:template.openshift.io]", g.Label("Size:M"), func() {
		// check who can delete TemplateInstances (anyone with project write access)
		for _, test := range tests {
			setUser(cli, test.user)

			templateinstancecopy := dummytemplateinstance.DeepCopy()
			templateinstance, err := cli.AdminTemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(context.Background(), templateinstancecopy, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Delete(context.Background(), templateinstance.Name, metav1.DeleteOptions{})
			if test.expectDeleteSuccess {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsForbidden(err)).To(o.BeTrue())

				err = cli.AdminTemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Delete(context.Background(), templateinstance.Name, metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})
})
