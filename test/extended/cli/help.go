package cli

import (
	"os/exec"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc help", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("oc-help")

	g.It("works as expected", g.Label("Size:S"), func() {
		err := exec.Command("kubectl").Run()
		o.Expect(err).NotTo(o.HaveOccurred())

		stdout, err := exec.Command("oc").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(stdout)).To(o.ContainSubstring("OpenShift Client"))

		// help for root commands must be consistent
		stdout, err = exec.Command("oc", "-h").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out := string(stdout)
		o.Expect(out).To(o.ContainSubstring("Build and Deploy Commands:"))
		o.Expect(out).To(o.ContainSubstring("Other Commands:"))
		o.Expect(out).NotTo(o.ContainSubstring("Options"))
		o.Expect(out).NotTo(o.ContainSubstring("Global Options"))

		stdout, err = exec.Command("oc", "--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(stdout)).To(o.ContainSubstring("OpenShift Client"))

		err = oc.Run("ex").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("policy").Args("-h").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("add-role-to-user"))
		o.Expect(out).NotTo(o.ContainSubstring("Other Commands:"))

		out, err = oc.Run("exec").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`\-\- COMMAND \[args\.\.\.\]`))

		out, err = oc.Run("rsh").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`COMMAND \[args\.\.\.\]`))

		// help for root commands with --help flag must be consistent
		out, err = oc.Run("login").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Options"))
		o.Expect(out).NotTo(o.ContainSubstring("Global Options"))
		o.Expect(out).To(o.ContainSubstring("insecure-skip-tls-verify"))

		// help for given command with --help flag must be consistent
		out, err = oc.Run("get").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Display one or many resources"))
		o.Expect(out).To(o.ContainSubstring("oc"))

		out, err = oc.Run("project").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Switch to another project"))

		out, err = oc.Run("projects").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("existing projects"))

		// help for given command through help command must be consistent
		out, err = oc.Run("help", "get").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Display one or many resources"))

		out, err = oc.Run("help", "project").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Switch to another project"))

		out, err = oc.Run("help", "projects").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("current active project and existing projects on the server"))

		// help tips must be consistent
		stdout, err = exec.Command("oc", "--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out = string(stdout)
		o.Expect(out).To(o.ContainSubstring(`Use "oc <command> --help" for more information`))
		o.Expect(out).To(o.ContainSubstring(`Use "oc options" for a list of global`))

		out, err = oc.Run("help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`Use "oc <command> --help" for more information`))
		o.Expect(out).To(o.ContainSubstring(`Use "oc options" for a list of global`))

		out, err = oc.Run("set").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`Use "oc options" for a list of global`))

		out, err = oc.Run("set", "env").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`Use "oc options" for a list of global`))

		// runnable commands with required flags must error consistently
		out, err = oc.Run("get").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Required resource not specified"))

		// commands that expect file paths must validate and error out correctly
		out, err = oc.Run("login").Args("--certificate-authority=/path/to/invalid").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("no such file or directory"))

		// make sure that typoed commands come back with non-zero return codes
		err = oc.Run("policy").Args("TYPO").Execute()
		o.Expect(err).To(o.HaveOccurred())
		err = oc.Run("secrets").Args("TYPO").Execute()
		o.Expect(err).To(o.HaveOccurred())

		// make sure that LDAP group sync and prune exists
		out, err = oc.Run("adm", "groups", "sync").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("external provider"))

		out, err = oc.Run("adm", "groups", "prune").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("external provider"))

		out, err = oc.Run("adm", "prune", "groups").Args("--help").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("external provider"))
	})
})
