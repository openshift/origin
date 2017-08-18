package main_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var testBinary string

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	testBinary, err = gexec.Build("code.cloudfoundry.org/lager/lagerflags/integration", "-race")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = Describe("CF-Lager", func() {
	It("provides flags", func() {
		session, err := gexec.Start(exec.Command(testBinary, "--help"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		session.Wait()
		Expect(session.Err.Contents()).To(ContainSubstring("-logLevel"))
	})

	It("pipes output to stdout", func() {
		session, err := gexec.Start(exec.Command(testBinary), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		session.Wait()

		Expect(session.Out.Contents()).To(ContainSubstring("info"))
	})

	It("defaults to the info log level", func() {
		session, err := gexec.Start(exec.Command(testBinary), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		session.Wait()

		Expect(session.Out.Contents()).NotTo(ContainSubstring("debug"))
		Expect(session.Out.Contents()).To(ContainSubstring("info"))
		Expect(session.Out.Contents()).To(ContainSubstring("error"))
		Expect(session.Out.Contents()).To(ContainSubstring("fatal"))
	})

	It("honors the passed-in log level", func() {
		session, err := gexec.Start(exec.Command(testBinary, "-logLevel=debug"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		session.Wait()

		Expect(session.Out.Contents()).To(ContainSubstring("debug"))
		Expect(session.Out.Contents()).To(ContainSubstring("info"))
		Expect(session.Out.Contents()).To(ContainSubstring("error"))
		Expect(session.Out.Contents()).To(ContainSubstring("fatal"))
	})
})
