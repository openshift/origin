package templates

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/test/e2e/framework"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/client"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][templates] templateservicebroker bind test", func() {
	defer g.GinkgoRecover()

	var (
		cli                = exutil.NewCLI("templates", exutil.KubeConfigPath())
		instanceID         = "aadda50d-d92c-402d-bd29-5ed2095aad2c"
		bindingID          = uuid.NewRandom().String()
		serviceID          = "d261a5c9-db37-40b5-ac0f-5709e0e3aac4"
		fixture            = exutil.FixturePath("testdata", "templates", "templateservicebroker_bind.yaml")
		clusterrolebinding *authorizationapi.ClusterRoleBinding
		brokercli          client.Client
		cliUser            user.Info
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			framework.SkipIfProviderIs("gce")

			var err error

			brokercli, err = TSBClient(cli)
			o.Expect(err).NotTo(o.HaveOccurred())

			cliUser = &user.DefaultInfo{Name: cli.Username(), Groups: []string{"system:authenticated"}}

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

			err = cli.AsAdmin().Run("new-app").Args(fixture, "-p", "NAMESPACE="+cli.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// wait for templateinstance controller to do its thing
			err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
				templateinstance, err := cli.TemplateClient().Template().TemplateInstances(cli.Namespace()).Get(instanceID, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				for _, c := range templateinstance.Status.Conditions {
					if c.Reason == "Failed" && c.Status == kapi.ConditionTrue {
						return false, fmt.Errorf("failed condition: %s", c.Message)
					}
					if c.Reason == "Created" && c.Status == kapi.ConditionTrue {
						return true, nil
					}
				}

				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				ns := cli.Namespace()
				cli.SetNamespace("openshift-template-service-broker")
				exutil.DumpPodStates(cli.AsAdmin())
				exutil.DumpPodLogsStartingWith("", cli.AsAdmin())
				cli.SetNamespace(ns)
			}

			err := cli.AdminAuthorizationClient().Authorization().ClusterRoleBindings().Delete(clusterrolebinding.Name, nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = cli.AdminTemplateClient().Template().BrokerTemplateInstances().Delete(instanceID, &metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should pass bind tests", func() {
			svc, err := cli.KubeClient().Core().Services(cli.Namespace()).Get("service", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			bind, err := brokercli.Bind(context.Background(), cliUser, instanceID, bindingID, &api.BindRequest{
				ServiceID: serviceID,
				PlanID:    uuid.NewRandom().String(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(bind.Credentials).To(o.HaveKeyWithValue("configmap-username", "configmap-username"))
			o.Expect(bind.Credentials).To(o.HaveKeyWithValue("secret-username", "secret-username"))
			o.Expect(bind.Credentials).To(o.HaveKeyWithValue("secret-password", "c2VjcmV0LXBhc3N3b3Jk"))
			o.Expect(bind.Credentials).To(o.HaveKeyWithValue("service-uri", "http://"+svc.Spec.ClusterIP+":1234"))
			o.Expect(bind.Credentials).To(o.HaveKeyWithValue("route-uri", "http://host/path"))
		})
	})
})
