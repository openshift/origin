package templates

import (
	"crypto/tls"
	"net/http"
	"strconv"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	"github.com/openshift/origin/pkg/openservicebroker/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateapiv1 "github.com/openshift/origin/pkg/template/api/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
)

var _ = g.Describe("[templates] templateservicebroker end-to-end test", func() {
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
	})

	g.AfterEach(func() {
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

	provision := func() {
		g.By("provisioning a service")
		_, err := brokercli.Provision(context.Background(), instanceID, &api.ProvisionRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
			Parameters: map[string]string{
				templateapi.NamespaceParameterKey:         cli.Namespace(),
				templateapi.RequesterUsernameParameterKey: cli.Username(),
				"DATABASE_USER":                           "test",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		templateInstance, err := cli.TemplateClient().TemplateInstances(cli.Namespace()).Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		secret, err := cli.KubeClient().Secrets(cli.Namespace()).Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(brokerTemplateInstance.Spec).To(o.Equal(templateapi.BrokerTemplateInstanceSpec{
			TemplateInstance: kapi.ObjectReference{
				Kind:      "TemplateInstance",
				Namespace: cli.Namespace(),
				Name:      templateInstance.Name,
				UID:       templateInstance.UID,
			},
			Secret: kapi.ObjectReference{
				Kind:      "Secret",
				Namespace: cli.Namespace(),
				Name:      secret.Name,
				UID:       secret.UID,
			},
		}))

		o.Expect(templateInstance.Spec).To(o.Equal(templateapi.TemplateInstanceSpec{
			Template: *template,
			Secret: kapi.LocalObjectReference{
				Name: secret.Name,
			},
			Requester: &templateapi.TemplateInstanceRequester{
				Username: cli.Username(),
			},
		}))

		o.Expect(templateInstance.Status.Conditions).To(o.HaveLen(1))
		o.Expect(templateInstance.Status.Conditions[0].Type).To(o.Equal(templateapi.TemplateInstanceReady))
		o.Expect(templateInstance.Status.Conditions[0].Status).To(o.Equal(kapi.ConditionTrue))

		o.Expect(secret.Type).To(o.Equal(kapi.SecretTypeOpaque))
		o.Expect(secret.Data).To(o.Equal(map[string][]byte{
			"DATABASE_USER": []byte("test"),
		}))

		examplesecret, err := cli.KubeClient().Secrets(cli.Namespace()).Get("cakephp-mysql-example")
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(examplesecret.Labels[templateapi.TemplateInstanceLabel]).To(o.Equal(instanceID))
		o.Expect(examplesecret.OwnerReferences).To(o.ContainElement(kapi.OwnerReference{
			APIVersion: templateapiv1.SchemeGroupVersion.String(),
			Kind:       "TemplateInstance",
			Name:       templateInstance.Name,
			UID:        templateInstance.UID,
		}))
		o.Expect(examplesecret.Data["database-user"]).To(o.BeEquivalentTo("test"))
		o.Expect(examplesecret.Data["database-password"]).To(o.MatchRegexp("^[a-zA-Z0-9]{16}$"))
	}

	bind := func() {
		g.By("binding to a service")
		bind, err := brokercli.Bind(context.Background(), instanceID, bindingID, &api.BindRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
			Parameters: map[string]string{
				templateapi.RequesterUsernameParameterKey: cli.Username(),
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.Equal([]string{bindingID}))

		services := bind.Credentials["services"].(map[string]interface{})

		service, err := cli.KubeClient().Services(cli.Namespace()).Get("mysql")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services["MYSQL_SERVICE_HOST"]).To(o.Equal(service.Spec.ClusterIP))
		o.Expect(services["MYSQL_SERVICE_PORT"]).To(o.Equal(strconv.Itoa(int(service.Spec.Ports[0].Port))))
	}

	unbind := func() {
		g.By("unbinding from a service")
		err := brokercli.Unbind(context.Background(), instanceID, bindingID)
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))
	}

	deprovision := func() {
		g.By("deprovisioning a service")
		err := brokercli.Deprovision(context.Background(), instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.AdminTemplateClient().BrokerTemplateInstances().Get(instanceID)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		_, err = cli.TemplateClient().TemplateInstances(cli.Namespace()).Get(instanceID)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		_, err = cli.KubeClient().Secrets(cli.Namespace()).Get(instanceID)
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		_, err = cli.KubeClient().Secrets(cli.Namespace()).Get("examplesecret")
		// TODO: uncomment  when GC is enabled
		// o.Expect(err).To(o.HaveOccurred())
		// o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())
	}

	g.It("should pass an end-to-end test", func() {
		catalog()
		provision()
		bind()
		unbind()
		deprovision()
	})
})
