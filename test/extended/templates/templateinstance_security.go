package templates

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
)

// Check that objects created through the TemplateInstance mechanism are done
// impersonating the requester, and that privilege escalation is not possible.
var _ = g.Describe("[templates] templateinstance security tests", func() {
	defer g.GinkgoRecover()

	var (
		cli = exutil.NewCLI("templates", exutil.KubeConfigPath())

		adminuser, edituser *userapi.User

		dummyservice = &kapi.Service{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "service",
				Namespace: "${NAMESPACE}",
			},
			Spec: kapi.ServiceSpec{
				Ports: []kapi.ServicePort{
					{
						Port: 1,
					},
				},
			},
		}

		dummyrolebinding = &authorizationapi.RoleBinding{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "rolebinding",
				Namespace: "${NAMESPACE}",
			},
			RoleRef: kapi.ObjectReference{
				Name: bootstrappolicy.AdminRoleName,
			},
		}
	)

	g.BeforeEach(func() {
		adminuser = createUser(cli, "adminuser", bootstrappolicy.AdminRoleName)
		edituser = createUser(cli, "edituser", bootstrappolicy.EditRoleName)
	})

	g.AfterEach(func() {
		deleteUser(cli, adminuser)
		deleteUser(cli, edituser)
	})

	g.It("should pass security tests", func() {
		tests := []struct {
			by              string
			user            *userapi.User
			namespace       string
			objects         []runtime.Object
			expectCondition templateapi.TemplateInstanceConditionType
			checkOK         func(namespace string) bool
		}{
			{
				by:              "checking edituser can create an object in a permitted namespace",
				user:            edituser,
				namespace:       cli.Namespace(),
				objects:         []runtime.Object{dummyservice},
				expectCondition: templateapi.TemplateInstanceReady,
				checkOK: func(namespace string) bool {
					_, err := cli.AdminKubeClient().Services(namespace).Get(dummyservice.Name)
					return err == nil
				},
			},
			{
				by:              "checking edituser can't create an object in a non-permitted namespace",
				user:            edituser,
				namespace:       "default",
				objects:         []runtime.Object{dummyservice},
				expectCondition: templateapi.TemplateInstanceInstantiateFailure,
				checkOK: func(namespace string) bool {
					_, err := cli.AdminKubeClient().Services(namespace).Get(dummyservice.Name)
					return err != nil && kerrors.IsNotFound(err)
				},
			},
			{
				by:              "checking edituser can't create a privileged object",
				user:            edituser,
				namespace:       cli.Namespace(),
				objects:         []runtime.Object{dummyrolebinding},
				expectCondition: templateapi.TemplateInstanceInstantiateFailure,
				checkOK: func(namespace string) bool {
					_, err := cli.AdminClient().RoleBindings(namespace).Get(dummyrolebinding.Name)
					return err != nil && kerrors.IsNotFound(err)
				},
			},
			{
				by:              "checking adminuser can create a privileged object",
				user:            adminuser,
				namespace:       cli.Namespace(),
				objects:         []runtime.Object{dummyrolebinding},
				expectCondition: templateapi.TemplateInstanceReady,
				checkOK: func(namespace string) bool {
					_, err := cli.AdminClient().RoleBindings(namespace).Get(dummyrolebinding.Name)
					return err == nil
				},
			},
		}

		for _, test := range tests {
			g.By(test.by)
			cli.ChangeUser(test.user.Name)

			secret, err := cli.KubeClient().Secrets(cli.Namespace()).Create(&kapi.Secret{
				ObjectMeta: kapi.ObjectMeta{
					Name: "secret",
				},
				Data: map[string][]byte{
					"NAMESPACE": []byte(test.namespace),
				},
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			templateinstance := &templateapi.TemplateInstance{
				ObjectMeta: kapi.ObjectMeta{
					Name: "templateinstance",
				},
				Spec: templateapi.TemplateInstanceSpec{
					Template: templateapi.Template{
						ObjectMeta: kapi.ObjectMeta{
							Name:      "template",
							Namespace: cli.Namespace(),
						},
						Parameters: []templateapi.Parameter{
							{
								Name:     "NAMESPACE",
								Required: true,
							},
						},
					},
					Secret: kapi.LocalObjectReference{
						Name: "secret",
					},
				},
			}

			err = templateapi.AddObjectsToTemplate(&templateinstance.Spec.Template, test.objects, latest.Versions...)
			o.Expect(err).NotTo(o.HaveOccurred())

			templateinstance, err = cli.TemplateClient().TemplateInstances(cli.Namespace()).Create(templateinstance)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = wait.Poll(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				templateinstance, err = cli.TemplateClient().TemplateInstances(cli.Namespace()).Get(templateinstance.Name)
				if err != nil {
					return false, err
				}
				return len(templateinstance.Status.Conditions) != 0, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			cond := &templateinstance.Status.Conditions[0]
			o.Expect(cond.Status).To(o.Equal(kapi.ConditionTrue))
			o.Expect(cond.Type).To(o.Equal(test.expectCondition))
			o.Expect(test.checkOK(test.namespace)).To(o.BeTrue())

			err = cli.TemplateClient().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = cli.KubeClient().Secrets(cli.Namespace()).Delete(secret.Name, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
