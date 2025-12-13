package templates

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
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

	g.It("should create and delete objects from varying API groups [apigroup:template.openshift.io][apigroup:route.openshift.io]", g.Label("Size:M"), func() {
		g.By("creating a template instance")
		err := cli.Run("create").Args("-f", fixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for templateinstance controller to do its thing
		err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
			templateinstance, err := cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), "templateinstance", metav1.GetOptions{})
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

		ctx := context.Background()
		// check everything was created as expected
		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get(ctx, "secret", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.KubeClient().AppsV1().Deployments(cli.Namespace()).Get(ctx, "deployment", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get(ctx, "route", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get(ctx, "newroute", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Delete(ctx, "templateinstance", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deleting the template instance")
		err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
			_, err := cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(ctx, "templateinstance", metav1.GetOptions{})
			if kapierrs.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// check everything was deleted as expected
		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get(ctx, "secret", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		_, err = cli.KubeClient().AppsV1().Deployments(cli.Namespace()).Get(ctx, "deployment", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get(ctx, "route", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		_, err = cli.RouteClient().RouteV1().Routes(cli.Namespace()).Get(ctx, "newroute", metav1.GetOptions{})
		if !kapierrs.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

	})

})
