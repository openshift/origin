package templates

import (
	"context"
	"errors"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"
	templatev1 "github.com/openshift/api/template/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// ensure that template instantiation waits for annotated objects
var _ = g.Describe("[sig-devex][Feature:Templates] templateinstance readiness test", func() {
	defer g.GinkgoRecover()

	var (
		cli              = exutil.NewCLIWithPodSecurityLevel("templates", admissionapi.LevelBaseline)
		template         *templatev1.Template
		templateinstance *templatev1.TemplateInstance
		templatefixture  = exutil.FixturePath("testdata", "templates", "templateinstance_readiness.yaml")
	)

	waitSettle := func() (bool, error) {
		var err error

		// must read the templateinstance before the build/deployment
		templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		build, err := cli.BuildClient().BuildV1().Builds(cli.Namespace()).Get(context.Background(), "simple-example-1", metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				err = nil
			}
			return false, err
		}

		deploymentObj, err := cli.AdminKubeClient().AppsV1().Deployments(cli.Namespace()).Get(context.Background(), "simple-example", metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				err = nil
			}
			return false, err
		}

		// if the instantiation has settled, quit
		switch build.Status.Phase {
		case buildv1.BuildPhaseCancelled, buildv1.BuildPhaseError, buildv1.BuildPhaseFailed:
			return true, nil

		case buildv1.BuildPhaseComplete:
			var progressing, available *appsv1.DeploymentCondition
			for i, condition := range deploymentObj.Status.Conditions {
				switch condition.Type {
				case appsv1.DeploymentProgressing:
					progressing = &deploymentObj.Status.Conditions[i]

				case appsv1.DeploymentAvailable:
					available = &deploymentObj.Status.Conditions[i]
				}
			}

			if (progressing != nil &&
				progressing.Status == corev1.ConditionTrue &&
				progressing.Reason == deploymentutil.NewRSAvailableReason &&
				available != nil &&
				available.Status == corev1.ConditionTrue) ||
				(progressing != nil &&
					progressing.Status == corev1.ConditionFalse) {
				return true, nil
			}
		}

		// the build or dc have not settled; the templateinstance must also
		// indicate this

		if TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceReady, corev1.ConditionTrue) {
			return false, errors.New("templateinstance unexpectedly reported ready")
		}
		if TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
			return false, errors.New("templateinstance unexpectedly reported failure")
		}

		return false, nil
	}

	g.Context("", func() {
		g.BeforeEach(func() {
			// Tests that push to an ImageStreamTag need to wait for the internal registry hostname
			_, err := exutil.WaitForInternalRegistryHostname(cli)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = cli.Run("create").Args("-f", templatefixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			template, err = cli.TemplateClient().TemplateV1().Templates(cli.Namespace()).Get(context.Background(), "simple-example", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(cli)
				exutil.DumpConfigMapStates(cli)
				exutil.DumpPodLogsStartingWith("", cli)
			}
		})

		g.It("should report ready soon after all annotated objects are ready [apigroup:template.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			var err error

			templateinstance = &templatev1.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "templateinstance",
				},
				Spec: templatev1.TemplateInstanceSpec{
					Template: *template,
				},
			}

			g.By("instantiating the templateinstance")
			templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(context.Background(), templateinstance, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build and dc to settle")
			err = wait.Poll(time.Second, 10*time.Minute, waitSettle)
			if err != nil {
				err := dumpObjectReadiness(cli, templateinstance)
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error running dumpObjectReadiness: %v", err)
				}
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the templateinstance to indicate ready")
			// in principle, this should happen within 20 seconds
			err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
				templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				if TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
					return false, errors.New("templateinstance unexpectedly reported failure")
				}

				return TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceReady, corev1.ConditionTrue), nil
			})
			if err != nil {
				err := dumpObjectReadiness(cli, templateinstance)
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error running dumpObjectReadiness: %v", err)
				}
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should report failed soon after an annotated objects has failed [apigroup:template.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			var err error

			secret, err := cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Create(context.Background(), &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret",
				},
				Data: map[string][]byte{
					"SOURCE_REPOSITORY_URL": []byte("https://bad"),
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			templateinstance = &templatev1.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name: "templateinstance",
				},
				Spec: templatev1.TemplateInstanceSpec{
					Template: *template,
					Secret: &corev1.LocalObjectReference{
						Name: secret.Name,
					},
				},
			}

			g.By("instantiating the templateinstance")
			templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(context.Background(), templateinstance, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build and dc to settle")
			err = wait.Poll(time.Second, 10*time.Minute, waitSettle)
			if err != nil {
				err := dumpObjectReadiness(cli, templateinstance)
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error running dumpObjectReadiness: %v", err)
				}
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the templateinstance to indicate failed")
			// in principle, this should happen within 20 seconds
			err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
				templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(context.Background(), templateinstance.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				if TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceReady, corev1.ConditionTrue) {
					return false, errors.New("templateinstance unexpectedly reported ready")
				}

				return TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue), nil
			})
			if err != nil {
				err := dumpObjectReadiness(cli, templateinstance)
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error running dumpObjectReadiness: %v", err)
				}
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
