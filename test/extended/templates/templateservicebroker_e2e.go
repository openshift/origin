package templates

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	rbacapi "k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/test/e2e/framework"

	templateapiv1 "github.com/openshift/api/template/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/bulk"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/client/internalversion"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/client"
	restutil "github.com/openshift/origin/pkg/util/rest"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][templates] templateservicebroker end-to-end test", func() {
	defer g.GinkgoRecover()

	var (
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

		err := exutil.WaitForBuilderAccount(cli.KubeClient().Core().ServiceAccounts(cli.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		brokercli, err = TSBClient(cli)
		o.Expect(err).NotTo(o.HaveOccurred())

		cliUser = &user.DefaultInfo{Name: cli.Username(), Groups: []string{"system:authenticated"}}

		// should have been created before the extended test runs
		template, err = cli.TemplateClient().Template().Templates("openshift").Get("mysql-ephemeral", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		processedtemplate, err = internalversion.NewTemplateProcessorClient(cli.AdminTemplateClient().Template().RESTClient(), "openshift").Process(template)
		o.Expect(err).NotTo(o.HaveOccurred())

		errs := runtime.DecodeList(processedtemplate.Objects, unstructured.UnstructuredJSONScheme)
		o.Expect(errs).To(o.BeEmpty())

		// privatetemplate is an additional template in our namespace
		privatetemplate, err = cli.TemplateClient().Template().Templates(cli.Namespace()).Create(&templateapi.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: "private",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// enable unauthenticated access to the service broker
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

	})

	g.AfterEach(func() {
		err := cli.AdminAuthorizationClient().Authorization().ClusterRoleBindings().Delete(clusterrolebinding.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		// it shouldn't be around, but if it is, clean up the
		// BrokerTemplateInstance object.  The object is not namespaced so the
		// namespace cleanup doesn't catch this.
		cli.AdminTemplateClient().Template().BrokerTemplateInstances().Delete(instanceID, nil)
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

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()

		_, err = brokercli.Provision(ctx, cliUser, instanceID, &api.ProvisionRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
			Context: api.KubernetesContext{
				Platform:  api.ContextPlatformKubernetes,
				Namespace: cli.Namespace(),
			},
			Parameters: map[string]string{
				"MYSQL_USER": "test",
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

		blockOwnerDeletion := true
		o.Expect(templateInstance.Annotations).To(o.Equal(map[string]string{
			api.OpenServiceBrokerInstanceExternalID: templateInstance.Name,
		}))
		o.Expect(templateInstance.OwnerReferences).To(o.ContainElement(metav1.OwnerReference{
			APIVersion:         templateapiv1.SchemeGroupVersion.String(),
			Kind:               "BrokerTemplateInstance",
			Name:               brokerTemplateInstance.Name,
			UID:                brokerTemplateInstance.UID,
			BlockOwnerDeletion: &blockOwnerDeletion,
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

		o.Expect(secret.OwnerReferences).To(o.ContainElement(metav1.OwnerReference{
			APIVersion:         templateapiv1.SchemeGroupVersion.String(),
			Kind:               "BrokerTemplateInstance",
			Name:               brokerTemplateInstance.Name,
			UID:                brokerTemplateInstance.UID,
			BlockOwnerDeletion: &blockOwnerDeletion,
		}))
		o.Expect(secret.Type).To(o.Equal(v1.SecretTypeOpaque))
		o.Expect(secret.Data).To(o.Equal(map[string][]byte{
			"MYSQL_USER": []byte("test"),
		}))

		examplesecret, err := cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("mysql", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(examplesecret.OwnerReferences).To(o.ContainElement(metav1.OwnerReference{
			APIVersion:         templateapiv1.SchemeGroupVersion.String(),
			Kind:               "TemplateInstance",
			Name:               templateInstance.Name,
			UID:                templateInstance.UID,
			BlockOwnerDeletion: &blockOwnerDeletion,
		}))
		o.Expect(examplesecret.Data["database-user"]).To(o.BeEquivalentTo("test"))
		o.Expect(examplesecret.Data["database-password"]).To(o.MatchRegexp("^[a-zA-Z0-9]{16}$"))
	}

	bind := func() {
		g.By("binding to a service")

		bind, err := brokercli.Bind(context.Background(), cliUser, instanceID, bindingID, &api.BindRequest{
			ServiceID: service.ID,
			PlanID:    plan.ID,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		brokerTemplateInstance, err := cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(brokerTemplateInstance.Spec.BindingIDs).To(o.Equal([]string{bindingID}))

		o.Expect(bind.Credentials).To(o.HaveKey("uri"))
		o.Expect(bind.Credentials["uri"]).To(o.HavePrefix("mysql://"))
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
		err := cli.TemplateClient().Template().Templates(cli.Namespace()).Delete(privatetemplate.Name, &metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = brokercli.Deprovision(context.Background(), cliUser, instanceID)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		restmapper := restutil.DefaultMultiRESTMapper()

		_, config, err := configapi.GetInternalKubeClient(exutil.KubeConfigPath(), nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		// check the namespace is empty
		for gvk := range legacyscheme.Scheme.AllKnownTypes() {
			if gvk.Version == runtime.APIVersionInternal {
				continue
			}

			switch gvk.GroupKind() {
			case kapi.Kind("Event"),
				kapi.Kind("ServiceAccount"),
				kapi.Kind("Secret"),
				kapi.Kind("RoleBinding"),
				rbacapi.Kind("RoleBinding"),
				authorizationapi.LegacyKind("RoleBinding"),
				authorizationapi.Kind("RoleBinding"),
				schema.GroupKind{Group: "events.k8s.io", Kind: "Event"}:
				continue
			}

			mapping, err := restmapper.RESTMapping(gvk.GroupKind())
			if meta.IsNoMatchError(err) {
				continue
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
				continue
			}

			restcli, err := bulk.ClientMapperFromConfig(config).ClientForMapping(mapping)
			o.Expect(err).NotTo(o.HaveOccurred())

			// list all objects
			obj, err := restcli.Get().Resource(mapping.Resource).Namespace(cli.Namespace()).Do().Get()
			if kerrors.IsNotFound(err) || kerrors.IsMethodNotSupported(err) {
				continue
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			list, err := meta.ExtractList(obj)
			o.Expect(err).NotTo(o.HaveOccurred())

			if gvk.GroupKind() == kapi.Kind("Pod") {
				// pods stick around for a while after deprovision because of
				// graceful deletion.  As long as every pod deletion timestamp
				// is set, that'll have to do.
				for _, obj := range list {
					meta, err := meta.Accessor(obj)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(meta.GetDeletionTimestamp()).NotTo(o.BeNil())
				}

			} else {
				if len(list) > 0 {
					fmt.Fprintf(g.GinkgoWriter, "error: found %d objects of GVK %s", len(list), gvk.String())
				}
				o.Expect(list).To(o.BeEmpty())
			}
		}
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

		g.It("should pass an end-to-end test", func() {
			catalog()
			provision()
			bind()
			unbind()
			// unbinding a second time should result in a gone message, but not an error
			unbind()
			deprovision()

			provision()
			bind()
			g.By("deleting the template instance that was bound")
			err := cli.Run("delete").Args("templateinstance", "--all").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			unbind()
			// unbinding a second time should result in a gone message, but not an error
			unbind()

		})
	})
})
