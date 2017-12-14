package templates

import (
	"fmt"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/test/e2e/framework"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/client"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][templates] templateservicebroker security test", func() {
	defer g.GinkgoRecover()

	var (
		cli                = exutil.NewCLI("templates", exutil.KubeConfigPath())
		instanceID         = uuid.NewRandom().String()
		bindingID          = uuid.NewRandom().String()
		template           *templateapi.Template
		clusterrolebinding *authorizationapi.ClusterRoleBinding
		brokercli          client.Client
		service            *api.Service
		plan               *api.Plan
		viewuser           *userapi.User
		edituser           *userapi.User
		nopermsuser        *userapi.User
	)

	g.BeforeEach(func() {
		framework.SkipIfProviderIs("gce")

		err := exutil.WaitForBuilderAccount(cli.KubeClient().Core().ServiceAccounts(cli.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		brokercli, err = TSBClient(cli)
		o.Expect(err).NotTo(o.HaveOccurred())

		template, err = cli.TemplateClient().Template().Templates("openshift").Get("mysql-ephemeral", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		clusterrolebinding, err = cli.AdminAuthorizationClient().Authorization().ClusterRoleBindings().Create(&authorizationapi.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: cli.Namespace() + "templateservicebroker-client",
			},
			RoleRef: kapi.ObjectReference{
				Name: bootstrappolicy.TemplateServiceBrokerClientRoleName,
			},
			Subjects: []kapi.ObjectReference{
				{
					Kind: authorizationapi.GroupKind,
					Name: bootstrappolicy.UnauthenticatedGroup,
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		viewuser = createUser(cli, "viewuser", bootstrappolicy.ViewRoleName)
		edituser = createUser(cli, "edituser", bootstrappolicy.EditRoleName)
		nopermsuser = createUser(cli, "nopermsuser", "")
	})

	g.AfterEach(func() {
		deleteUser(cli, viewuser)
		deleteUser(cli, edituser)
		deleteUser(cli, nopermsuser)

		err := cli.AdminAuthorizationClient().Authorization().ClusterRoleBindings().Delete(clusterrolebinding.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		cli.AdminTemplateClient().Template().BrokerTemplateInstances().Delete(instanceID, nil)
	})

	catalog := func() {
		g.By("returning a catalog")
		catalog, err := brokercli.Catalog(context.Background())
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
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
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
			templateInstance, err := cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(instanceID, metav1.GetOptions{})
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
		_, err := brokercli.Bind(context.Background(), &user.DefaultInfo{Name: username, Groups: []string{"system:authenticated"}}, instanceID, bindingID, &api.BindRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
		})
		return err
	}

	unbind := func(username string) error {
		g.By("unbinding from a service")
		return brokercli.Unbind(context.Background(), &user.DefaultInfo{Name: username, Groups: []string{"system:authenticated"}}, instanceID, bindingID)
	}

	deprovision := func(username string) error {
		g.By("deprovisioning a service")
		return brokercli.Deprovision(context.Background(), &user.DefaultInfo{Name: username, Groups: []string{"system:authenticated"}}, instanceID)
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
			_, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

			g.By("having view permissions to the namespace, provision should fail with 403")
			err = provision(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, no BrokerTemplateInstance should be created")
			_, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

			g.By("having edit permissions to the namespace, provision should succeed")
			err = provision(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			brokerTemplateInstance, err := cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

			g.By("having no permissions to the namespace, bind should fail with 403")
			err = bind(nopermsuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having no permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			brokerTemplateInstance, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

			g.By("having view permissions to the namespace, bind should fail with 403") // view does not enable reading Secrets
			err = bind(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			brokerTemplateInstance, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

			g.By("having edit permissions to the namespace, bind should succeed")
			err = bind(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			brokerTemplateInstance, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.Equal([]string{bindingID}))

			g.By("having no permissions to the namespace, unbind should fail with 403")
			err = unbind(nopermsuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having no permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err := cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having view permissions to the namespace, unbind should fail with 403")
			err = unbind(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having edit permissions to the namespace, unbind should succeed")
			err = unbind(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			brokerTemplateInstance, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.BeEmpty())

			g.By("having no permissions to the namespace, deprovision should fail with 403")
			err = deprovision(nopermsuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having no permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having view permissions to the namespace, deprovision should fail with 403")
			err = deprovision(viewuser.Name)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
			o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

			g.By("having view permissions to the namespace, the BrokerTemplateInstance should be unchanged")
			newBrokerTemplateInstance, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(newBrokerTemplateInstance).To(o.Equal(brokerTemplateInstance))

			g.By("having edit permissions to the namespace, deprovision should succeed")
			err = deprovision(edituser.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())
		})
	})
})
