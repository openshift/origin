package templates

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

// ensure that we can instantiate Kubernetes and OpenShift objects, legacy and
// non-legacy, from a range of API groups.
var _ = g.Describe("[sig-devex][Feature:Templates] templateinstance object kinds test", func() {
	defer g.GinkgoRecover()

	var (
		fixture = exutil.FixturePath("testdata", "templates", "templateinstance_objectkinds.yaml")
		cli     = exutil.NewCLI("templates")
	)

	g.It("should create and delete objects from varying API groups", func() {
		g.Skip("Bug 1731222: skip template tests until we determine what is broken")
		g.By("creating a template instance")
		err := cli.Run("create").Args("-f", fixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for templateinstance controller to do its thing
		err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
			templateinstance, err := cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get("templateinstance", metav1.GetOptions{})
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

		// check everything was created as expected
		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("secret", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.KubeClient().AppsV1().Deployments(cli.Namespace()).Get("deployment", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get("route", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get("newroute", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Delete("templateinstance", nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deleting the template instance")
		err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
			_, err := cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get("templateinstance", metav1.GetOptions{})
			if kapierrs.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// check everything was deleted as expected
		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("secret", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		_, err = cli.KubeClient().AppsV1().Deployments(cli.Namespace()).Get("deployment", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get("route", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get("newroute", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

	})

})
