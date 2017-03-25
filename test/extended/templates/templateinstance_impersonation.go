package templates

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"
)

// 1. Check that users can't create or update templateinstances unless they are,
// or can impersonate, the requester.
// 2. Check that templateinstancespecs, particularly including
// requester.username, are immutable.
var _ = g.Describe("[templates] templateinstance impersonation tests", func() {
	defer g.GinkgoRecover()

	var (
		cli = exutil.NewCLI("templates", exutil.KubeConfigPath())

		adminuser       *userapi.User // project admin, but can't impersonate anyone
		impersonateuser *userapi.User // project edit, and can impersonate edituser1
		edituser1       *userapi.User // project edit, can be impersonated by impersonateuser
		edituser2       *userapi.User // project edit
		viewuser        *userapi.User // project view

		clusterrole        *authorizationapi.ClusterRole
		clusterrolebinding *authorizationapi.ClusterRoleBinding

		dummytemplateinstance *templateapi.TemplateInstance

		tests []struct {
			user                      *userapi.User
			expectCreateUpdateSuccess bool
			expectDeleteSuccess       bool
		}
	)

	g.BeforeEach(func() {
		var err error

		adminuser = createUser(cli, "adminuser", bootstrappolicy.AdminRoleName)
		impersonateuser = createUser(cli, "impersonateuser", bootstrappolicy.EditRoleName)
		edituser1 = createUser(cli, "edituser1", bootstrappolicy.EditRoleName)
		edituser2 = createUser(cli, "edituser2", bootstrappolicy.EditRoleName)
		viewuser = createUser(cli, "viewuser", bootstrappolicy.ViewRoleName)

		dummytemplateinstance = &templateapi.TemplateInstance{
			ObjectMeta: kapi.ObjectMeta{
				Name: "test",
			},
			Spec: templateapi.TemplateInstanceSpec{
				Template: templateapi.Template{
					ObjectMeta: kapi.ObjectMeta{
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
			// post the status to avoid kicking off the controller
			Status: templateapi.TemplateInstanceStatus{
				Conditions: []templateapi.TemplateInstanceCondition{
					{
						Type:   templateapi.TemplateInstanceReady,
						Status: kapi.ConditionTrue,
					},
				},
			},
		}

		tests = []struct {
			user                      *userapi.User
			expectCreateUpdateSuccess bool
			expectDeleteSuccess       bool
		}{
			{
				user: nil, // system-admin
				expectCreateUpdateSuccess: true, // can impersonate anyone
				expectDeleteSuccess:       true,
			},
			{
				user: adminuser,
				expectCreateUpdateSuccess: false, // cannot impersonate edituser1
				expectDeleteSuccess:       true,
			},
			{
				user: impersonateuser,
				expectCreateUpdateSuccess: true, // can impersonate edituser1
				expectDeleteSuccess:       true,
			},
			{
				user: edituser1,
				expectCreateUpdateSuccess: true, // is edituser1
				expectDeleteSuccess:       true,
			},
			{
				user: edituser2,
				expectCreateUpdateSuccess: false, // cannot impersonate edituser1
				expectDeleteSuccess:       true,
			},
			{
				user: viewuser,
				expectCreateUpdateSuccess: false, // cannot create things and cannot impersonate edituser1
				expectDeleteSuccess:       false,
			},
		}

		// additional plumbing to enable impersonateuser to impersonate edituser1
		clusterrole, err = cli.AdminClient().ClusterRoles().Create(&authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{
				Name: cli.Namespace() + "-impersonate",
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:         sets.NewString("impersonate"),
					APIGroups:     []string{userapi.GroupName},
					Resources:     sets.NewString(authorizationapi.UserResource),
					ResourceNames: sets.NewString(edituser1.Name),
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		clusterrolebinding, err = cli.AdminClient().ClusterRoleBindings().Create(&authorizationapi.ClusterRoleBinding{
			ObjectMeta: kapi.ObjectMeta{
				Name: cli.Namespace() + "-impersonate",
			},
			RoleRef: kapi.ObjectReference{
				Name: clusterrole.Name,
			},
			Subjects: []kapi.ObjectReference{
				{
					Kind: authorizationapi.UserKind,
					Name: impersonateuser.Name,
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func() {
		deleteUser(cli, adminuser)
		deleteUser(cli, impersonateuser)
		deleteUser(cli, edituser1)
		deleteUser(cli, edituser2)
		deleteUser(cli, viewuser)

		err := cli.AdminClient().ClusterRoles().Delete(clusterrole.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = cli.AdminClient().ClusterRoleBindings().Delete(clusterrolebinding.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should pass impersonation creation tests", func() {
		// check who can create TemplateInstances (anyone with project write access
		// AND is/can impersonate spec.requester.username)
		for _, test := range tests {
			setUser(cli, test.user)

			templateinstancecopy, err := kapi.Scheme.DeepCopy(dummytemplateinstance)
			o.Expect(err).NotTo(o.HaveOccurred())
			templateinstance, err := cli.TemplateClient().TemplateInstances(cli.Namespace()).Create(templateinstancecopy.(*templateapi.TemplateInstance))

			if !test.expectCreateUpdateSuccess {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())

				err = cli.TemplateClient().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})

	g.It("should pass impersonation update tests", func() {
		// check who can update TemplateInstances (anyone with project write access
		// AND is/can impersonate spec.requester.username)
		for _, test := range tests {
			setUser(cli, test.user)

			templateinstancecopy, err := kapi.Scheme.DeepCopy(dummytemplateinstance)
			o.Expect(err).NotTo(o.HaveOccurred())
			templateinstance, err := cli.AdminTemplateClient().TemplateInstances(cli.Namespace()).Create(templateinstancecopy.(*templateapi.TemplateInstance))
			o.Expect(err).NotTo(o.HaveOccurred())

			newtemplateinstance, err := cli.TemplateClient().TemplateInstances(cli.Namespace()).Update(templateinstance)
			if !test.expectCreateUpdateSuccess {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsInvalid(err) || kerrors.IsForbidden(err)).To(o.BeTrue())
			} else {
				o.Expect(err).NotTo(o.HaveOccurred())
				templateinstance = newtemplateinstance
			}

			// ensure spec (particularly including spec.requester.username) is
			// immutable
			templateinstance.Spec.Requester.Username = edituser2.Name
			_, err = cli.TemplateClient().TemplateInstances(cli.Namespace()).Update(templateinstance)
			o.Expect(err).To(o.HaveOccurred())

			err = cli.AdminTemplateClient().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.It("should pass impersonation deletion tests", func() {
		// check who can delete TemplateInstances (anyone with project write access)
		for _, test := range tests {
			setUser(cli, test.user)

			templateinstancecopy, err := kapi.Scheme.DeepCopy(dummytemplateinstance)
			o.Expect(err).NotTo(o.HaveOccurred())
			templateinstance, err := cli.AdminTemplateClient().TemplateInstances(cli.Namespace()).Create(templateinstancecopy.(*templateapi.TemplateInstance))
			o.Expect(err).NotTo(o.HaveOccurred())

			err = cli.TemplateClient().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
			if test.expectDeleteSuccess {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(kerrors.IsForbidden(err)).To(o.BeTrue())

				err = cli.AdminTemplateClient().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})
})
