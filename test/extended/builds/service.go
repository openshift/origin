package builds

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kdeployutil "k8s.io/kubernetes/test/e2e/framework/deployment"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-builds][Feature:Builds] build can reference a cluster service", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLIWithPodSecurityLevel("build-service", admissionapi.LevelBaseline)
		testDockerfile = fmt.Sprintf(`
FROM %s
RUN cat /etc/resolv.conf
RUN curl -vvv hello-nodejs:8080
`, image.ShellImage())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("with a build being created from new-build", func() {
			g.It("should be able to run a build that references a cluster service [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("standing up a new hello world nodejs service via oc new-app")
				err := oc.Run("new-app").Args("registry.redhat.io/ubi8/nodejs-16:latest~https://github.com/sclorg/nodejs-ex.git", "--name", "hello-nodejs").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "hello-nodejs-1", nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs("hello-nodejs", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				deploy, derr := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Get(context.Background(), "hello-nodejs", metav1.GetOptions{})
				if kapierrs.IsNotFound(derr) {
					// if deployment is not there we're working with old new-app producing deployment configs
					err := exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), "hello-nodejs", 1, true, oc)
					o.Expect(err).NotTo(o.HaveOccurred())
				} else {
					// if present - wait for deployment
					o.Expect(derr).NotTo(o.HaveOccurred())
					err := kdeployutil.WaitForDeploymentComplete(oc.KubeClient(), deploy)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "hello-nodejs")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("calling oc new-build with a Dockerfile")
				err = oc.Run("new-build").Args("-D", "-", "--to", "test:latest").InputString(testDockerfile).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build is in Complete phase")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "test-1", nil, nil, nil)
				//debug for failures
				if err != nil {
					exutil.DumpBuildLogs("test", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})
