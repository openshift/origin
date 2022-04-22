package cli

import (
	"os"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc set image", func() {
	defer g.GinkgoRecover()

	var (
		deploymentConfig = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "test-deployment-config.yaml")
		imageStream      = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "image-streams", "image-streams-centos7.json")
		oc               = exutil.NewCLI("oc-set-image")
	)

	g.It("can set images for pods and deployments", func() {
		g.By("creating test deployment, pod, and image stream")
		err := oc.Run("create").Args("-f", deploymentConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		file, err := writeObjectToFile(newHelloPod())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(file)

		err = oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("-f", imageStream).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for created resources to be ready for testing")
		err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
			err := oc.Run("get").Args("imagestreamtags", "ruby:2.7-ubi8").Execute()
			return err == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("testing --local flag validation")
		out, err := oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=ruby:2.7-ubi8", "--local").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("you must specify resources by --filename when --local is set."))

		g.By("testing --dry-run with -o flags")
		out, err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=ruby:2.7-ubi8", "--source=istag", "--dry-run").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-deployment-config"))
		o.Expect(out).To(o.ContainSubstring("deploymentconfig.apps.openshift.io/test-deployment-config image updated (dry run)"))

		out, err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=ruby:2.7-ubi8", "--source=istag", "--dry-run", "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("deploymentconfig.apps.openshift.io/test-deployment-config"))

		g.By("testing basic image updates")
		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=ruby:2.7-ubi8", "--source=istag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("dc/test-deployment-config", "-o", "jsonpath='{.spec.template.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("image-registry.openshift-image-registry.svc:5000/e2e-test-oc-set-image-"))
		o.Expect(out).To(o.ContainSubstring("/ruby@sha256:"))

		g.By("repeating basic image updates to ensure nothing changed")
		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=ruby:2.7-ubi8", "--source=istag").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("dc/test-deployment-config", "-o", "jsonpath='{.spec.template.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("image-registry.openshift-image-registry.svc:5000/e2e-test-oc-set-image-"))
		o.Expect(out).To(o.ContainSubstring("/ruby@sha256:"))

		g.By("testing invalid image tags")
		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=ruby:XYZ", "--source=istag").Execute()
		o.Expect(err).To(o.HaveOccurred())

		err = oc.Run("set").Args("image", "dc/test-deployment-config", "ruby-helloworld=ruby:XYZ", "--source=isimage").Execute()
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
		err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
			// during the set image, it's possible that the temporary deployer pod existed, but the moment we invoke the patches the pod is gone, b/c it completed its task, so we retry several times
			err := oc.Run("set").Args("image", "pods,dc", "*=ruby:2.7-ubi8", "--all", "--source=imagestreamtag").Execute()
			return err == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("pod/hello-openshift", "-o", "jsonpath='{.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("image-registry.openshift-image-registry.svc:5000/e2e-test-oc-set-image-"))
		o.Expect(out).To(o.ContainSubstring("/ruby@sha256:"))

		out, err = oc.Run("get").Args("dc/test-deployment-config", "-o", "jsonpath='{.spec.template.spec.containers[0].image}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("image-registry.openshift-image-registry.svc:5000/e2e-test-oc-set-image-"))
		o.Expect(out).To(o.ContainSubstring("/ruby@sha256:"))
	})
})
