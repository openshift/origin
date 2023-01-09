package cli

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] templates", func() {
	defer g.GinkgoRecover()

	var (
		oc              = exutil.NewCLI("oc-templates")
		appTemplatePath = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "application-template-dockerbuild.json")
	)

	g.It("different namespaces [apigroup:user.openshift.io][apigroup:project.openshift.io][apigroup:template.openshift.io][apigroup:authorization.openshift.io][Skipped:Disconnected]", func() {
		bob := oc.CreateUser("bob-")

		err := oc.Run("create").Args("-f", appTemplatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("policy").Args("add-role-to-user", "admin", bob.Name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.ChangeUser(bob.Name)

		testProject2 := oc.Namespace() + "-project2"
		out, err := oc.Run("new-project").Args(testProject2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("Now using project \"%s\" on server ", testProject2)))
		defer func() {
			err = oc.WithoutNamespace().Run("delete", "project").Args(testProject2).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
			return oc.WithoutNamespace().Run("get").Args("templates").Execute() == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.WithoutNamespace().Run("create").Args("-f", appTemplatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.WithoutNamespace().Run("process").Args("template/ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("process").Args("templates/ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("process").Args(oc.Namespace() + "//ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.WithoutNamespace().Run("process").Args(oc.Namespace() + "/template/ruby-helloworld-sample").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		outputYamlFile, err := oc.WithoutNamespace().Run("get").Args("template", "ruby-helloworld-sample", "-o", "yaml").OutputToFile("template.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.WithoutNamespace().Run("process").Args("-f", outputYamlFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
