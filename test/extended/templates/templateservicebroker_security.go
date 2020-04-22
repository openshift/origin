package templates

import (
	"fmt"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	templatev1 "github.com/openshift/api/template/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/test/extended/templates/openservicebroker/api"
	"github.com/openshift/origin/test/extended/templates/openservicebroker/client"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex][Feature:Templates] templateservicebroker security test", func() {
	defer g.GinkgoRecover()
	ctx := context.Background()

	var (
		cli                = exutil.NewCLI("templates")
		instanceID         = uuid.NewRandom().String()
		bindingID          = uuid.NewRandom().String()
		template           *templatev1.Template
		clusterrolebinding *authorizationv1.ClusterRoleBinding
		brokercli          client.Client
		service            *api.Service
		plan               *api.Plan
		viewuser           *userv1.User
		edituser           *userv1.User
		nopermsuser        *userv1.User
	)

	g.JustBeforeEach(func() {
		var err error
		brokercli, err = TSBClient(cli)
		if kerrors.IsNotFound(err) {
			e2eskipper.Skipf("The template service broker is not installed: %v", err)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		template, err = cli.TemplateClient().TemplateV1().Templates("openshift").Get(ctx, "mysql-ephemeral", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		clusterrolebinding, err = cli.AdminAuthorizationClient().AuthorizationV1().ClusterRoleBindings().Create(ctx, &authorizationv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: cli.Namespace() + "templateservicebroker-client",
			},
			RoleRef: corev1.ObjectReference{
				Name: "system:openshift:templateservicebroker-client",
			},
			Subjects: []corev1.ObjectReference{
				{
					Kind: authorizationv1.GroupKind,
					Name: "system:unauthenticated",
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		viewuser = createUser(cli, "viewuser", "view")
		edituser = createUser(cli, "edituser", "edit")
		nopermsuser = createUser(cli, "nopermsuser", "")
	})

	g.AfterEach(func() {
		deleteUser(cli, viewuser)
		deleteUser(cli, edituser)
		deleteUser(cli, nopermsuser)

		err := cli.AdminAuthorizationClient().AuthorizationV1().ClusterRoleBindings().Delete(ctx, clusterrolebinding.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Delete(ctx, instanceID, metav1.DeleteOptions{})
	})

	catalog := func() {
		g.By("returning a catalog")
		catalog, err := brokercli.Catalog(ctx)
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, service = range catalog.Services {
			if service.ID == string(template.UID) {
				o.Expect(service.Plans).NotTo(o.BeEmpty())
				plan = &service.Plans[0]
				break
			}
		}
		o.Expect(service.ID).To(o.BeEquivalentTo(template.UID))
	}

	provision := func(username string) error {
		g.By("provisioning a service")
		ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()

		_, err := brokercli.Provision(ctx, &user.DefaultInfo{Name: username, Groups: []string{"system:authenticated"}}, instanceID, &api.ProvisionRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
			Context: api.KubernetesContext{
				Platform:  api.ContextPlatformKubernetes,
				Namespace: cli.Namespace(),
			},
		})
		if err != nil {
			templateInstance, err := cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(ctx, instanceID, metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "error getting TemplateInstance after failed provision: %v\n", err)
			} else {
				err := dumpObjectReadiness(cli, templateInstance)
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error running dumpObjectReadiness: %v\n", err)
				}
			}
		}
		return err
	}

	bind := func(username string) error {
		g.By("binding to a service")
		_, err := brokercli.Bind(ctx, &user.DefaultInfo{Name: username, Groups: []string{"system:authenticated"}}, instanceID, bindingID, &api.BindRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
		})
		return err
	}

	unbind := func(username string) error {
		g.By("unbinding from a service")
		return brokercli.Unbind(ctx, &user.DefaultInfo{Name: username, Groups: []string{"system:authenticated"}}, instanceID, bindingID)
	}

	deprovision := func(username string) error {
		g.By("deprovisioning a service")
		return brokercli.Deprovision(ctx, &user.DefaultInfo{Name: username, Groups: []string{"system:authenticated"}}, instanceID)
	}

	g.Context("", func() {
		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				ns := cli.Namespace()
				cli.SetNamespace("openshift-template-service-broker")
				exutil.DumpPodStates(cli.AsAdmin())
				exutil.DumpPodLogsStartingWith("", cli.AsAdmin())
				cli.SetNamespace(ns)

				exutil.DumpPodStates(cli)
				exutil.DumpPodLogsStartingWith("", cli)
			}
		})

		g.It("should pass security tests", func() {
			catalog()

			g.By("having no permissions to the namespace, provision should fail with 403")
			err := provision(nopermsuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having no permissions to the namespace, no BrokerTemplateInstance should be created")
			_, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

			g.By("having view permissions to the namespace, provision should fail with 403")
			err = provision(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, no BrokerTemplateInstance should be created")
			_, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

			g.By("having edit permissions to the namespace, provision should succeed")
			err = provision(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			brokerTemplateInstance, err := cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

			g.By("having no permissions to the namespace, bind should fail with 403")
			err = bind(nopermsuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having no permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			brokerTemplateInstance, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

			g.By("having view permissions to the namespace, bind should fail with 403") // view does not enable reading Secrets
			err = bind(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			brokerTemplateInstance, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

			g.By("having edit permissions to the namespace, bind should succeed")
			err = bind(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			brokerTemplateInstance, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.Equal([]string{bindingID}))

			g.By("having no permissions to the namespace, unbind should fail with 403")
			err = unbind(nopermsuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having no permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err := cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having view permissions to the namespace, unbind should fail with 403")
			err = unbind(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having edit permissions to the namespace, unbind should succeed")
			err = unbind(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			brokerTemplateInstance, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.BeEmpty())

			g.By("having no permissions to the namespace, deprovision should fail with 403")
			err = deprovision(nopermsuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having no permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having view permissions to the namespace, deprovision should fail with 403")
			err = deprovision(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having edit permissions to the namespace, deprovision should succeed")
			err = deprovision(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Get(ctx, instanceID, metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())
		})
	})
})
