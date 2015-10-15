package cli

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("cli: parallel: rsync", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("cli-rsync", exutil.KubeConfigPath())
		templatePath = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		sourcePath1  = exutil.FixturePath("..", "..", "examples", "image-streams")
		sourcePath2  = exutil.FixturePath("..", "..", "examples", "sample-app")
	)

	g.Describe("oc rsync", func() {
		g.It("should copy files with rsync and tar to running container", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc new-app -f %q", templatePath))
			err := oc.Run("new-app").Args("-f", templatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the jenkins service get endpoints")
			err = oc.KubeFramework().WaitForAnEndpoint("jenkins")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Getting the jenkins pod name")
			selector, _ := labels.Parse("name=jenkins")
			pods, err := oc.KubeREST().Pods(oc.Namespace()).List(selector, fields.Everything())
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(pods.Items)).ToNot(o.BeZero())
			podName := pods.Items[0].Name

			g.By(fmt.Sprintf("calling oc rsync %s %s:/tmp", sourcePath1, podName))
			err = oc.Run("rsync").Args(sourcePath1, fmt.Sprintf("%s:/tmp", podName)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying that files are copied to the container")
			result, err := oc.Run("rsh").Args(podName, "ls", "/tmp/image-streams").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).To(o.ContainSubstring("image-streams-centos7.json"))

			g.By(fmt.Sprintf("calling oc rsync --use-tar --delete %s/ %s:/tmp/image-streams", sourcePath2, podName))
			err = oc.Run("rsync").Args("--use-tar", "--delete", sourcePath2+"/", fmt.Sprintf("%s:/tmp/image-streams", podName)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying that the expected files are in the container")
			result, err = oc.Run("rsh").Args(podName, "ls", "/tmp/image-streams").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).To(o.ContainSubstring("application-template-stibuild.json"))
			o.Expect(result).NotTo(o.ContainSubstring("image-streams-centos7.json"))

			g.By("Getting an error if copying to a destination directory where there is no write permission")
			result, err = oc.Run("rsync").Args(sourcePath1, fmt.Sprintf("%s:/", podName)).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(result).To(o.ContainSubstring("Permission denied"))
		})
	})
})
