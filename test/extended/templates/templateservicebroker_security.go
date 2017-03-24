package templates

import (
	"crypto/tls"
	"net/http"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	"github.com/openshift/origin/pkg/openservicebroker/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
)

var _ = g.Describe("[templates] templateservicebroker security test", func() {
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
		var err error

		template, err = cli.Client().Templates("openshift").Get("cakephp-mysql-example")
		o.Expect(err).NotTo(o.HaveOccurred())

		clusterrolebinding, err = cli.AdminClient().ClusterRoleBindings().Create(&authorizationapi.ClusterRoleBinding{
			ObjectMeta: kapi.ObjectMeta{
				Name: cli.Namespace() + "templateservicebroker-client-binding",
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

		adminClientConfig, err := testutil.GetClusterAdminClientConfig(exutil.KubeConfigPath())
		o.Expect(err).NotTo(o.HaveOccurred())

		brokercli = client.NewClient(&http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}, adminClientConfig.Host+templateapi.ServiceBrokerRoot)

		viewuser = createUser(cli, "viewuser", bootstrappolicy.ViewRoleName)
		edituser = createUser(cli, "edituser", bootstrappolicy.EditRoleName)
		nopermsuser = createUser(cli, "nopermsuser", "")
	})

	g.AfterEach(func() {
		deleteUser(cli, viewuser)
		deleteUser(cli, edituser)
		deleteUser(cli, nopermsuser)

		err := cli.AdminClient().ClusterRoleBindings().Delete(clusterrolebinding.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		cli.AdminTemplateClient().BrokerTemplateInstances().Delete(instanceID, nil)
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
		_, err := brokercli.Provision(context.Background(), instanceID, &api.ProvisionRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
			Parameters: map[string]string{
				templateapi.NamespaceParameterKey:         cli.Namespace(),
				templateapi.RequesterUsernameParameterKey: username,
			},
		})
		return err
	}

	bind := func(username string) error {
		g.By("binding to a service")
		_, err := brokercli.Bind(context.Background(), instanceID, bindingID, &api.BindRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
			Parameters: map[string]string{
				templateapi.RequesterUsernameParameterKey: username,
			},
		})
		return err
	}

	g.It("should pass security tests", func() {
		catalog()

		g.By("having no permissions to the namespace, provision should fail with 403")
		err := provision(nopermsuser.Name)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
		o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

		g.By("having no permissions to the namespace, no BrokerTemplateInstance should be created")
		_, err = cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		g.By("having view permissions to the namespace, provision should fail with 403")
		err = provision(viewuser.Name)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
		o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

		g.By("having view permissions to the namespace, no BrokerTemplateInstance should be created")
		_, err = cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		g.By("having edit permissions to the namespace, provision should succeed")
		err = provision(edituser.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

		g.By("having no permissions to the namespace, bind should fail with 403")
		err = bind(nopermsuser.Name)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
		o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

		g.By("having no permissions to the namespace, the BrokerTemplateInstance should be unchanged")
		brokerTemplateInstance, err = cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

		g.By("having view permissions to the namespace, bind should fail with 403") // view does not enable reading Secrets
		err = bind(viewuser.Name)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err).To(o.BeAssignableToTypeOf(&client.ServerError{}))
		o.Expect(err.(*client.ServerError).StatusCode).To(o.Equal(http.StatusForbidden))

		g.By("having view permissions to the namespace, the BrokerTemplateInstance should be unchanged")
		brokerTemplateInstance, err = cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))

		g.By("having edit permissions to the namespace, bind should succeed")
		err = bind(edituser.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err = cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.Equal([]string{bindingID}))
	})
})
