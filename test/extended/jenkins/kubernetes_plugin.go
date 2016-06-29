package jenkins

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/pkg/util/wait"
)

var _ = g.Describe("[jenkins] schedule jobs on pod slaves", func() {
	defer g.GinkgoRecover()

	var (
		jenkinsExampleDir           = filepath.Join("examples", "jenkins-master")
		jenkinsMasterTemplate       = filepath.Join(jenkinsExampleDir, "jenkins-master-template.json")
		jenkinsSlaveBuilderTemplate = filepath.Join(jenkinsExampleDir, "jenkins-slave-template.json")

		oc = exutil.NewCLI("jenkins-kube", exutil.KubeConfigPath())
	)

	var waitForBuildComplete = func(name string) (bool, error) {
		out, err := oc.Run("get").Args("build", name, "-o", "template", "--template", "{{ .status.phase }}").Output()
		if err != nil {
			return false, nil
		}
		return strings.Contains(out, "Complete"), nil
	}

	g.Describe("use of jenkins with kubernetes plugin", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It("by creating slave from existing builder and adding it to Jenkins master", func() {

			g.By("create the jenkins slave builder template")
			err := oc.Run("create").Args("-f", jenkinsSlaveBuilderTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create the jenkins master template")
			err = oc.Run("create").Args("-f", jenkinsMasterTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("build the Jenkins slave for ruby-22-centos7")
			err = oc.Run("new-app").Args("--template", "jenkins-slave-builder").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for the slave to be built")
			err = wait.Poll(1*time.Second, 5*time.Minute, func() (bool, error) {
				return waitForBuildComplete("ruby-22-centos7-slave-1")
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("grant service account in jenkins container access to API")
			err = oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+oc.Namespace()+":default", "-n", oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("build the Jenkins master")
			err = oc.Run("new-app").Args("--template", "jenkins-master").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for the master to be built")
			err = wait.Poll(1*time.Second, 5*time.Minute, func() (bool, error) {
				return waitForBuildComplete("jenkins-master-1")
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for jenkins deployment")
			err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "jenkins", oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("get ip and port for jenkins service")
			serviceIP, err := oc.Run("get").Args("svc", "jenkins", "--config",
				exutil.KubeConfigPath()).Template("{{.spec.clusterIP}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(serviceIP).NotTo(o.BeEmpty())
			port, err := oc.Run("get").Args("svc", "jenkins", "--config",
				exutil.KubeConfigPath()).Template("{{ $x := index .spec.ports 0}}{{$x.port}}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(port).NotTo(o.BeEmpty())

			jenkinsUri := fmt.Sprintf("http://%s:%s", serviceIP, port)
			g.By(fmt.Sprintf("wait for jenkins to come up at %q", jenkinsUri))
			err = waitForJenkinsActivity(jenkinsUri, "", 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("inspecting the Jenkins master logs the slave image should be registered")
			out, err := oc.Run("logs").Args("dc/jenkins").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Adding image ruby-22-centos7-jenkins-slave:latest as Kubernetes slave"))

			g.By("kick the ruby-hello-world-test job")
			immediateInteractionWithJenkins(fmt.Sprintf("%s/job/ruby-hello-world-test/build?delay=0sec", jenkinsUri), "POST", nil, 201)
			verifyPodProvisioned := func() (bool, error) {
				out, err := oc.Run("logs").Args("dc/jenkins").Output()
				if err != nil {
					return false, err
				}
				return strings.Contains(out, "Kubernetes Pod Template provisioning successfully completed"), nil
			}

			err = wait.Poll(2*time.Second, 5*time.Minute, verifyPodProvisioned)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
