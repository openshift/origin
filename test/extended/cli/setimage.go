package cli

import (
	"os"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const openshiftCLIImageStreamTag = "openshift/cli:latest"

// trimmedGetJSONPath runs oc get and returns stdout only (stderr is not merged), with jsonpath
// quote wrappers removed. Client warnings must not be mixed into assertions.
func trimmedGetJSONPath(oc *exutil.CLI, args ...string) (string, error) {
	stdout, _, err := oc.Run("get").Args(args...).Outputs()
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(stdout), `'"`), nil
}

func expectResolvedCLIImage(image, cliDigest string) {
	o.Expect(image).To(o.ContainSubstring("@sha256:" + cliDigest))
}

var _ = g.Describe("[sig-cli] oc set image", func() {
	defer g.GinkgoRecover()

	var (
		deploymentConfig = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "test-deployment-config.yaml")
		deployment       = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "test-deployment.yaml")
		imageStream      = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "image-streams", "image-streams-centos7.json")
		oc               = exutil.NewCLIWithPodSecurityLevel("oc-set-image", admissionapi.LevelBaseline)
	)

	g.It("can set images for pods and deployments [apigroup:image.openshift.io][apigroup:apps.openshift.io]", func() {
		payloadCLI, err := exutil.SearchLatestImage(oc, "cli")
		o.Expect(err).NotTo(o.HaveOccurred())
		cliDigest := ""
		if idx := strings.LastIndex(payloadCLI, "@sha256:"); idx >= 0 {
			cliDigest = payloadCLI[idx+len("@sha256:"):]
		}
		o.Expect(cliDigest).NotTo(o.BeEmpty())

		g.By("creating test deployment, pod, and image stream")
		err = oc.Run("create").Args("-f", deploymentConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		file, err := writeObjectToFile(newHelloPod())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(file)

		err = oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", imageStream).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("testing --local flag validation")
		out, err := oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld="+openshiftCLIImageStreamTag, "--local").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("you must specify resources by --filename when --local is set."))

		g.By("testing --dry-run=client with -o flags")
		out, err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag", "--dry-run=client").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-deployment-config"))
		o.Expect(out).To(o.ContainSubstring("deploymentconfig.apps.openshift.io/test-deployment-config image updated (dry run)"))

		out, err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag", "--dry-run=client", "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("deploymentconfig.apps.openshift.io/test-deployment-config"))

		g.By("testing basic image updates")
		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		image, err := trimmedGetJSONPath(oc, "dc/test-deployment-config", "-o", "jsonpath={.spec.template.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)

		g.By("repeating basic image updates to ensure nothing changed")
		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		image, err = trimmedGetJSONPath(oc, "dc/test-deployment-config", "-o", "jsonpath={.spec.template.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)

		g.By("testing invalid image tags")
		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=openshift/cli:notarealtagfortest", "--source=istag").Execute()
		o.Expect(err).To(o.HaveOccurred())

		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=openshift/cli:notarealtagfortest", "--source=isimage").Execute()
		o.Expect(err).To(o.HaveOccurred())

		g.By("setting a different, valid image on deployment config")
		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=nginx").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err = oc.Run("get").Args("dc/test-deployment-config", "-o", "jsonpath='{.spec.template.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("nginx"))

		g.By("setting a different, valid image on pod")
		err = oc.Run("set").Args("image", "pod/hello-openshift", "hello-openshift=nginx").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("pod/hello-openshift", "-o", "jsonpath='{.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("nginx"))

		g.By("setting a different, valid image tag on pod")
		err = oc.Run("set").Args("image", "pod/hello-openshift", "hello-openshift=nginx:1.9.1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("pod/hello-openshift", "-o", "jsonpath='{.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("nginx:1.9.1"))

		g.By("setting a different, valid image on multiple resources")
		err = oc.Run("set").Args("image", "pods,dc", "*="+openshiftCLIImageStreamTag, "--all", "--source=imagestreamtag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		image, err = trimmedGetJSONPath(oc, "pod/hello-openshift", "-o", "jsonpath={.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)

		image, err = trimmedGetJSONPath(oc, "dc/test-deployment-config", "-o", "jsonpath={.spec.template.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)
	})

	g.It("can set images for pods and deployments [apigroup:image.openshift.io]", func() {
		payloadCLI, err := exutil.SearchLatestImage(oc, "cli")
		o.Expect(err).NotTo(o.HaveOccurred())
		cliDigest := ""
		if idx := strings.LastIndex(payloadCLI, "@sha256:"); idx >= 0 {
			cliDigest = payloadCLI[idx+len("@sha256:"):]
		}
		o.Expect(cliDigest).NotTo(o.BeEmpty())

		g.By("creating test deployment, pod, and image stream")
		err = oc.Run("create").Args("-f", deployment).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		file, err := writeObjectToFile(newHelloPod())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(file)

		err = oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", imageStream).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("testing --local flag validation")
		out, err := oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld="+openshiftCLIImageStreamTag, "--local").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("you must specify resources by --filename when --local is set."))

		g.By("testing --dry-run=client with -o flags")
		out, err = oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag", "--dry-run=client").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-deployment"))
		o.Expect(out).To(o.ContainSubstring("deployment.apps/test-deployment image updated (dry run)"))

		out, err = oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag", "--dry-run=client", "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("deployment.apps/test-deployment"))

		g.By("testing basic image updates")
		err = oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		image, err := trimmedGetJSONPath(oc, "deployment/test-deployment", "-o", "jsonpath={.spec.template.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)

		g.By("repeating basic image updates to ensure nothing changed")
		err = oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld="+openshiftCLIImageStreamTag, "--source=istag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		image, err = trimmedGetJSONPath(oc, "deployment/test-deployment", "-o", "jsonpath={.spec.template.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)

		g.By("testing invalid image tags")
		err = oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld=openshift/cli:notarealtagfortest", "--source=istag").Execute()
		o.Expect(err).To(o.HaveOccurred())

		err = oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld=openshift/cli:notarealtagfortest", "--source=isimage").Execute()
		o.Expect(err).To(o.HaveOccurred())

		g.By("setting a different, valid image on deployment")
		err = oc.Run("set").Args("image", "deployment/test-deployment", "ruby-helloworld=nginx").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err = oc.Run("get").Args("deployment/test-deployment", "-o", "jsonpath='{.spec.template.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("nginx"))

		g.By("setting a different, valid image on pod")
		err = oc.Run("set").Args("image", "pod/hello-openshift", "hello-openshift=nginx").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("pod/hello-openshift", "-o", "jsonpath='{.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("nginx"))

		g.By("setting a different, valid image tag on pod")
		err = oc.Run("set").Args("image", "pod/hello-openshift", "hello-openshift=nginx:1.9.1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("pod/hello-openshift", "-o", "jsonpath='{.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("nginx:1.9.1"))

		g.By("setting a different, valid image on multiple resources")
		err = oc.Run("set").Args("image", "pods,deployments", "*="+openshiftCLIImageStreamTag, "--all", "--source=imagestreamtag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		image, err = trimmedGetJSONPath(oc, "pod/hello-openshift", "-o", "jsonpath={.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)

		image, err = trimmedGetJSONPath(oc, "deployment/test-deployment", "-o", "jsonpath={.spec.template.spec.containers[0].image}")
		o.Expect(err).NotTo(o.HaveOccurred())
		expectResolvedCLIImage(image, cliDigest)
	})
})
