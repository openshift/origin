package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[cli][Slow] can use rsync to upload files to pods", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("cli-rsync", exutil.KubeConfigPath())
		templatePath = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		sourcePath1  = exutil.FixturePath("..", "..", "examples", "image-streams")
		sourcePath2  = exutil.FixturePath("..", "..", "examples", "sample-app")
		strategies   = []string{"rsync", "rsync-daemon", "tar"}
	)

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
		pods, err := oc.KubeREST().Pods(oc.Namespace()).List(kapi.ListOptions{LabelSelector: selector})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).ToNot(o.BeZero())
		podName = pods.Items[0].Name
	})

	g.Describe("copy by strategy", func() {

		testRsyncFn := func(strategy string) func() {
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

				g.By(fmt.Sprintf("Copying files from container to local directory with --delete: oc rsync %s:/tmp/image-streams/ %s --strategy=%s", podName, tempDir, strategy))
				originalName := "application-template-stibuild.json"
				modifiedName := "application-template-stirenamed.json"
				err = os.Rename(filepath.Join(tempDir, originalName), filepath.Join(tempDir, modifiedName))
				o.Expect(err).NotTo(o.HaveOccurred())

				err = oc.Run("rsync").Args(
					fmt.Sprintf("%s:/tmp/image-streams/", podName),
					tempDir,
					"--delete",
					fmt.Sprintf("--strategy=%s", strategy)).Execute()
				g.By(fmt.Sprintf("Verifying that the expected files are in the local directory"))
				o.Expect(err).NotTo(o.HaveOccurred())
				// After the copy with --delete, the file with 'modifiedName' should have been deleted
				// and the file with 'originalName' should have been restored.
				foundOriginal := false
				foundModified := false
				files, err = ioutil.ReadDir(tempDir)
				for _, f := range files {
					if strings.Contains(f.Name(), originalName) {
						foundOriginal = true
					}
					if strings.Contains(f.Name(), modifiedName) {
						foundModified = true
					}
				}
				g.By("Verifying original file is in the local directory")
				o.Expect(foundOriginal).To(o.BeTrue())

				g.By("Verifying renamed file is not in the local directory")
				o.Expect(foundModified).To(o.BeFalse())

				g.By("Getting an error if copying to a destination directory where there is no write permission")
				result, err = oc.Run("rsync").Args(
					sourcePath1,
					fmt.Sprintf("%s:/", podName),
					fmt.Sprintf("--strategy=%s", strategy)).Output()
				o.Expect(err).To(o.HaveOccurred())
			}
		}

		for _, strategy := range strategies {
			g.It(fmt.Sprintf("should copy files with the %s strategy", strategy), testRsyncFn(strategy))
		}
	})

	g.Describe("rsync specific flags", func() {

		g.It("should honor the --exclude flag", func() {
			g.By(fmt.Sprintf("Calling oc rsync %s %s:/tmp --exclude=image-streams-rhel7.json", sourcePath1, podName))
			err := oc.Run("rsync").Args(
				sourcePath1,
				fmt.Sprintf("%s:/tmp", podName),
				"--exclude=image-streams-rhel7.json").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying that files are copied to the container")
			result, err := oc.Run("rsh").Args(podName, "ls", "/tmp/image-streams").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).To(o.ContainSubstring("image-streams-centos7.json"))
			o.Expect(result).NotTo(o.ContainSubstring("image-streams-rhel7.json"))
		})

		g.It("should honor the --include flag", func() {
			g.By(fmt.Sprintf("Calling oc rsync %s %s:/tmp --exclude=*.json --include=image-streams-rhel7.json", sourcePath1, podName))
			err := oc.Run("rsync").Args(
				sourcePath1,
				fmt.Sprintf("%s:/tmp", podName),
				"--exclude=*.json",
				"--include=image-streams-rhel7.json").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying that files are copied to the container")
			result, err := oc.Run("rsh").Args(podName, "ls", "/tmp/image-streams").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).To(o.ContainSubstring("image-streams-rhel7.json"))
			o.Expect(result).NotTo(o.ContainSubstring("image-streams-centos7.json"))
		})

		g.It("should honor the --progress flag", func() {
			g.By(fmt.Sprintf("Calling oc rsync %s %s:/tmp --progress", sourcePath1, podName))
			result, err := oc.Run("rsync").Args(
				sourcePath1,
				fmt.Sprintf("%s:/tmp", podName),
				"--progress").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result).To(o.ContainSubstring("100%"))
		})

		g.It("should honor the --no-perms flag", func() {
			g.By("Creating a temporary destination directory")
			tempDir, err := ioutil.TempDir("", "rsync")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Copying the jenkins directory from the pod to the temp directory: oc rsync %s:/var/lib/jenkins %s", podName, tempDir))
			err = oc.Run("rsync").Args(
				fmt.Sprintf("%s:/var/lib/jenkins", podName),
				tempDir).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			localJenkinsDir := filepath.Join(tempDir, "jenkins")
			g.By("By changing the permissions on the local jenkins directory")
			err = os.Chmod(localJenkinsDir, 0700)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Copying the local jenkins directory to the pod with no flags: oc rsync %s/ %s:/var/lib/jenkins", localJenkinsDir, podName))
			err = oc.Run("rsync").Args(
				fmt.Sprintf("%s/", localJenkinsDir),
				fmt.Sprintf("%s:/var/lib/jenkins", podName)).Execute()
			// An error should occur trying to set permissions on the directory
			o.Expect(err).To(o.HaveOccurred())

			g.By(fmt.Sprintf("Copying the local jenkins directory to the pod with: oc rsync %s/ %s:/var/lib/jenkins --no-perms", localJenkinsDir, podName))
			err = oc.Run("rsync").Args(
				fmt.Sprintf("%s/", localJenkinsDir),
				fmt.Sprintf("%s:/var/lib/jenkins", podName),
				"--no-perms").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

})
