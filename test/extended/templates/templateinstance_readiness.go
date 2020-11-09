package templates

import (
	"errors"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	templatev1 "github.com/openshift/api/template/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	templatecontroller "github.com/openshift/origin/pkg/template/controller"
	exutil "github.com/openshift/origin/test/extended/util"
)

// ensure that template instantiation waits for annotated objects
var _ = g.Describe("[Conformance][templates] templateinstance readiness test", func() {
	defer g.GinkgoRecover()

	var (
		cli              = exutil.NewCLI("templates", exutil.KubeConfigPath())
		template         *templatev1.Template
		templateinstance *templatev1.TemplateInstance
		templatefixture  = exutil.FixturePath("..", "..", "examples", "quickstarts", "cakephp-mysql.json")
	)

	waitSettle := func() (bool, error) {
		var err error

		// must read the templateinstance before the build/dc
		templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		build, err := cli.BuildClient().Build().Builds(cli.Namespace()).Get("cakephp-mysql-example-1", metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				err = nil
			}
			return false, err
		}

		dc, err := cli.AppsClient().AppsV1().DeploymentConfigs(cli.Namespace()).Get("cakephp-mysql-example", metav1.GetOptions{})
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
			for i, condition := range dc.Status.Conditions {
				switch condition.Type {
				case appsv1.DeploymentProgressing:
					progressing = &dc.Status.Conditions[i]

				case appsv1.DeploymentAvailable:
					available = &dc.Status.Conditions[i]
				}
			}

			if (progressing != nil &&
				progressing.Status == corev1.ConditionTrue &&
				progressing.Reason == appsutil.NewRcAvailableReason &&
				available != nil &&
				available.Status == corev1.ConditionTrue) ||
				(progressing != nil &&
					progressing.Status == corev1.ConditionFalse) {
				return true, nil
			}
		}

		// the build or dc have not settled; the templateinstance must also
		// indicate this

		if templatecontroller.TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceReady, corev1.ConditionTrue) {
			return false, errors.New("templateinstance unexpectedly reported ready")
		}
		if templatecontroller.TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
			return false, errors.New("templateinstance unexpectedly reported failure")
		}

		return false, nil
	}

	g.Context("", func() {
		g.BeforeEach(func() {
			g.By("waiting for default service account")
			err := exutil.WaitForServiceAccount(cli.KubeClient().Core().ServiceAccounts(cli.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for builder service account")
			err = exutil.WaitForServiceAccount(cli.KubeClient().Core().ServiceAccounts(cli.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())

			err = cli.Run("create").Args("-f", templatefixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			template, err = cli.TemplateClient().TemplateV1().Templates(cli.Namespace()).Get("cakephp-mysql-example", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(cli)
				exutil.DumpPodLogsStartingWith("", cli)
			}
		})

		g.It("should report ready soon after all annotated objects are ready", func() {
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
			templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(templateinstance)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build and dc to settle")
			err = wait.Poll(time.Second, 20*time.Minute, waitSettle)
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
				templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				if templatecontroller.TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
					return false, errors.New("templateinstance unexpectedly reported failure")
				}

				return templatecontroller.TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceReady, corev1.ConditionTrue), nil
			})
			if err != nil {
				err := dumpObjectReadiness(cli, templateinstance)
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error running dumpObjectReadiness: %v", err)
				}
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should report failed soon after an annotated objects has failed", func() {
			var err error

			secret, err := cli.KubeClient().Core().Secrets(cli.Namespace()).Create(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret",
				},
				Data: map[string][]byte{
					"SOURCE_REPOSITORY_URL": []byte("https://bad"),
				},
			})
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
			templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(templateinstance)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build and dc to settle")
			err = wait.Poll(time.Second, 20*time.Minute, waitSettle)
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
				templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				if templatecontroller.TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceReady, corev1.ConditionTrue) {
					return false, errors.New("templateinstance unexpectedly reported ready")
				}

				return templatecontroller.TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue), nil
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
