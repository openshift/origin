package jenkins

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	exurl "github.com/openshift/origin/test/extended/util/compat_otp/url"
)

const (
	UseLocalPluginSnapshotEnvVarName       = "USE_SNAPSHOT_JENKINS_IMAGE"
	UseLocalClientPluginSnapshotEnvVarName = "USE_SNAPSHOT_JENKINS_CLIENT_IMAGE"
	UseLocalSyncPluginSnapshotEnvVarName   = "USE_SNAPSHOT_JENKINS_SYNC_IMAGE"
	UseLocalLoginPluginSnapshotEnvVarName  = "USE_SNAPSHOT_JENKINS_LOGIN_IMAGE"
)

// JenkinsRef represents a Jenkins instance running on an OpenShift server
type JenkinsRef struct {
	oc   *exutil.CLI
	host string
	port string
	// The namespace in which the Jenkins server is running
	namespace  string
	token      string
	uri_tester *exurl.Tester
}

// FlowDefinition can be marshalled into XML to represent a Jenkins workflow job definition.
type FlowDefinition struct {
	XMLName          xml.Name `xml:"flow-definition"`
	Plugin           string   `xml:"plugin,attr"`
	KeepDependencies bool     `xml:"keepDependencies"`
	Definition       Definition
}

// Definition is part of a FlowDefinition
type Definition struct {
	XMLName xml.Name `xml:"definition"`
	Class   string   `xml:"class,attr"`
	Plugin  string   `xml:"plugin,attr"`
	Script  string   `xml:"script"`
}

// NewRef creates a jenkins reference from an OC client
func NewRef(oc *exutil.CLI) *JenkinsRef {
	g.By("get ip and port for jenkins service")
	serviceIP, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{.spec.clusterIP}}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	port, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{ $x := index .spec.ports 0}}{{$x.port}}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("get token via whoami")
	token, err := oc.Run("whoami").Args("-t").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	j := &JenkinsRef{
		oc:         oc,
		host:       serviceIP,
		port:       port,
		namespace:  oc.Namespace(),
		token:      token,
		uri_tester: exurl.NewTester(oc.AdminKubeClient(), oc.Namespace()),
	}
	return j
}

// Namespace returns the Jenkins namespace
func (j *JenkinsRef) Namespace() string {
	return j.namespace
}

// BuildURI builds a URI for the Jenkins server.
func (j *JenkinsRef) BuildURI(resourcePathFormat string, a ...interface{}) string {
	resourcePath := fmt.Sprintf(resourcePathFormat, a...)
	return fmt.Sprintf("http://%s:%v/%s", j.host, j.port, resourcePath)
}

// GetResource submits a GET request to this Jenkins server.
// Returns a response body and status code or an error.
func (j *JenkinsRef) GetResource(resourcePathFormat string, a ...interface{}) (string, int, error) {
	uri := j.BuildURI(resourcePathFormat, a...)
	e2e.Logf("Retrieving Jenkins resource: %q", uri)
	response := j.uri_tester.WithErrorPassthrough(true).Response(
		exurl.Expect("GET", uri).WithToken(j.token),
	)
	var err error
	if len(response.Error) > 0 {
		err = fmt.Errorf("%s", response.Error)
	}
	rc := -1
	if response.Response != nil {
		rc = response.Response.StatusCode
	}
	return string(response.Body), rc, err
}

// Post sends a POST to the Jenkins server. Returns response body and status code or an error.
func (j *JenkinsRef) Post(reqBodyFile, resourcePathFormat, contentType string, a ...interface{}) (string, int, error) {
	uri := j.BuildURI(resourcePathFormat, a...)
	response := j.uri_tester.WithErrorPassthrough(true).Response(
		exurl.Expect("POST", uri).WithBodyToUpload(reqBodyFile, j.uri_tester.Podname(), j.oc).WithToken(j.token).WithHeader("Content-Type", contentType),
	)
	var err error
	if len(response.Error) > 0 {
		err = fmt.Errorf("%s", response.Error)
	}
	rc := -1
	if response.Response != nil {
		rc = response.Response.StatusCode
	}
	return string(response.Body), rc, err
}

// PostXML sends a POST to the Jenkins server. If a body is specified, it should be XML.
// Returns response body and status code or an error.
func (j *JenkinsRef) PostXML(reqBodyFile, resourcePathFormat string, a ...interface{}) (string, int, error) {
	return j.Post(reqBodyFile, resourcePathFormat, "application/xml", a...)
}

// GetResourceWithStatus repeatedly tries to GET a jenkins resource with an acceptable
// HTTP status. Retries for the specified duration.
func (j *JenkinsRef) GetResourceWithStatus(validStatusList []int, timeout time.Duration, resourcePathFormat string, a ...interface{}) (string, int, error) {
	var retBody string
	var retStatus int
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {
		body, status, err := j.GetResource(resourcePathFormat, a...)
		if err != nil {
			e2e.Logf("Error accessing resource: %v", err)
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
			e2e.Logf("Expected http status [%v] during GET but received [%v] for %s with body %s", validStatusList, status, resourcePathFormat, body)
			return false, nil
		}
		retBody = body
		retStatus = status
		return true, nil
	})
	if err != nil {
		uri := j.BuildURI(resourcePathFormat, a...)
		return "", retStatus, fmt.Errorf("Error waiting for status %v from resource path %s: %v", validStatusList, uri, err)
	}
	return retBody, retStatus, nil
}

// WaitForContent waits for a particular HTTP status and HTML matching a particular
// pattern to be returned by this Jenkins server. An error will be returned
// if the condition is not matched within the timeout period.
func (j *JenkinsRef) WaitForContent(verificationRegEx string, verificationStatus int, timeout time.Duration, resourcePathFormat string, a ...interface{}) (string, error) {
	var matchingContent = ""
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {

		content, _, err := j.GetResourceWithStatus([]int{verificationStatus}, timeout, resourcePathFormat, a...)
		if err != nil {
			return false, nil
		}

		if len(verificationRegEx) > 0 {
			re := regexp.MustCompile(verificationRegEx)
			if re.MatchString(content) {
				matchingContent = content
				return true, nil
			} else {
				e2e.Logf("Content did not match verification regex %q:\n %v", verificationRegEx, content)
				return false, nil
			}
		} else {
			matchingContent = content
			return true, nil
		}
	})

	if err != nil {
		uri := j.BuildURI(resourcePathFormat, a...)
		return "", fmt.Errorf("Error waiting for status %v and verification regex %q from resource path %s: %v", verificationStatus, verificationRegEx, uri, err)
	} else {
		return matchingContent, nil
	}
}

// CreateItem submits XML to create a named item on the Jenkins server.
func (j *JenkinsRef) CreateItem(name string, itemDefXML string) {
	g.By(fmt.Sprintf("Creating new jenkins item: %s", name))
	_, status, err := j.PostXML(itemDefXML, "createItem?name=%s", name)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	o.ExpectWithOffset(1, status).To(o.Equal(200))
}

// GetJobBuildNumber returns the current buildNumber on the named project OR "new" if
// there are no builds against a job yet.
func (j *JenkinsRef) GetJobBuildNumber(name string, timeout time.Duration) (string, error) {
	body, status, err := j.GetResourceWithStatus([]int{200, 404}, timeout, "job/%s/lastBuild/buildNumber", name)
	if err != nil {
		return "", err
	}
	if status != 200 {
		return "new", nil
	}
	return body, nil
}

// StartJob triggers a named Jenkins job. The job can be monitored with the
// returned object.
func (j *JenkinsRef) StartJob(jobName string) *JobMon {
	lastBuildNumber, err := j.GetJobBuildNumber(jobName, time.Minute)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	jmon := &JobMon{
		j:               j,
		lastBuildNumber: lastBuildNumber,
		buildNumber:     "",
		jobName:         jobName,
	}

	e2e.Logf("Current timestamp for [%s]: %q", jobName, jmon.lastBuildNumber)
	g.By(fmt.Sprintf("Starting jenkins job: %s", jobName))
	_, status, err := j.PostXML("", "job/%s/build?delay=0sec", jobName)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	o.ExpectWithOffset(1, status).To(o.Equal(201))

	return jmon
}

// ProcessJenkinsJobUsingVars returns the path of the modified Jenkins job XML file. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
// Variables named in the vars map will also be replaced with their
// corresponding value.
func (j *JenkinsRef) ProcessJenkinsJobUsingVars(filename, namespace string, vars map[string]string) string {
	pre := exutil.FixturePath("testdata", "jenkins-plugin", filename)
	post := exutil.ArtifactPath(filename)

	if vars == nil {
		vars = map[string]string{}
	}
	vars["PROJECT_NAME"] = namespace
	err := exutil.VarSubOnFile(pre, post, vars)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	data, err := ioutil.ReadFile(post)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	newfile, err := CreateTempFile(string(data))
	e2e.Logf("new temp file %s err %v", newfile, err)
	if err != nil {
		files, dbgerr := ioutil.ReadDir("/tmp")
		if dbgerr != nil {
			e2e.Logf("problem diagnosing /tmp: %v", dbgerr)
		} else {
			for _, file := range files {
				e2e.Logf("found file %s under temp isdir %q mode %s", file.Name(), file.IsDir(), file.Mode().String())
			}
		}
	}
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	return newfile

}

// ProcessJenkinsJob returns the path of the modified Jenkins job XML file. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
func (j *JenkinsRef) ProcessJenkinsJob(filename, namespace string) string {
	return j.ProcessJenkinsJobUsingVars(filename, namespace, nil)
}

// BuildDSLJob returns an XML string defining a Jenkins workflow/pipeline DSL job. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
func (j *JenkinsRef) BuildDSLJob(namespace string, scriptLines ...string) (string, error) {
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
	e2e.Logf("Formulated DSL Project XML:\n%s\n\n", output)
	return string(output), err
}

// GetJobConsoleLogs returns the console logs of a particular buildNumber.
func (j *JenkinsRef) GetJobConsoleLogs(jobName, buildNumber string) (string, error) {
	return j.WaitForContent("", 200, 10*time.Minute, "job/%s/%s/consoleText", jobName, buildNumber)
}

// GetJobConsoleLogsAndMatchViaBuildResult leverages various information in the BuildResult and
// returns the corresponding console logs, as well as look for matching string
func (j *JenkinsRef) GetJobConsoleLogsAndMatchViaBuildResult(br *exutil.BuildResult, match string) (string, error) {
	if br == nil {
		return "", fmt.Errorf("passed in nil BuildResult")
	}
	if br.Build == nil {
		if br.Oc == nil {
			return "", fmt.Errorf("BuildResult oc should have been set up during BuildResult construction")
		}
		var err error // interestingly, removing this line and using := on the next got a compile error
		br.Build, err = br.Oc.BuildClient().BuildV1().Builds(br.Oc.Namespace()).Get(context.Background(), br.BuildName, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
	}
	bldURI := br.Build.Annotations[buildv1.BuildJenkinsLogURLAnnotation]
	if len(bldURI) > 0 {
		// need to strip the route host...WaitForContent will prepend the svc ip:port we need to use in ext tests
		url, err := url.Parse(bldURI)
		if err != nil {
			return "", err
		}
		bldURI = strings.Trim(url.Path, "/")
		return j.WaitForContent(match, 200, 10*time.Minute, bldURI)
	}
	return "", fmt.Errorf("build %#v is missing the build uri annontation", br.Build)
}

// GetLastJobConsoleLogs returns the last build associated with a Jenkins job.
func (j *JenkinsRef) GetLastJobConsoleLogs(jobName string) (string, error) {
	return j.GetJobConsoleLogs(jobName, "lastBuild")
}

// Finds the pod running Jenkins
func FindJenkinsPod(oc *exutil.CLI) *corev1.Pod {
	pods, err := exutil.GetApplicationPods(oc, "jenkins")
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	if pods == nil || pods.Items == nil {
		g.Fail("No pods matching jenkins deploymentconfig in namespace " + oc.Namespace())
	}

	o.ExpectWithOffset(1, len(pods.Items)).To(o.Equal(1))
	return &pods.Items[0]
}

// OverridePodTemplateImages sees if this is a prow-gcp e2e test invocation, and we want to override the agent image for the default pod templates;
// the jenkins image will pick up the env vars passed to new-app and update the image field of the pod templates with the values
func OverridePodTemplateImages(newAppArgs []string) []string {
	nodejsAgent := os.Getenv("IMAGE_NODEJS_AGENT")
	if len(strings.TrimSpace(nodejsAgent)) > 0 {
		newAppArgs = append(newAppArgs, "-e", fmt.Sprintf("NODEJS_SLAVE_IMAGE=%s", nodejsAgent))
	}
	mavenAgent := os.Getenv("IMAGE_MAVEN_AGENT")
	if len(strings.TrimSpace(mavenAgent)) > 0 {
		newAppArgs = append(newAppArgs, "-e", fmt.Sprintf("MAVEN_SLAVE_IMAGE=%s", mavenAgent))
	}
	return newAppArgs
}

// SetupDockerhubImage pull in a jenkins image from docker.io for aws-build testing;
// at some point during 4.0 dev, the jenkins imagestream in the openshift namespace
// will leverage the rhel images from the terms based registry at registry.redhat.io
// where credentials will be needed; we want to test against pre-release images
func SetupDockerhubImage(localImageName, snapshotImageStream string, newAppArgs []string, oc *exutil.CLI) []string {
	g.By("Creating a Jenkins imagestream for overridding the default Jenkins imagestream in the openshift namespace")

	// Create an imagestream based on the Jenkins' plugin PR-Testing image (https://github.com/openshift/jenkins-plugin/blob/master/PR-Testing/README).
	err := oc.Run("new-build").Args("-D", fmt.Sprintf("FROM %s", localImageName), "--to", snapshotImageStream).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("waiting for build to finish")
	err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), snapshotImageStream+"-1", exutil.CheckBuildSuccess, exutil.CheckBuildFailed, exutil.CheckBuildCancelled)
	if err != nil {
		exutil.DumpBuildLogs(snapshotImageStream, oc)
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	// Supplant the normal imagestream with the local imagestream using template parameters
	newAppArgs = append(newAppArgs, "-p", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()))
	newAppArgs = append(newAppArgs, "-p", fmt.Sprintf("JENKINS_IMAGE_STREAM_TAG=%s:latest", snapshotImageStream))

	return newAppArgs
}

// pulls in a jenkins image built from a PR change for one of our plugins
func SetupSnapshotImage(envVarName, localImageName, snapshotImageStream string, newAppArgs []string, oc *exutil.CLI) ([]string, bool) {
	tag := []string{localImageName}
	hexIDs, err := exutil.DumpAndReturnTagging(tag)

	// If the user has expressed an interest in local plugin testing by setting the
	// SNAPSHOT_JENKINS_IMAGE environment variable, try to use the local image. Inform them
	// either about which image is being used in case their test fails.
	snapshotImagePresent := len(hexIDs) > 0 && err == nil
	useSnapshotImage := os.Getenv(envVarName) != ""

	if useSnapshotImage {
		g.By("Creating a snapshot Jenkins imagestream and overridding the default Jenkins imagestream")
		o.Expect(snapshotImagePresent).To(o.BeTrue())

		e2e.Logf("\n\nIMPORTANT: You are testing a local jenkins snapshot image.")
		e2e.Logf("In order to target the official image stream, you must unset %s before running extended tests.\n\n", envVarName)

		// Create an imagestream based on the Jenkins' plugin PR-Testing image (https://github.com/openshift/jenkins-plugin/blob/master/PR-Testing/README).
		err = oc.Run("new-build").Args("-D", fmt.Sprintf("FROM %s", localImageName), "--to", snapshotImageStream).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for build to finish")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), snapshotImageStream+"-1", exutil.CheckBuildSuccess, exutil.CheckBuildFailed, exutil.CheckBuildCancelled)
		if err != nil {
			exutil.DumpBuildLogs(snapshotImageStream, oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		// Supplant the normal imagestream with the local imagestream using template parameters
		newAppArgs = append(newAppArgs, "-p", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()))
		newAppArgs = append(newAppArgs, "-p", fmt.Sprintf("JENKINS_IMAGE_STREAM_TAG=%s:latest", snapshotImageStream))

	} else {
		if snapshotImagePresent {
			e2e.Logf("\n\nIMPORTANT: You have a local OpenShift jenkins snapshot image, but it is not being used for testing.")
			e2e.Logf("In order to target your local image, you must set %s to some value before running extended tests.\n\n", envVarName)
		}
	}

	return newAppArgs, useSnapshotImage
}

func ProcessLogURLAnnotations(oc *exutil.CLI, t *exutil.BuildResult) (*url.URL, error) {
	if len(t.Build.Annotations[buildv1.BuildJenkinsLogURLAnnotation]) == 0 {
		return nil, fmt.Errorf("build %s does not contain a Jenkins URL annotation", t.BuildName)
	}
	jenkinsLogURL, err := url.Parse(t.Build.Annotations[buildv1.BuildJenkinsLogURLAnnotation])
	if err != nil {
		return nil, fmt.Errorf("cannot parse jenkins log URL (%s): %v", t.Build.Annotations[buildv1.BuildJenkinsLogURLAnnotation], err)
	}
	if len(t.Build.Annotations[buildv1.BuildJenkinsConsoleLogURLAnnotation]) == 0 {
		return nil, fmt.Errorf("build %s does not contain a Jenkins Console URL annotation", t.BuildName)
	}
	_, err = url.Parse(t.Build.Annotations[buildv1.BuildJenkinsConsoleLogURLAnnotation])
	if err != nil {
		return nil, fmt.Errorf("cannot parse jenkins console log URL (%s): %v", t.Build.Annotations[buildv1.BuildJenkinsConsoleLogURLAnnotation], err)
	}
	if len(t.Build.Annotations[buildv1.BuildJenkinsBlueOceanLogURLAnnotation]) == 0 {
		return nil, fmt.Errorf("build %s does not contain a Jenkins BlueOcean URL annotation", t.BuildName)
	}
	_, err = url.Parse(t.Build.Annotations[buildv1.BuildJenkinsBlueOceanLogURLAnnotation])
	if err != nil {
		return nil, fmt.Errorf("cannot parse jenkins log blueocean URL (%s): %v", t.Build.Annotations[buildv1.BuildJenkinsBlueOceanLogURLAnnotation], err)
	}
	return jenkinsLogURL, nil
}

func DumpLogs(oc *exutil.CLI, t *exutil.BuildResult) (string, error) {
	var err error
	if t.Build == nil {
		t.Build, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), t.BuildName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("cannot retrieve build %s: %v", t.BuildName, err)
		}
	}
	jenkinsLogURL, err := ProcessLogURLAnnotations(oc, t)
	if err != nil {
		return "", err
	}
	jenkinsRef := NewRef(oc)
	log, _, err := jenkinsRef.GetResource(jenkinsLogURL.Path)
	if err != nil {
		return "", fmt.Errorf("cannot get jenkins log: %v", err)
	}
	return log, nil
}

// CreateTempFile stores the specified data in a temp dir/temp file
// for the test who calls it
func CreateTempFile(data string) (string, error) {
	testDir, err := ioutil.TempDir(os.TempDir(), "test-files")
	if err != nil {
		return "", err
	}
	testFile, err := ioutil.TempFile(testDir, "test-file")
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(testFile.Name(), []byte(data), 0666); err != nil {
		return "", err
	}
	return testFile.Name(), nil
}
