package cli

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
	exutilimage "github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-cli] oc status", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-status")

	g.It("returns expected help messages [apigroup:project.openshift.io][apigroup:build.openshift.io][apigroup:image.openshift.io][apigroup:route.openshift.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("status").Args("-h").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("oc describe buildconfig"))

		out, err = oc.Run("status").Args("--suggest").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("oc new-app"))
	})

	g.It("can show correct status after switching between projects [apigroup:project.openshift.io][apigroup:image.openshift.io][Serial]", g.Label("Size:M"), func() {
		projectBar := oc.Namespace() + "-project-bar"
		projectBar2 := oc.Namespace() + "-project-bar-2"
		projectStatus := oc.Namespace() + "-project-status"

		out, err := oc.Run("status").Args("--all-namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Showing all projects on server"))

		out, err = oc.Run("status").Args("-A").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Showing all projects on server"))

		g.By("create a new project")
		err = oc.WithoutNamespace().Run("new-project").Args(projectBar, "--display-name=my project", "--description=test project").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err = oc.WithoutNamespace().Run("project").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("Using project \"%s\"", projectBar)))

		g.By("make sure `oc status` does not use \"no projects\" message if there is a project created")
		out, err = oc.WithoutNamespace().Run("status").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("In project my project (%s) on server", projectBar)))

		g.By("create a second project")
		err = oc.WithoutNamespace().Run("new-project").Args(projectBar2, "--display-name=my project 2", "--description=test project 2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err = oc.WithoutNamespace().Run("project").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("Using project \"%s\"", projectBar2)))

		g.By(fmt.Sprintf("delete the current project `%s` and make sure `oc status` does not return the \"no projects\" message since `%s` still exists", projectBar2, projectBar))
		out, err = oc.WithoutNamespace().Run("delete").Args("project", projectBar2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal(fmt.Sprintf("project.project.openshift.io \"%s\" deleted", projectBar2)))

		err = oc.WithoutNamespace().Run("project").Args(projectBar).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("delete").Args("project", projectBar).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.PollImmediate(time.Second, 30*time.Second, func() (done bool, err error) {
			out, err = oc.Run("get").Args("projects").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(out, projectBar) || strings.Contains(out, projectBar2) {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.WithoutNamespace().Run("new-project").Args(projectStatus, "--display-name=my project", "--description=test project").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.WithoutNamespace().Run("delete").Args("project", projectStatus).Execute()
		}()

		g.By("verify jobs are showing in status")
		image := exutilimage.ShellImage()
		err = oc.Run("create").Args("job", "pi", "--image", image, "--namespace", projectStatus).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("status", "--namespace", projectStatus).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("job/pi manages %s", image)))
	})
})
