package templates

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	"github.com/openshift/origin/test/extended/templates/openservicebroker/api"
	"github.com/openshift/origin/test/extended/templates/openservicebroker/client"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex][Feature:Templates] templateservicebroker bind test", func() {
	defer g.GinkgoRecover()

	var (
		cli                = exutil.NewCLI("templates")
		instanceID         = "aadda50d-d92c-402d-bd29-5ed2095aad2c"
		bindingID          = uuid.NewRandom().String()
		serviceID          = "d261a5c9-db37-40b5-ac0f-5709e0e3aac4"
		fixture            = exutil.FixturePath("testdata", "templates", "templateservicebroker_bind.yaml")
		clusterrolebinding *authorizationv1.ClusterRoleBinding
		brokercli          client.Client
		cliUser            user.Info
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			var err error
			brokercli, err = TSBClient(cli)
			if kerrors.IsNotFound(err) {
				e2eskipper.Skipf("The template service broker is not installed: %v", err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			cliUser = &user.DefaultInfo{Name: cli.Username(), Groups: []string{"system:authenticated"}}

			// enable unauthenticated access to the service broker
			clusterrolebinding, err = cli.AdminAuthorizationClient().AuthorizationV1().ClusterRoleBindings().Create(context.Background(), &authorizationv1.ClusterRoleBinding{
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

			err = cli.AsAdmin().Run("new-app").Args(fixture, "-p", "NAMESPACE="+cli.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// wait for templateinstance controller to do its thing
			err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
				templateinstance, err := cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), instanceID, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				for _, c := range templateinstance.Status.Conditions {
					if c.Reason == "Failed" && c.Status == corev1.ConditionTrue {
						return false, fmt.Errorf("failed condition: %s", c.Message)
					}
					if c.Reason == "Created" && c.Status == corev1.ConditionTrue {
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

			err := cli.AdminAuthorizationClient().AuthorizationV1().ClusterRoleBindings().Delete(context.Background(), clusterrolebinding.Name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = cli.AdminTemplateClient().TemplateV1().BrokerTemplateInstances().Delete(context.Background(), instanceID, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should pass bind tests", func() {
			g.Skip("Bug 1731222: skip template tests until we determine what is broken")
			svc, err := cli.KubeClient().CoreV1().Services(cli.Namespace()).Get(context.Background(), "service", metav1.GetOptions{})
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
