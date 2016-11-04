package image_ecosystem

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/wait"

	"os"
	"os/exec"

	exutil "github.com/openshift/origin/test/extended/util"
)

type JenkinsRef struct {
	oc   *exutil.CLI
	host string
	port string
	// The namespace in which the Jenkins server is running
	namespace string
	password  string
}

const (
	useLocalPluginSnapshotEnvVarName = "USE_SNAPSHOT_JENKINS_IMAGE"
	localPluginSnapshotImage         = "openshift/jenkins-plugin-snapshot-test:latest"
)

// Struct can be marshalled into XML to represent a Jenkins workflow job definition.
type FlowDefinition struct {
	XMLName          xml.Name `xml:"flow-definition"`
	Plugin           string   `xml:"plugin,attr"`
	KeepDependencies bool     `xml:"keepDependencies"`
	Definition       Definition
}

type Definition struct {
	XMLName xml.Name `xml:"definition"`
	Class   string   `xml:"class,attr"`
	Plugin  string   `xml:"plugin,attr"`
	Script  string   `xml:"script"`
}

// Builds a URI for the Jenkins server.
func (j *JenkinsRef) buildURI(resourcePathFormat string, a ...interface{}) string {
	resourcePath := fmt.Sprintf(resourcePathFormat, a...)
	return fmt.Sprintf("http://%s:%v/%s", j.host, j.port, resourcePath)
}

// Submits a GET request to this Jenkins server.
// Returns a response body and status code or an error.
func (j *JenkinsRef) getResource(resourcePathFormat string, a ...interface{}) (string, int, error) {
	uri := j.buildURI(resourcePathFormat, a...)
	ginkgolog("Retrieving Jekins resource: %q", uri)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", 0, fmt.Errorf("Unable to build request for uri %q: %v", uri, err)
	}

	// http://stackoverflow.com/questions/17714494/golang-http-request-results-in-eof-errors-when-making-multiple-requests-successi
	req.Close = true

	req.SetBasicAuth("admin", j.password)
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return "", 0, fmt.Errorf("Unable to GET uri %q: %v", uri, err)
	}

	defer resp.Body.Close()
	status := resp.StatusCode

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("Error reading GET response %q: %v", uri, err)
	}

	return string(body), status, nil
}

// Sends a POST to the Jenkins server. If a body is specified, it should be XML.
// Returns response body and status code or an error.
func (j *JenkinsRef) postXML(reqBody io.Reader, resourcePathFormat string, a ...interface{}) (string, int, error) {
	return j.post(reqBody, resourcePathFormat, "application/xml", a...)
}

// Sends a POST to the Jenkins server. Returns response body and status code or an error.
func (j *JenkinsRef) post(reqBody io.Reader, resourcePathFormat, contentType string, a ...interface{}) (string, int, error) {
	uri := j.buildURI(resourcePathFormat, a...)

	req, err := http.NewRequest("POST", uri, reqBody)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	// http://stackoverflow.com/questions/17714494/golang-http-request-results-in-eof-errors-when-making-multiple-requests-successi
	req.Close = true

	if reqBody != nil {
		req.Header.Set("Content-Type", contentType)
		req.Header.Del("Expect") // jenkins will return 417 if we have an expect hdr
	}
	req.SetBasicAuth("admin", j.password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("Error posting request to %q: %v", uri, err)
	}

	defer resp.Body.Close()
	status := resp.StatusCode

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("Error reading post response body %q: %v", uri, err)
	}

	return string(body), status, nil
}

// Creates simple entry in the GinkgoWriter.
func ginkgolog(format string, a ...interface{}) {
	fmt.Fprintf(g.GinkgoWriter, format+"\n", a...)
}

// Repeatedly tries to GET a jenkins resource with an acceptable
// HTTP status. Retries for the specified duration.
func (j *JenkinsRef) getResourceWithStatus(validStatusList []int, timeout time.Duration, resourcePathFormat string, a ...interface{}) (string, int, error) {
	var retBody string
	var retStatus int
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {
		body, status, err := j.getResource(resourcePathFormat, a...)
		if err != nil {
			ginkgolog("Error accessing resource: %v", err)
			return false, nil
		}
		var found bool
		for _, s := range validStatusList {
			if status == s {
				found = true
				break
			}
		}
		if !found {
			ginkgolog("Expected http status [%v] during GET by recevied [%v]", validStatusList, status)
			return false, nil
		}
		retBody = body
		retStatus = status
		return true, nil
	})
	if err != nil {
		uri := j.buildURI(resourcePathFormat, a...)
		return "", retStatus, fmt.Errorf("Error waiting for status %v from resource path %s: %v", validStatusList, uri, err)
	}
	return retBody, retStatus, nil
}

// Waits for a particular HTTP status and HTML matching a particular
// pattern to be returned by this Jenkins server. An error will be returned
// if the condition is not matched within the timeout period.
func (j *JenkinsRef) waitForContent(verificationRegEx string, verificationStatus int, timeout time.Duration, resourcePathFormat string, a ...interface{}) (string, error) {
	var matchingContent = ""
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {

		content, _, err := j.getResourceWithStatus([]int{verificationStatus}, timeout, resourcePathFormat, a...)
		if err != nil {
			return false, nil
		}

		if len(verificationRegEx) > 0 {
			re := regexp.MustCompile(verificationRegEx)
			if re.MatchString(content) {
				matchingContent = content
				return true, nil
			} else {
				ginkgolog("Content did not match verification regex %q:\n %v", verificationRegEx, content)
				return false, nil
			}
		} else {
			matchingContent = content
			return true, nil
		}
	})

	if err != nil {
		uri := j.buildURI(resourcePathFormat, a...)
		return "", fmt.Errorf("Error waiting for status %v and verification regex %q from resource path %s: %v", verificationStatus, verificationRegEx, uri, err)
	} else {
		return matchingContent, nil
	}
}

// Submits XML to create a named item on the Jenkins server.
func (j *JenkinsRef) createItem(name string, itemDefXML string) {
	g.By(fmt.Sprintf("Creating new jenkins item: %s", name))
	_, status, err := j.postXML(bytes.NewBufferString(itemDefXML), "createItem?name=%s", name)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	o.ExpectWithOffset(1, status).To(o.Equal(200))
}

type JobMon struct {
	j               *JenkinsRef
	lastBuildNumber string
	buildNumber     string
	jobName         string
}

// Returns the current buildNumber on the named project OR "new" if
// there are no builds against a job yet.
func (j *JenkinsRef) getJobBuildNumber(name string, timeout time.Duration) (string, error) {
	body, status, err := j.getResourceWithStatus([]int{200, 404}, timeout, "job/%s/lastBuild/buildNumber", name)
	if err != nil {
		return "", err
	}
	if status != 200 {
		return "new", nil
	}
	return body, nil
}

// Waits for the timestamp on the Jenkins job to change. Returns
// and error if the timeout expires.
func (jmon *JobMon) await(timeout time.Duration) error {
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {

		buildNumber, err := jmon.j.getJobBuildNumber(jmon.jobName, time.Minute)
		o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

		ginkgolog("Checking build number for job %q current[%v] vs last[%v]", jmon.jobName, buildNumber, jmon.lastBuildNumber)
		if buildNumber == jmon.lastBuildNumber {
			return false, nil
		}

		if jmon.buildNumber == "" {
			jmon.buildNumber = buildNumber
		}
		body, status, err := jmon.j.getResource("job/%s/%s/api/json?depth=1", jmon.jobName, jmon.buildNumber)
		o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
		o.ExpectWithOffset(1, status).To(o.Equal(200))

		body = strings.ToLower(body)
		if strings.Contains(body, "\"building\":true") {
			ginkgolog("Jenkins job %q still building:\n%s\n\n", jmon.jobName, body)
			return false, nil
		}

		if strings.Contains(body, "\"result\":null") {
			ginkgolog("Jenkins job %q still building result:\n%s\n\n", jmon.jobName, body)
			return false, nil
		}

		ginkgolog("Jenkins job %q build complete:\n%s\n\n", jmon.jobName, body)
		return true, nil
	})
	return err
}

// Triggers a named Jenkins job. The job can be monitored with the
// returned object.
func (j *JenkinsRef) startJob(jobName string) *JobMon {
	lastBuildNumber, err := j.getJobBuildNumber(jobName, time.Minute)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	jmon := &JobMon{
		j:               j,
		lastBuildNumber: lastBuildNumber,
		buildNumber:     "",
		jobName:         jobName,
	}

	ginkgolog("Current timestamp for [%s]: %q", jobName, jmon.lastBuildNumber)
	g.By(fmt.Sprintf("Starting jenkins job: %s", jobName))
	_, status, err := j.postXML(nil, "job/%s/build?delay=0sec", jobName)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	o.ExpectWithOffset(1, status).To(o.Equal(201))

	return jmon
}

// Returns the content of a Jenkins job XML file. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
func (j *JenkinsRef) readJenkinsJob(filename, namespace string) string {
	pre := exutil.FixturePath("testdata", "jenkins-plugin", filename)
	post := exutil.ArtifactPath(filename)
	err := exutil.VarSubOnFile(pre, post, "PROJECT_NAME", namespace)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	data, err := ioutil.ReadFile(post)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	return string(data)
}

// Returns an XML string defining a Jenkins workflow/pipeline DSL job. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
func (j *JenkinsRef) buildDSLJob(namespace string, scriptLines ...string) (string, error) {
	script := strings.Join(scriptLines, "\n")
	script = strings.Replace(script, "PROJECT_NAME", namespace, -1)
	fd := FlowDefinition{
		Plugin: "workflow-job@2.7",
		Definition: Definition{
			Class:  "org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition",
			Plugin: "workflow-cps@2.18",
			Script: script,
		},
	}
	output, err := xml.MarshalIndent(fd, "  ", "    ")
	ginkgolog("Formulated DSL Project XML:\n%s\n\n", output)
	return string(output), err
}

// Returns the console logs of a particular buildNumber.
func (j *JenkinsRef) getJobConsoleLogs(jobName, buildNumber string) (string, error) {
	return j.waitForContent("", 200, 10*time.Minute, "job/%s/%s/consoleText", jobName, buildNumber)
}

// Returns the last build associated with a Jenkins job.
func (j *JenkinsRef) getLastJobConsoleLogs(jobName string) (string, error) {
	return j.getJobConsoleLogs(jobName, "lastBuild")
}

// Loads a Jenkins related template using new-app.
func loadFixture(oc *exutil.CLI, filename string) {
	resourcePath := exutil.FixturePath("testdata", "jenkins-plugin", filename)
	err := oc.Run("new-app").Args(resourcePath).Execute()
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
}

func assertEnvVars(oc *exutil.CLI, buildPrefix string, varsToFind map[string]string) {

	buildList, err := oc.REST().Builds(oc.Namespace()).List(kapi.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure that expected start-build environment variables were injected
	for _, build := range buildList.Items {
		ginkgolog("Found build: %q", build.GetName())
		if strings.HasPrefix(build.GetName(), buildPrefix) {
			envs := []kapi.EnvVar{}
			if build.Spec.Strategy.DockerStrategy != nil && build.Spec.Strategy.DockerStrategy.Env != nil {
				envs = build.Spec.Strategy.DockerStrategy.Env
			} else if build.Spec.Strategy.SourceStrategy != nil && build.Spec.Strategy.SourceStrategy.Env != nil {
				envs = build.Spec.Strategy.SourceStrategy.Env
			} else {
				continue
			}

			for k, v := range varsToFind {
				found := false
				for _, env := range envs {
					ginkgolog("Found %s=%s in build %s", env.Name, env.Value, build.GetName())
					if k == env.Name && v == env.Value {
						found = true
						break
					}
				}
				o.ExpectWithOffset(1, found).To(o.BeTrue())
			}
		}
	}
}

func getAdminPassword(oc *exutil.CLI) string {
	envs, err := oc.Run("set").Args("env", "dc/jenkins", "--list").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	kvs := strings.Split(envs, "\n")
	for _, kv := range kvs {
		if strings.HasPrefix(kv, "JENKINS_PASSWORD=") {
			s := strings.Split(kv, "=")
			fmt.Fprintf(g.GinkgoWriter, "\nJenkins admin password %s\n", s[1])
			return s[1]
		}
	}
	return "password"
}

func followDCLogs(oc *exutil.CLI, jenkinsNamespace string) {
	oc.SetNamespace(jenkinsNamespace)
	oc.Run("logs").Args("-f", "dc/jenkins").Execute()

}

var _ = g.Describe("[image_ecosystem][jenkins][Slow] openshift pipeline plugin", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("jenkins-plugin", exutil.KubeConfigPath())
	var j *JenkinsRef
	var dcLogFollow *exec.Cmd
	var dcLogStdOut, dcLogStdErr *bytes.Buffer

	g.AfterEach(func() {
		ginkgolog("Jenkins DC description follows. If there were issues, check to see if there were any restarts in the jenkins pod.")
		exutil.DumpDeploymentLogs("jenkins", oc)

		// Destroy the Jenkins namespace
		oc.Run("delete").Args("project", j.namespace).Execute()
		if dcLogFollow != nil && dcLogStdOut != nil && dcLogStdErr != nil {
			ginkgolog("Waiting for Jenkins DC log follow to terminate")
			dcLogFollow.Process.Wait()
			ginkgolog("Jenkins server logs from test:\nstdout>\n%s\n\nstderr>\n%s\n\n", string(dcLogStdOut.Bytes()), string(dcLogStdErr.Bytes()))
			dcLogFollow = nil
		} else {
			ginkgolog("Logs were not captured!\n%v\n%v\n%v\n", dcLogFollow, dcLogStdOut, dcLogStdErr)
		}
	})

	g.BeforeEach(func() {
		testNamespace := oc.Namespace()

		jenkinsNamespace := oc.Namespace() + "-jenkins"
		g.By("Starting a Jenkins instance in namespace: " + jenkinsNamespace)

		oc.Run("new-project").Args(jenkinsNamespace).Execute()
		oc.SetNamespace(jenkinsNamespace)

		time.Sleep(10 * time.Second) // Give project time to initialize

		g.By("kick off the build for the jenkins ephermeral and application templates")
		tag := []string{localPluginSnapshotImage}
		hexIDs, err := exutil.DumpAndReturnTagging(tag)

		// If the user has expressed an interest in local plugin testing by setting the
		// SNAPSHOT_JENKINS_IMAGE environment variable, try to use the local image. Inform them
		// either about which image is being used in case their test fails.
		snapshotImagePresent := len(hexIDs) > 0 && err == nil
		useSnapshotImage := os.Getenv(useLocalPluginSnapshotEnvVarName) != ""

		//TODO disabling oauth until we can update getAdminPassword path to handle oauth (perhaps borrow from oauth integration tests)
		newAppArgs := []string{exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json"), "-p", "ENABLE_OAUTH=false"}

		if useSnapshotImage {
			g.By("Creating a snapshot Jenkins imagestream and overridding the default Jenkins imagestream")
			o.Expect(snapshotImagePresent).To(o.BeTrue())

			ginkgolog("")
			ginkgolog("")
			ginkgolog("IMPORTANT: You are testing a local jenkins snapshot image.")
			ginkgolog("In order to target the official image stream, you must unset %s before running extended tests.", useLocalPluginSnapshotEnvVarName)
			ginkgolog("")
			ginkgolog("")

			// Create an imagestream based on the Jenkins' plugin PR-Testing image (https://github.com/openshift/jenkins-plugin/blob/master/PR-Testing/README).
			snapshotImageStream := "jenkins-plugin-snapshot-test"
			err = oc.Run("new-build").Args("-D", fmt.Sprintf("FROM %s", localPluginSnapshotImage), "--to", snapshotImageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("logs").Args("-f", "bc/jenkins-plugin-snapshot-test").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Supplant the normal imagestream with the local imagestream using template parameters
			newAppArgs = append(newAppArgs, "-p", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()))
			newAppArgs = append(newAppArgs, "-p", fmt.Sprintf("JENKINS_IMAGE_STREAM_TAG=%s:latest", snapshotImageStream))

		} else {
			if snapshotImagePresent {
				ginkgolog("")
				ginkgolog("")
				ginkgolog("IMPORTANT: You have a local OpenShift jenkins snapshot image, but it is not being used for testing.")
				ginkgolog("In order to target your local image, you must set %s to some value before running extended tests.", useLocalPluginSnapshotEnvVarName)
				ginkgolog("")
				ginkgolog("")
			}
		}

		err = oc.Run("new-app").Args(newAppArgs...).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for jenkins deployment")
		err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "jenkins", oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("get ip and port for jenkins service")
		serviceIP, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{.spec.clusterIP}}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		port, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{ $x := index .spec.ports 0}}{{$x.port}}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("get admin password")
		password := getAdminPassword(oc)
		o.Expect(password).ShouldNot(o.BeEmpty())

		j = &JenkinsRef{
			oc:        oc,
			host:      serviceIP,
			port:      port,
			namespace: jenkinsNamespace,
			password:  password,
		}

		g.By("wait for jenkins to come up")
		_, err = j.waitForContent("", 200, 10*time.Minute, "")

		if err != nil {
			exutil.DumpDeploymentLogs("jenkins", oc)
		}

		o.Expect(err).NotTo(o.HaveOccurred())

		if useSnapshotImage {
			g.By("verifying the test image is being used")
			// for the test image, confirm that a snapshot version of the plugin is running in the jenkins image we'll test against
			_, err = j.waitForContent(`About OpenShift Pipeline Jenkins Plugin ([0-9\.]+)-SNAPSHOT`, 200, 10*time.Minute, "/pluginManager/plugin/openshift-pipeline/thirdPartyLicenses")
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		// Start capturing logs from this deployment config.
		// This command will terminate if the Jekins instance crashes. This
		// ensures that even if the Jenkins DC restarts, we should capture
		// logs from the crash.
		dcLogFollow, dcLogStdOut, dcLogStdErr, err = oc.Run("logs").Args("-f", "dc/jenkins").Background()
		o.Expect(err).NotTo(o.HaveOccurred())

		oc.SetNamespace(testNamespace)

		g.By("set up policy for jenkins jobs in " + oc.Namespace())
		err = oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+j.namespace+":jenkins").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Populate shared Jenkins namespace with artifacts that can be used by all tests
		loadFixture(oc, "shared-resources-template.json")

		// Allow resources to settle. ImageStream tags seem unavailable without this wait.
		time.Sleep(10 * time.Second)

	})

	g.Context("jenkins-plugin test context  ", func() {

		g.It("jenkins-plugin test trigger build", func() {

			jobName := "test-build-job"
			data := j.readJenkinsJob("build-job.xml", oc.Namespace())
			j.createItem(jobName, data)
			jmon := j.startJob(jobName)
			jmon.await(10 * time.Minute)

			// the build and deployment is by far the most time consuming portion of the test jenkins job;
			// we leverage some of the openshift utilities for waiting for the deployment before we poll
			// jenkins for the successful job completion
			g.By("waiting for frontend, frontend-prod deployments as signs that the build has finished")
			err := exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "frontend", oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "frontend-prod", oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("get build console logs and see if succeeded")
			logs, err := j.waitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)
			ginkgolog("\n\nJenkins logs>\n%s\n\n", logs)
			o.Expect(err).NotTo(o.HaveOccurred())

		})

		g.It("jenkins-plugin test trigger build with envs", func() {

			jobName := "test-build-with-env-job"
			data := j.readJenkinsJob("build-with-env-job.xml", oc.Namespace())
			j.createItem(jobName, data)
			jmon := j.startJob(jobName)
			jmon.await(10 * time.Minute)

			logs, err := j.getLastJobConsoleLogs(jobName)
			ginkgolog("\n\nJenkins logs>\n%s\n\n", logs)
			o.Expect(err).NotTo(o.HaveOccurred())

			// the build and deployment is by far the most time consuming portion of the test jenkins job;
			// we leverage some of the openshift utilities for waiting for the deployment before we poll
			// jenkins for the successful job completion
			g.By("waiting for frontend deployments as signs that the build has finished")
			err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "frontend", oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("get build console logs and see if succeeded")
			_, err = j.waitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)

			assertEnvVars(oc, "frontend-", map[string]string{
				"a": "b",
				"C": "D",
				"e": "",
			})

		})

		g.It("jenkins-plugin test trigger build DSL", func() {

			buildsBefore, err := oc.REST().Builds(oc.Namespace()).List(kapi.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			data, err := j.buildDSLJob(oc.Namespace(),
				"node{",
				"openshiftBuild( namespace:'PROJECT_NAME', bldCfg: 'frontend', env: [ [ name : 'a', value : 'b' ], [ name : 'C', value : 'D' ], [ name : 'e', value : '' ] ] )",
				"}",
			)

			jobName := "test-build-dsl-job"
			j.createItem(jobName, data)
			monitor := j.startJob(jobName)
			err = monitor.await(10 * time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
				buildsAfter, err := oc.REST().Builds(oc.Namespace()).List(kapi.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				return (len(buildsAfter.Items) != len(buildsBefore.Items)), nil
			})

			buildsAfter, err := oc.REST().Builds(oc.Namespace()).List(kapi.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(buildsAfter.Items)).To(o.Equal(len(buildsBefore.Items) + 1))

			log, err := j.getLastJobConsoleLogs(jobName)
			ginkgolog("Job logs>>\n%s\n\n", log)

			assertEnvVars(oc, "frontend-", map[string]string{
				"a": "b",
				"C": "D",
				"e": "",
			})

		})

		g.It("jenkins-plugin test exec DSL", func() {

			// Create a running pod in which we can execute our commands
			oc.Run("new-app").Args("https://github.com/openshift/ruby-hello-world").Execute()

			var targetPod *kapi.Pod
			err := wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
				pods, err := oc.KubeREST().Pods(oc.Namespace()).List(kapi.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				for _, p := range pods.Items {
					if !strings.Contains(p.Name, "build") && !strings.Contains(p.Name, "deploy") && p.Status.Phase == "Running" {
						targetPod = &p
						return true, nil
					}
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			targetContainer := targetPod.Spec.Containers[0]

			data, err := j.buildDSLJob(oc.Namespace(),
				"node{",
				fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', command: [ 'echo', 'hello', 'world', '1' ] )", targetPod.Name),
				fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', command: 'echo', arguments : [ 'hello', 'world', '2' ] )", targetPod.Name),
				fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', command: 'echo', arguments : [ [ value: 'hello' ], [ value : 'world' ], [ value : '3' ] ] )", targetPod.Name),
				fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', container: '%s', command: [ 'echo', 'hello', 'world', '4' ] )", targetPod.Name, targetContainer.Name),
				fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', container: '%s', command: 'echo', arguments : [ 'hello', 'world', '5' ] )", targetPod.Name, targetContainer.Name),
				fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', container: '%s', command: 'echo', arguments : [ [ value: 'hello' ], [ value : 'world' ], [ value : '6' ] ] )", targetPod.Name, targetContainer.Name),
				"}",
			)

			jobName := "test-exec-dsl-job"
			j.createItem(jobName, data)
			monitor := j.startJob(jobName)
			err = monitor.await(10 * time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			log, err := j.getLastJobConsoleLogs(jobName)
			ginkgolog("Job logs>>\n%s\n\n", log)

			o.Expect(strings.Contains(log, "hello world 1")).To(o.BeTrue())
			o.Expect(strings.Contains(log, "hello world 2")).To(o.BeTrue())
			o.Expect(strings.Contains(log, "hello world 3")).To(o.BeTrue())
			o.Expect(strings.Contains(log, "hello world 4")).To(o.BeTrue())
			o.Expect(strings.Contains(log, "hello world 5")).To(o.BeTrue())
			o.Expect(strings.Contains(log, "hello world 6")).To(o.BeTrue())
		})

		g.It("jenkins-plugin test multitag", func() {

			loadFixture(oc, "multitag-template.json")
			err := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
				_, err := oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag3", "orig")
				if err != nil {
					return false, nil
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			jobName := "test-multitag-job"
			data := j.readJenkinsJob("multitag-job.xml", oc.Namespace())
			j.createItem(jobName, data)
			monitor := j.startJob(jobName)
			err = monitor.await(10 * time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			log, err := j.getLastJobConsoleLogs(jobName)
			ginkgolog("Job logs>>\n%s\n\n", log)

			// Assert stream tagging results
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag", "prod")
			o.Expect(err).NotTo(o.HaveOccurred())

			// 1 to N mapping
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag", "prod2")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag", "prod3")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag", "prod4")
			o.Expect(err).NotTo(o.HaveOccurred())

			// N to 1 mapping
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag", "prod5")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag2", "prod5")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag3", "prod5")
			o.Expect(err).NotTo(o.HaveOccurred())

			// N to N mapping
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag", "prod6")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag2", "prod7")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag3", "prod8")
			o.Expect(err).NotTo(o.HaveOccurred())

			// N to N mapping with creation
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag4", "prod9")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag5", "prod10")
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag6", "prod11")
			o.Expect(err).NotTo(o.HaveOccurred())

		})

		g.It("jenkins-plugin test multitag DSL", func() {

			testNamespace := oc.Namespace()

			loadFixture(oc, "multitag-template.json")
			err := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
				_, err := oc.REST().ImageStreamTags(oc.Namespace()).Get("multitag3", "orig")
				if err != nil {
					return false, nil
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			anotherNamespace := oc.Namespace() + "-multitag-target"
			oc.Run("new-project").Args(anotherNamespace).Execute()

			time.Sleep(10 * time.Second) // Give project time to initialize policies.

			// Allow jenkins service account to edit the new namespace
			oc.SetNamespace(anotherNamespace)
			err = oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+j.namespace+":jenkins").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			oc.SetNamespace(testNamespace)

			ginkgolog("Using testNamespace: %q and currentNamespace: %q", testNamespace, oc.Namespace())

			data, err := j.buildDSLJob(oc.Namespace(),
				"node{",
				"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag', destTag: 'prod' )",
				"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2', destTag: 'prod1, prod2, prod3' )",
				"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2,multitag7', destTag: 'prod4' )",
				"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag5,multitag6', destTag: 'prod5, prod6' )",
				fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag', destTag: 'prod' )", anotherNamespace),
				fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2', destTag: 'prod1, prod2, prod3' )", anotherNamespace),
				fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2,multitag7', destTag: 'prod4' )", anotherNamespace),
				fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag5,multitag6', destTag: 'prod5, prod6' )", anotherNamespace),
				"}",
			)

			jobName := "test-multitag-dsl-job"
			j.createItem(jobName, data)
			monitor := j.startJob(jobName)
			err = monitor.await(10 * time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			time.Sleep(10 * time.Second)

			log, err := j.getLastJobConsoleLogs(jobName)
			o.Expect(err).NotTo(o.HaveOccurred())
			ginkgolog("Job logs>>\n%s\n\n", log)

			// Assert stream tagging results
			for _, namespace := range []string{oc.Namespace(), anotherNamespace} {
				g.By("Checking tags in namespace: " + namespace)
				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag", "prod")
				o.Expect(err).NotTo(o.HaveOccurred())

				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag2", "prod1")
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag2", "prod2")
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag2", "prod3")
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag2", "prod4")
				o.Expect(err).NotTo(o.HaveOccurred())

				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag5", "prod5")
				o.Expect(err).NotTo(o.HaveOccurred())

				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag6", "prod6")
				o.Expect(err).NotTo(o.HaveOccurred())

				_, err = oc.REST().ImageStreamTags(namespace).Get("multitag7", "prod4")
				o.Expect(err).NotTo(o.HaveOccurred())
			}

		})

		testImageStreamSCM := func(jobXMLFile string) {
			jobName := "test-imagestream-scm"
			g.By("creating a jenkins job with an imagestream SCM")
			data := j.readJenkinsJob(jobXMLFile, oc.Namespace())
			j.createItem(jobName, data)

			// Because polling is enabled, a job should start automatically and fail
			// Wait for it to run and fail
			tree := url.QueryEscape("jobs[name,color]")
			xpath := url.QueryEscape("//job/name[text()='test-imagestream-scm']/../color")
			jobStatusURI := "api/xml?tree=%s&xpath=%s"
			g.By("waiting for initial job to complete")
			wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
				result, status, err := j.getResource(jobStatusURI, tree, xpath)
				o.Expect(err).NotTo(o.HaveOccurred())
				if status == 200 && strings.Contains(result, "red") {
					return true, nil
				}
				return false, nil
			})

			// Create a new imagestream tag and expect a job to be kicked off
			// that will create a new tag in the current namespace
			g.By("creating an imagestream tag in the current project")
			oc.Run("tag").Args("openshift/jenkins:latest", fmt.Sprintf("%s/testimage:v1", oc.Namespace())).Execute()

			// Wait after the image has been tagged for the Jenkins job to run
			// and create the new imagestream/tag
			g.By("verifying that the job ran by looking for the resulting imagestream tag")
			err := exutil.TimedWaitForAnImageStreamTag(oc, oc.Namespace(), "localjenkins", "develop", 10*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.It("jenkins-plugin test imagestream SCM", func() {
			testImageStreamSCM("imagestream-scm-job.xml")
		})

		g.It("jenkins-plugin test imagestream SCM DSL", func() {
			testImageStreamSCM("imagestream-scm-dsl-job.xml")
		})

		g.It("jenkins-plugin test connection test", func() {

			jobName := "test-build-job"
			data := j.readJenkinsJob("build-job.xml", oc.Namespace())
			j.createItem(jobName, data)

			g.By("trigger test connection logic, check for success")
			testConnectionBody := bytes.NewBufferString("apiURL=&authToken=")
			result, code, err := j.post(testConnectionBody, "job/test-build-job/descriptorByName/com.openshift.jenkins.plugins.pipeline.OpenShiftBuilder/testConnection", "application/x-www-form-urlencoded")
			if code != 200 {
				err = fmt.Errorf("Expected return code of 200")
			}
			if matched, _ := regexp.MatchString(".*Connection successful.*", result); !matched {
				err = fmt.Errorf("Expecting 'Connection successful', Got: %s", result)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("trigger test connection logic, check for failure")
			testConnectionBody = bytes.NewBufferString("apiURL=https%3A%2F%2F1.2.3.4&authToken=")
			result, code, err = j.post(testConnectionBody, "job/test-build-job/descriptorByName/com.openshift.jenkins.plugins.pipeline.OpenShiftBuilder/testConnection", "application/x-www-form-urlencoded")
			if code != 200 {
				err = fmt.Errorf("Expected return code of 200")
			}
			if matched, _ := regexp.MatchString(".*Connection unsuccessful.*", result); !matched {
				err = fmt.Errorf("Expecting 'Connection unsuccessful', Got: %s", result)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

		})
	})
})
