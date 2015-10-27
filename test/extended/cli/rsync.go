package cli

import (
	"fmt"
	"io/ioutil"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("cli: parallel: oc rsync", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("cli-rsync", exutil.KubeConfigPath())
		templatePath = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		sourcePath1  = exutil.FixturePath("..", "..", "examples", "image-streams")
		sourcePath2  = exutil.FixturePath("..", "..", "examples", "sample-app")
		strategies   = []string{"rsync", "rsync-daemon", "tar"}
	)

	g.Describe("copy by strategy", func() {
		var podName string

		g.JustBeforeEach(func() {
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
			podName = pods.Items[0].Name
		})

		testRsyncFunc := func(strategy string) func() {
			return func() {
				g.By(fmt.Sprintf("Calling oc rsync %s %s:/tmp --strategy=%s", sourcePath1, podName, strategy))
				err := oc.Run("rsync").Args(
					sourcePath1,
					fmt.Sprintf("%s:/tmp", podName),
					fmt.Sprintf("--strategy=%s", strategy)).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Verifying that files are copied to the container")
				result, err := oc.Run("rsh").Args(podName, "ls", "/tmp/image-streams").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(result).To(o.ContainSubstring("image-streams-centos7.json"))

				g.By(fmt.Sprintf("Calling oc rsync %s/ %s:/tmp/image-streams --strategy=%s --delete", sourcePath2, podName, strategy))
				err = oc.Run("rsync").Args(
					sourcePath2+"/",
					fmt.Sprintf("%s:/tmp/image-streams", podName),
					fmt.Sprintf("--strategy=%s", strategy),
					"--delete").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Verifying that the expected files are in the container")
				result, err = oc.Run("rsh").Args(podName, "ls", "/tmp/image-streams").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(result).To(o.ContainSubstring("application-template-stibuild.json"))
				o.Expect(result).NotTo(o.ContainSubstring("image-streams-centos7.json"))

				g.By("Creating a local temporary directory")
				tempDir, err := ioutil.TempDir("", "rsync")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("Copying files from container to local directory: oc rsync %s:/tmp/image-streams/ %s --strategy=%s", podName, tempDir, strategy))
				err = oc.Run("rsync").Args(
					fmt.Sprintf("%s:/tmp/image-streams/", podName),
					tempDir,
					fmt.Sprintf("--strategy=%s", strategy)).Execute()

				g.By(fmt.Sprintf("Verifying that files were copied to the local directory"))
				files, err := ioutil.ReadDir(tempDir)
				o.Expect(err).NotTo(o.HaveOccurred())
				found := false
				for _, f := range files {
					if strings.Contains(f.Name(), "application-template-stibuild.json") {
						found = true
						break
					}
				}
				o.Expect(found).To(o.BeTrue())

				g.By("Getting an error if copying to a destination directory where there is no write permission")
				result, err = oc.Run("rsync").Args(
					sourcePath1,
					fmt.Sprintf("%s:/", podName),
					fmt.Sprintf("--strategy=%s", strategy)).Output()
				o.Expect(err).To(o.HaveOccurred())
			}
		}

		for _, strategy := range strategies {
			g.It(fmt.Sprintf("should copy files with the %s strategy", strategy), testRsyncFunc(strategy))
		}
	})
})
