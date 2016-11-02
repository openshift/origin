package image_ecosystem

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/pkg/util/wait"
)

// patchTemplate finds BuildConfigs in a template, changes their source type to Binary, and removes all triggers
func patchTemplate(filename string, outDir string) string {
	inputJson, err := ioutil.ReadFile(filename)
	o.Expect(err).ToNot(o.HaveOccurred())

	var template map[string]interface{}
	err = json.Unmarshal(inputJson, &template)
	o.Expect(err).ToNot(o.HaveOccurred())

	for _, obj := range template["objects"].([]interface{}) {
		bc := obj.(map[string]interface{})
		if kind := bc["kind"].(string); kind != "BuildConfig" {
			continue
		}
		spec := bc["spec"].(map[string]interface{})
		spec["triggers"] = []interface{}{}

		source := spec["source"].(map[string]interface{})
		source["type"] = "Binary"
		delete(source, "git")
		delete(source, "contextDir")
	}

	outputJson, err := json.MarshalIndent(template, "", "  ")
	o.Expect(err).ToNot(o.HaveOccurred())

	basename := filepath.Base(filename)
	outputFile := filepath.Join(outDir, basename)
	err = ioutil.WriteFile(outputFile, outputJson, 0644)
	o.Expect(err).ToNot(o.HaveOccurred())

	return outputFile
}

var _ = g.Describe("[image_ecosystem][jenkins] schedule jobs on pod slaves", func() {
	defer g.GinkgoRecover()

	var (
		jenkinsExampleDir = filepath.Join("examples", "jenkins", "master-slave")
		oc                = exutil.NewCLI("jenkins-kube", exutil.KubeConfigPath())
	)

	var (
		jenkinsMasterTemplate       string
		jenkinsSlaveBuilderTemplate string
		jsonTempDir                 string
	)

	g.Describe("use of jenkins with kubernetes plugin", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.BeforeEach(func() {
			var err error
			jsonTempDir, err = ioutil.TempDir(exutil.TestContext.OutputDir, "jenkins-kubernetes-")
			o.Expect(err).NotTo(o.HaveOccurred())

			// We need to prepare the templates first in order to use binary builds:
			// 1. remove BuildConfig triggers to not start build immediately after instantiating template,
			// 2. remove contextDir so that we can send just that directory as a binary, not whole repo.
			jenkinsMasterTemplate = patchTemplate(filepath.Join(jenkinsExampleDir, "jenkins-master-template.json"), jsonTempDir)
			jenkinsSlaveBuilderTemplate = patchTemplate(filepath.Join(jenkinsExampleDir, "jenkins-slave-template.json"), jsonTempDir)
		})

		g.AfterEach(func() {
			if len(jsonTempDir) > 0 {
				os.RemoveAll(jsonTempDir)
			}
		})

		g.It("by creating slave from existing builder and adding it to Jenkins master", func() {

			g.By("create the jenkins slave builder template")
			err := oc.Run("create").Args("-f", jenkinsSlaveBuilderTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create the jenkins master template")
			err = oc.Run("create").Args("-f", jenkinsMasterTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("instantiate the slave template")
			err = oc.Run("new-app").Args("--template", "jenkins-slave-builder").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("build the Jenkins slave for ruby-22-centos7")
			br, err := exutil.StartBuildAndWait(oc, "ruby-22-centos7-jenkins-slave", "--wait", "--from-dir", "examples/jenkins/master-slave/slave")
			br.AssertSuccess()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("grant service account in jenkins container access to API")
			err = oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+oc.Namespace()+":default", "-n", oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("instantiate the master template")
			err = oc.Run("new-app").Args("--template", "jenkins-master").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("build the Jenkins master")
			br, err = exutil.StartBuildAndWait(oc, "jenkins-master", "--wait", "--from-dir", "examples/jenkins/master-slave")
			br.AssertSuccess()
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

			g.By("get admin password")
			password := getAdminPassword(oc)
			o.Expect(password).ShouldNot(o.BeEmpty())

			j := JenkinsRef{
				oc:        oc,
				host:      serviceIP,
				port:      port,
				password:  password,
				namespace: oc.Namespace(),
			}

			g.By(fmt.Sprintf("wait for jenkins to come up at http://%s:%s", serviceIP, port))
			_, err = j.waitForContent("", 200, 3*time.Minute, "")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("inspecting the Jenkins master logs the slave image should be registered")
			out, err := oc.Run("logs").Args("dc/jenkins").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Adding image ruby-22-centos7-jenkins-slave:latest as Kubernetes slave"))

			g.By("kick the ruby-hello-world-test job")
			j.startJob("ruby-hello-world-test")
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
