package templates

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	exutil "github.com/openshift/origin/test/extended/util"
)

// 1. Check that users can't create or update templateinstances unless they are,
// or can impersonate, the requester.
// 2. Check that templateinstancespecs, particularly including
// requester.username and groups, are immutable.
var _ = g.Describe("[Conformance][templates] templateinstance impersonation tests", func() {
	defer g.GinkgoRecover()

	var (
		cli = exutil.NewCLI("templates", exutil.KubeConfigPath())

		adminuser              *userapi.User // project admin, but can't impersonate anyone
		impersonateuser        *userapi.User // project edit, and can impersonate edituser1
		impersonatebygroupuser *userapi.User
		impersonategroup       *userapi.Group
		edituser1              *userapi.User // project edit, can be impersonated by impersonateuser
		edituser2              *userapi.User // project edit
		viewuser               *userapi.User // project view

		dummytemplateinstance *templateapi.TemplateInstance

		dummycondition = templateapi.TemplateInstanceCondition{
			Type:   templateapi.TemplateInstanceConditionType("dummy"),
			Status: kapi.ConditionTrue,
		}

		tests []struct {
			user                      *userapi.User
			expectCreateSuccess       bool
			expectDeleteSuccess       bool
			hasUpdatePermission       bool
			hasUpdateStatusPermission bool
		}
	)

	g.BeforeEach(func() {
		adminuser = createUser(cli, "adminuser", bootstrappolicy.AdminRoleName)
		impersonateuser = createUser(cli, "impersonateuser", bootstrappolicy.EditRoleName)
		impersonatebygroupuser = createUser(cli, "impersonatebygroupuser", bootstrappolicy.EditRoleName)
		edituser1 = createUser(cli, "edituser1", bootstrappolicy.EditRoleName)
		edituser2 = createUser(cli, "edituser2", bootstrappolicy.EditRoleName)
		viewuser = createUser(cli, "viewuser", bootstrappolicy.ViewRoleName)

		impersonategroup = createGroup(cli, "impersonategroup", bootstrappolicy.EditRoleName)
		addUserToGroup(cli, impersonatebygroupuser.Name, impersonategroup.Name)

		// additional plumbing to enable impersonateuser to impersonate edituser1
		role, err := cli.AdminAuthorizationClient().Authorization().Roles(cli.Namespace()).Create(&authorizationapi.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "impersonater",
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:     sets.NewString("assign"),
					APIGroups: []string{templateapi.GroupName},
					Resources: sets.NewString("templateinstances"),
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.AdminAuthorizationClient().Authorization().RoleBindings(cli.Namespace()).Create(&authorizationapi.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "impersonater-binding",
			},
			RoleRef: kapi.ObjectReference{
				Name:      role.Name,
				Namespace: cli.Namespace(),
			},
			Subjects: []kapi.ObjectReference{
				{
					Kind: authorizationapi.UserKind,
					Name: impersonateuser.Name,
				},
				{
					Kind: authorizationapi.GroupKind,
					Name: impersonategroup.Name,
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// I think we get flakes when the group cache hasn't yet noticed the
		// new group membership made above.  Wait until all it looks like
		// all the users above have access to the namespace as expected.
		err = wait.PollImmediate(time.Second, 30*time.Second, func() (done bool, err error) {
			for _, user := range []*userapi.User{adminuser, impersonateuser, impersonatebygroupuser, edituser1, edituser2, viewuser} {
				cli.ChangeUser(user.Name)
				sar, err := cli.AuthorizationClient().Authorization().LocalSubjectAccessReviews(cli.Namespace()).Create(&authorizationapi.LocalSubjectAccessReview{
					Action: authorizationapi.Action{
						Verb:     "get",
						Resource: "pods",
					},
				})
				if err != nil {
					return false, err
				}
				if !sar.Allowed {
					return false, nil
				}
			}

			cli.ChangeUser(impersonatebygroupuser.Name)
			sar, err := cli.AuthorizationClient().Authorization().LocalSubjectAccessReviews(cli.Namespace()).Create(&authorizationapi.LocalSubjectAccessReview{
				Action: authorizationapi.Action{
					Verb:     "assign",
					Group:    templateapi.GroupName,
					Resource: "templateinstances",
				},
			})
			if err != nil {
				return false, err
			}
			if !sar.Allowed {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		dummytemplateinstance = &templateapi.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: templateapi.TemplateInstanceSpec{
				Template: templateapi.Template{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "template",
						Namespace: "dummy",
					},
				},
				// all the tests work with a templateinstance which is set up to
				// impersonate edituser1
				Requester: &templateapi.TemplateInstanceRequester{
					Username: edituser1.Name,
				},
			},
		}

		tests = []struct {
			user                      *userapi.User
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

	g.It("should pass impersonation creation tests", func() {
		// check who can create TemplateInstances (anyone with project write access
		// AND is/can impersonate spec.requester.username)
		for _, test := range tests {
			setUser(cli, test.user)

			templateinstancecopy := dummytemplateinstance.DeepCopy()
			templateinstance, err := cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Create(templateinstancecopy)

			if !test.expectCreateSuccess {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())

				err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})

	g.It("should pass impersonation update tests", func() {
		// check who can update TemplateInstances.  Via Update(), spec updates
		// should be rejected (with the exception of spec.metadata fields used
		// by the garbage collector, not tested here).  Status updates should be
		// silently ignored.  Via UpdateStatus(), spec updates should be
		// silently ignored.  Status should only be updatable by a user with
		// update access to that endpoint.  In practice this is intended only to
		// be the templateinstance controller and system:admin.
		for _, test := range tests {
			var templateinstancecopy *templateapi.TemplateInstance
			setUser(cli, test.user)

			templateinstance, err := cli.AdminTemplateClient().Template().TemplateInstances(cli.Namespace()).Create(dummytemplateinstance)
			o.Expect(err).NotTo(o.HaveOccurred())

			// ensure spec (particularly including spec.requester.username and groups) are
			// immutable via Update()
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Spec.Requester.Username = edituser2.Name

				_, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Update(templateinstancecopy)
				return err
			})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())

			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Spec.Requester.Groups = append(templateinstancecopy.Spec.Requester.Groups, "foo")

				_, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Update(templateinstancecopy)
				return err
			})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())

			// ensure status changes are ignored via Update()
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Status.Conditions = append(templateinstancecopy.Status.Conditions, dummycondition)

				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Update(templateinstancecopy)
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
				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Spec.Requester.Username = edituser2.Name

				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).UpdateStatus(templateinstancecopy)
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
				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				templateinstancecopy.Status.Conditions = []templateapi.TemplateInstanceCondition{dummycondition}

				templateinstancecopy, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).UpdateStatus(templateinstancecopy)
				return err
			})
			if !test.hasUpdateStatusPermission {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(templateinstancecopy.Status.Conditions).To(o.ContainElement(dummycondition))
			}

			err = cli.AdminTemplateClient().Template().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.It("should pass impersonation deletion tests", func() {
		// check who can delete TemplateInstances (anyone with project write access)
		for _, test := range tests {
			setUser(cli, test.user)

			templateinstancecopy := dummytemplateinstance.DeepCopy()
			templateinstance, err := cli.AdminTemplateClient().Template().TemplateInstances(cli.Namespace()).Create(templateinstancecopy)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
			if test.expectDeleteSuccess {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsForbidden(err)).To(o.BeTrue())

				err = cli.AdminTemplateClient().Template().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})
})
