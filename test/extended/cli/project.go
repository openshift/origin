package cli

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("[sig-cli] oc project", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-project").AsAdmin()

	g.It("--show-labels works for projects [apigroup:project.openshift.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("label").Args("namespace", oc.Namespace(), "foo=bar").Output()
		o.Expect(out).To(o.ContainSubstring("labeled"))
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("project", oc.Namespace(), "--show-labels").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("foo=bar"))
	})

	g.It("can switch between different projects [apigroup:authorization.openshift.io][apigroup:user.openshift.io][apigroup:project.openshift.io][Serial]", g.Label("Size:M"), func() {
		g.By("check auth usage is correct")
		_, err := oc.Run("policy", "who-can").Args("get", "pods", "-n", "missing-ns").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.Run("auth", "can-i").Args("get", "pods", "-n", "missing-ns").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.Run("auth", "can-i").Args("--list", "-n", "missing-ns").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		testProjectUser := "bob-" + oc.Namespace()

		_, err = oc.Run("create", "user").Args(testProjectUser).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			err = oc.Run("delete", "user").Args(testProjectUser).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		out, err := oc.Run("auth", "can-i").Args("get", "pods", "--as="+testProjectUser, "-n", "missing-ns").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("no"))

		_, err = oc.Run("auth", "can-i").Args("--list", "--as="+testProjectUser, "-n", "missing-ns").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("projects with invalid argument should fail")
		out, err = oc.Run("projects").Args("test_arg").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("no arguments"))

		testProject1 := oc.Namespace() + "-project1"
		testProject2 := oc.Namespace() + "-project2"
		testProject3 := oc.Namespace() + "-project3"

		g.By("new project should be created")
		out, err = oc.Run("new-project").Args(testProject1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("Now using project \"%s\" on server ", testProject1)))
		defer func() {
			err = oc.Run("delete", "namespace").Args(testProject1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
			out, err = oc.Run("projects").Output()
			if err != nil {
				return false, nil
			}

			return strings.Contains(out, fmt.Sprintf("Using project \"%s\" on server", testProject1)), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("new-project").Args(testProject2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("Now using project \"%s\" on server ", testProject2)))
		defer func() {
			err = oc.Run("delete", "namespace").Args(testProject2).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("HA masters means that you may have to wait for the lists to settle, so you allow for that by waiting")
		err = wait.PollImmediate(500*time.Millisecond, 2*time.Minute, func() (bool, error) {
			out, err = oc.Run("projects").Output()
			if err != nil {
				return false, nil
			}

			return strings.Contains(out, "You have access to the following projects and can switch between them with ") &&
				strings.Contains(out, testProject1) &&
				strings.Contains(out, testProject2), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("new-project").Args(testProject3, "--skip-config-write").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("To switch to this project and start adding applications, use"))
		defer func() {
			err = oc.Run("delete", "namespace").Args(testProject3).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		out, err = oc.Run("config", "view").Args("-o", `jsonpath="{.contexts[*].context.namespace}"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring(testProject3))

		err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
			out, err = oc.Run("projects").Output()
			if err != nil {
				return false, nil
			}

			return strings.Contains(out, testProject3), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("project").Args(testProject3).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("Now using project \"%s\"", testProject3)))

		out, err = oc.Run("config", "view").Args("-o", `jsonpath="{.contexts[*].context.namespace}"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(testProject3))
	})
})
