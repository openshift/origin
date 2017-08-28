package templates

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/client"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][templates] templateservicebroker end-to-end test", func() {
	defer g.GinkgoRecover()

	var (
		tsbOC               = exutil.NewCLI("openshift-template-service-broker", exutil.KubeConfigPath())
		portForwardCmdClose func() error

		cli                = exutil.NewCLI("templates", exutil.KubeConfigPath())
		instanceID         = uuid.NewRandom().String()
		bindingID          = uuid.NewRandom().String()
		template           *templateapi.Template
		processedtemplate  *templateapi.Template
		privatetemplate    *templateapi.Template
		clusterrolebinding *authorizationapi.ClusterRoleBinding
		brokercli          client.Client
		service            *api.Service
		plan               *api.Plan
		cliUser            user.Info
	)

	g.BeforeEach(func() {
		framework.SkipIfProviderIs("gce")
		brokercli, portForwardCmdClose = EnsureTSB(tsbOC)

		cliUser = &user.DefaultInfo{Name: cli.Username(), Groups: []string{"system:authenticated"}}
		var err error

		// should have been created before the extended test runs
		template, err = cli.TemplateClient().Template().Templates("openshift").Get("cakephp-mysql-example", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		processedtemplate, err = cli.AdminClient().TemplateConfigs("openshift").Create(template)
		o.Expect(err).NotTo(o.HaveOccurred())

		errs := runtime.DecodeList(processedtemplate.Objects, unstructured.UnstructuredJSONScheme)
		o.Expect(errs).To(o.BeEmpty())

		// privatetemplate is an additional template in our namespace
		privatetemplate, err = cli.Client().Templates(cli.Namespace()).Create(&templateapi.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: "private",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// enable unauthenticated access to the service broker
		clusterrolebinding, err = cli.AdminClient().ClusterRoleBindings().Create(&authorizationapi.ClusterRoleBinding{
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

	})

	g.AfterEach(func() {
		framework.SkipIfProviderIs("gce")
		err := cli.AdminClient().ClusterRoleBindings().Delete(clusterrolebinding.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		// it shouldn't be around, but if it is, clean up the
		// BrokerTemplateInstance object.  The object is not namespaced so the
		// namespace cleanup doesn't catch this.
		cli.AdminTemplateClient().Template().BrokerTemplateInstances().Delete(instanceID, nil)

		err = portForwardCmdClose()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	catalog := func() {
		g.By("returning a catalog")
		catalog, err := brokercli.Catalog(context.Background())
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, s := range catalog.Services {
			// confirm our private template isn't returned
			o.Expect(s.ID).NotTo(o.BeEquivalentTo(privatetemplate.UID))
			if s.ID == string(template.UID) {
				service = s
				o.Expect(service.Plans).NotTo(o.BeEmpty())
				plan = &service.Plans[0]
			}
		}
		o.Expect(service.ID).To(o.BeEquivalentTo(template.UID))
	}

	provision := func() {
		g.By("provisioning a service")
		// confirm our private template can't be provisioned
		_, err := brokercli.Provision(context.Background(), cliUser, instanceID, &api.ProvisionRequest{
			ServiceID: string(privatetemplate.UID),
			PlanID:    plan.ID,
			Context: api.KubernetesContext{
				Platform:  api.ContextPlatformKubernetes,
				Namespace: cli.Namespace(),
			},
		})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("not found"))

		_, err = brokercli.Provision(context.Background(), cliUser, instanceID, &api.ProvisionRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
			Context: api.KubernetesContext{
				Platform:  api.ContextPlatformKubernetes,
				Namespace: cli.Namespace(),
			},
			Parameters: map[string]string{
				"DATABASE_USER": "test",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		templateInstance, err := cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(instanceID, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		secret, err := cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get(instanceID, metav1.GetOptions{})
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
			Secret: &kapi.LocalObjectReference{
				Name: secret.Name,
			},
			Requester: &templateapi.TemplateInstanceRequester{
				Username: cli.Username(),
				Groups:   []string{"system:authenticated"},
			},
		}))

		o.Expect(templateInstance.Status.Conditions).To(o.HaveLen(1))
		o.Expect(templateInstance.HasCondition(templateapi.TemplateInstanceReady, kapi.ConditionTrue)).To(o.Equal(true))

		o.Expect(templateInstance.Status.Objects).To(o.HaveLen(len(template.Objects)))
		for i, obj := range templateInstance.Status.Objects {
			u := processedtemplate.Objects[i].(*unstructured.Unstructured)
			o.Expect(obj.Ref.Kind).To(o.Equal(u.GetKind()))
			o.Expect(obj.Ref.Namespace).To(o.Equal(cli.Namespace()))
			o.Expect(obj.Ref.Name).To(o.Equal(u.GetName()))
			o.Expect(obj.Ref.UID).ToNot(o.BeEmpty())
		}

		o.Expect(secret.Type).To(o.Equal(v1.SecretTypeOpaque))
		o.Expect(secret.Data).To(o.Equal(map[string][]byte{
			"DATABASE_USER": []byte("test"),
		}))

		examplesecret, err := cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("cakephp-mysql-example", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(examplesecret.Labels[templateapi.TemplateInstanceLabel]).To(o.Equal(instanceID))
		o.Expect(examplesecret.OwnerReferences).To(o.ContainElement(metav1.OwnerReference{
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

		// create some more objects to exercise bind a bit more
		bindconfigmap, err := cli.KubeClient().CoreV1().ConfigMaps(cli.Namespace()).Create(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bindsecret",
				Annotations: map[string]string{
					templateapi.ExposeAnnotationPrefix + "configmap-username": "{.data['username']}",
				},
				Labels: map[string]string{
					templateapi.TemplateInstanceLabel: instanceID,
				},
			},
			Data: map[string]string{
				"username": "configmap-username",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		bindsecret, err := cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bindsecret",
				Annotations: map[string]string{
					templateapi.ExposeAnnotationPrefix + "secret-username":       "{.data['username']}",
					templateapi.Base64ExposeAnnotationPrefix + "secret-password": "{.data['password']}",
				},
				Labels: map[string]string{
					templateapi.TemplateInstanceLabel: instanceID,
				},
			},
			Data: map[string][]byte{
				"username": []byte("secret-username"),
				"password": []byte("secret-password"),
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		bindservice, err := cli.KubeClient().CoreV1().Services(cli.Namespace()).Create(&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bindservice",
				Annotations: map[string]string{
					templateapi.ExposeAnnotationPrefix + "service-uri": `http://{.spec.clusterIP}:{.spec.ports[?(.name=="port")].port}`,
				},
				Labels: map[string]string{
					templateapi.TemplateInstanceLabel: instanceID,
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name: "port",
						Port: 1234,
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		bindroute, err := cli.Client().Routes(cli.Namespace()).Create(&routeapi.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bindroute",
				Annotations: map[string]string{
					templateapi.ExposeAnnotationPrefix + "route-uri": "http://{.spec.host}{.spec.path}",
				},
				Labels: map[string]string{
					templateapi.TemplateInstanceLabel: instanceID,
				},
			},
			Spec: routeapi.RouteSpec{
				Host: "host",
				Path: "/path",
				To: routeapi.RouteTargetReference{
					Kind: "Service",
					Name: "bindservice",
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		bind, err := brokercli.Bind(context.Background(), cliUser, instanceID, bindingID, &api.BindRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.Equal([]string{bindingID}))

		o.Expect(bind.Credentials).To(o.HaveKey("uri"))
		o.Expect(bind.Credentials["uri"]).To(o.HavePrefix("http://"))

		o.Expect(bind.Credentials).To(o.HaveKeyWithValue("configmap-username", "configmap-username"))
		o.Expect(bind.Credentials).To(o.HaveKeyWithValue("secret-username", "secret-username"))
		o.Expect(bind.Credentials).To(o.HaveKeyWithValue("secret-password", "c2VjcmV0LXBhc3N3b3Jk"))
		o.Expect(bind.Credentials).To(o.HaveKeyWithValue("service-uri", "http://"+bindservice.Spec.ClusterIP+":1234"))
		o.Expect(bind.Credentials).To(o.HaveKeyWithValue("route-uri", "http://host/path"))

		err = cli.KubeClient().CoreV1().ConfigMaps(cli.Namespace()).Delete(bindconfigmap.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Delete(bindsecret.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = cli.KubeClient().CoreV1().Services(cli.Namespace()).Delete(bindservice.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = cli.Client().Routes(cli.Namespace()).Delete(bindroute.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	unbind := func() {
		g.By("unbinding from a service")
		err := brokercli.Unbind(context.Background(), cliUser, instanceID, bindingID)
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.HaveLen(0))
	}

	deprovision := func() {
		g.By("deprovisioning a service")
		err := brokercli.Deprovision(context.Background(), cliUser, instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		_, err = cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(instanceID, metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get(instanceID, metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		// TODO: check that the namespace is actually empty at this point
		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("examplesecret", metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())
	}

	g.It("should pass an end-to-end test", func() {
		framework.SkipIfProviderIs("gce")
		catalog()
		provision()
		bind()
		unbind()
		deprovision()
	})
})
