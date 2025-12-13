package builds

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kdeployutil "k8s.io/kubernetes/test/e2e/framework/deployment"
	admissionapi "k8s.io/pod-security-admission/api"

	deployutil "github.com/openshift/origin/test/extended/deployments"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	a58 = "a234567890123456789012345678901234567890123456789012345678"
	a59 = "a2345678901234567890123456789012345678901234567890123456789"
)

var _ = g.Describe("[sig-builds][Feature:Builds] oc new-app", func() {
	// Previously, the maximum length of app names creatable by new-app has
	// inadvertently been decreased, e.g. by creating an annotation somewhere
	// whose name itself includes the app name.  Ensure we can create and fully
	// deploy an app with a 58 character name [63 maximum - len('-9999' suffix)].

	oc := exutil.NewCLIWithPodSecurityLevel("new-app", admissionapi.LevelBaseline)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("waiting on the local namespace builder/default SAs")
			err := exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
			deployutil.DeploymentConfigFailureTrap(oc, a58, g.CurrentSpecReport().Failed())
			deployutil.DeploymentConfigFailureTrap(oc, a59, g.CurrentSpecReport().Failed())
		})

		g.It("should succeed with a --name of 58 characters [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			g.By("calling oc new-app")
			err := oc.Run("new-app").Args("registry.redhat.io/ubi8/nodejs-16:latest~https://github.com/sclorg/nodejs-ex", "--name", a58, "--build-env=BUILD_LOGLEVEL=5").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the build to complete")
			err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), a58+"-1", nil, nil, nil)
			if err != nil {
				exutil.DumpBuildLogs(a58, oc)
				exutil.DumpPodStates(oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			deploy, derr := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Get(context.Background(), a58, metav1.GetOptions{})
			if kapierrs.IsNotFound(derr) {
				// if deployment is not there we're working with old new-app producing deployment configs
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), a58, 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				// if present - wait for deployment
				o.Expect(derr).NotTo(o.HaveOccurred())
				err = kdeployutil.WaitForDeploymentComplete(oc.KubeClient(), deploy)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		g.It("should fail with a --name longer than 58 characters [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			g.By("calling oc new-app")
			out, err := oc.Run("new-app").Args("registry.redhat.io/ubi8/nodejs-16:latest~https://github.com/sclorg/nodejs-ex", "--name", a59).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("error: invalid name: "))
		})

		g.It("should succeed with an imagestream [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			// Bug 1767163 - oc new-app with --image-stream produced invalid labels
			g.By("calling oc new-app with imagestream")
			// Note: the imagestream used here does not matter (does not have to a valid builder) since we are not checking
			// the output results.  Since we can't rely on the samples operator being present to install sample s2i-enabled
			// imagestreams, just use one of the static imagestreams instead (cli:latest)
			out, err := oc.Run("new-app").Args("registry.redhat.io/ubi8/nodejs-16:latest~https://github.com/sclorg/nodejs-ex", "--image-stream=cli:latest").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).NotTo(o.ContainSubstring("error:"))
		})
	})
})
