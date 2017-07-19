package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	batchv1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kbatchclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/batch/v1"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	kinternalcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/test/e2e/framework"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/util/namer"
	"github.com/openshift/origin/test/extended/testdata"
)

const pvPrefix = "pv-"

// WaitForOpenShiftNamespaceImageStreams waits for the standard set of imagestreams to be imported
func WaitForOpenShiftNamespaceImageStreams(oc *CLI) error {
	langs := []string{"ruby", "nodejs", "perl", "php", "python", "wildfly", "mysql", "postgresql", "mongodb", "jenkins"}
	scan := func() bool {
		for _, lang := range langs {
			fmt.Fprintf(g.GinkgoWriter, "Checking language %v \n", lang)
			is, err := oc.Client().ImageStreams("openshift").Get(lang, metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "ImageStream Error: %#v \n", err)
				return false
			}
			for tag := range is.Spec.Tags {
				fmt.Fprintf(g.GinkgoWriter, "Checking tag %v \n", tag)
				if _, ok := is.Status.Tags[tag]; !ok {
					fmt.Fprintf(g.GinkgoWriter, "Tag Error: %#v \n", ok)
					return false
				}
			}
		}
		return true
	}

	success := false
	for i := 0; i < 10; i++ {
		fmt.Fprintf(g.GinkgoWriter, "Running scan #%v \n", i)
		success = scan()
		if success {
			break
		}
		fmt.Fprintf(g.GinkgoWriter, "Sleeping for 3 seconds \n")
		time.Sleep(3 * time.Second)
	}
	if success {
		fmt.Fprintf(g.GinkgoWriter, "Success! \n")
		return nil
	}
	DumpImageStreams(oc)
	return fmt.Errorf("Failed to import expected imagestreams")
}

// CheckOpenShiftNamespaceImageStreams is a temporary workaround for the intermittent
// issue seen in extended tests where *something* is deleteing the pre-loaded, languange
// imagestreams from the OpenShift namespace
func CheckOpenShiftNamespaceImageStreams(oc *CLI) {
	missing := false
	langs := []string{"ruby", "nodejs", "perl", "php", "python", "wildfly", "mysql", "postgresql", "mongodb", "jenkins"}
	for _, lang := range langs {
		_, err := oc.Client().ImageStreams("openshift").Get(lang, metav1.GetOptions{})
		if err != nil {
			missing = true
			break
		}
	}

	if missing {
		fmt.Fprint(g.GinkgoWriter, "\n\n openshift namespace image streams corrupted \n\n")
		DumpImageStreams(oc)
		out, err := oc.Run("get").Args("is", "-n", "openshift", "--config", KubeConfigPath()).Output()
		err = fmt.Errorf("something has tampered with the image streams in the openshift namespace; look at audits in master log; \n%s\n", out)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		fmt.Fprint(g.GinkgoWriter, "\n\n openshift namespace image streams OK \n\n")
	}

}

//DumpImageStreams will dump both the openshift namespace and local namespace imagestreams
// as part of debugging when the language imagestreams in the openshift namespace seem to disappear
func DumpImageStreams(oc *CLI) {
	out, err := oc.Run("get").Args("is", "-n", "openshift", "-o", "yaml", "--config", KubeConfigPath()).Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n  imagestreams in openshift namespace: \n%s\n", out)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n  error on getting imagestreams in openshift namespace: %+v\n%#v\n", err, out)
	}
	out, err = oc.Run("get").Args("is", "-o", "yaml").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n  imagestreams in dynamic test namespace: \n%s\n", out)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n  error on getting imagestreams in dynamic test namespace: %+v\n%#v\n", err, out)
	}
	ids, err := ListImages()
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "\n  got error on docker images %+v\n", err)
	} else {
		for _, id := range ids {
			fmt.Fprintf(g.GinkgoWriter, " found local image %s\n", id)
		}
	}
}

// DumpBuildLogs will dump the latest build logs for a BuildConfig for debug purposes
func DumpBuildLogs(bc string, oc *CLI) {
	buildOutput, err := oc.Run("logs").Args("-f", "bc/"+bc, "--timestamps").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  build logs : %s\n\n", buildOutput)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on build logs %+v\n\n", err)
	}

	// if we suspect that we are filling up the registry file system, call ExamineDiskUsage / ExaminePodDiskUsage
	// also see if manipulations of the quota around /mnt/openshift-xfs-vol-dir exist in the extended test set up scripts
	ExamineDiskUsage()
	ExaminePodDiskUsage(oc)
}

func GetDeploymentConfigPods(oc *CLI, dcName string, version int64) (*kapiv1.PodList, error) {
	return oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: ParseLabelsOrDie(fmt.Sprintf("%s=%s-%d", deployapi.DeployerPodForDeploymentLabel, dcName, version)).String()})
}

func GetApplicationPods(oc *CLI, dcName string) (*kapiv1.PodList, error) {
	return oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: ParseLabelsOrDie(fmt.Sprintf("deploymentconfig=%s", dcName)).String()})
}

// DumpDeploymentLogs will dump the latest deployment logs for a DeploymentConfig for debug purposes
func DumpDeploymentLogs(dcName string, version int64, oc *CLI) {
	fmt.Fprintf(g.GinkgoWriter, "Dumping deployment logs for deploymentconfig %q\n", dcName)

	pods, err := GetDeploymentConfigPods(oc, dcName, version)
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "Unable to retrieve pods for deploymentconfig %q: %v\n", dcName, err)
		return
	}

	DumpPodLogs(pods.Items, oc)
}

// DumpApplicationPodLogs will dump the latest application logs for a DeploymentConfig for debug purposes
func DumpApplicationPodLogs(dcName string, oc *CLI) {
	fmt.Fprintf(g.GinkgoWriter, "Dumping application logs for deploymentconfig %q\n", dcName)

	pods, err := GetApplicationPods(oc, dcName)
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "Unable to retrieve pods for deploymentconfig %q: %v\n", dcName, err)
		return
	}

	DumpPodLogs(pods.Items, oc)
}

func DumpPodLogs(pods []kapiv1.Pod, oc *CLI) {
	for _, pod := range pods {
		descOutput, err := oc.Run("describe").Args("pod/" + pod.Name).Output()
		if err == nil {
			fmt.Fprintf(g.GinkgoWriter, "Describing pod %q\n%s\n\n", pod.Name, descOutput)
		} else {
			fmt.Fprintf(g.GinkgoWriter, "Error retrieving description for pod %q: %v\n\n", pod.Name, err)
		}

		depOutput, err := oc.Run("logs").Args("pod/" + pod.Name).Output()
		if err == nil {
			fmt.Fprintf(g.GinkgoWriter, "Log for pod %q\n---->\n%s\n<----end of log for %[1]q\n", pod.Name, depOutput)
		} else {
			fmt.Fprintf(g.GinkgoWriter, "Error retrieving logs for pod %q: %v\n\n", pod.Name, err)
		}
	}

}

// GetMasterThreadDump will get a golang thread stack dump
func GetMasterThreadDump(oc *CLI) {
	out, err := oc.AsAdmin().Run("get").Args("--raw", "/debug/pprof/goroutine?debug=2").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n Master thread stack dump:\n\n%s\n\n", string(out))
		return
	}
	fmt.Fprintf(g.GinkgoWriter, "\n\n got error on oc get --raw /debug/pprof/goroutine?godebug=2: %v\n\n", err)
}

// ExamineDiskUsage will dump df output on the testing system; leveraging this as part of diagnosing
// the registry's disk filling up during external tests on jenkins
func ExamineDiskUsage() {
	out, err := exec.Command("/bin/df", "-m").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n df -m output: %s\n\n", string(out))
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n got error on df %v\n\n", err)
	}
	out, err = exec.Command("/bin/docker", "info").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n docker info output: \n%s\n\n", string(out))
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n got error on docker inspect %v\n\n", err)
	}
}

// ExaminePodDiskUsage will dump df/du output on registry pod; leveraging this as part of diagnosing
// the registry's disk filling up during external tests on jenkins
func ExaminePodDiskUsage(oc *CLI) {
	out, err := oc.Run("get").Args("pods", "-o", "json", "-n", "default", "--config", KubeConfigPath()).Output()
	var podName string
	if err == nil {
		b := []byte(out)
		var list kapiv1.PodList
		err = json.Unmarshal(b, &list)
		if err == nil {
			for _, pod := range list.Items {
				fmt.Fprintf(g.GinkgoWriter, "\n\n looking at pod %s \n\n", pod.ObjectMeta.Name)
				if strings.Contains(pod.ObjectMeta.Name, "docker-registry-") && !strings.Contains(pod.ObjectMeta.Name, "deploy") {
					podName = pod.ObjectMeta.Name
					break
				}
			}
		} else {
			fmt.Fprintf(g.GinkgoWriter, "\n\n got json unmarshal err: %v\n\n", err)
		}
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on get pods: %v\n\n", err)
	}
	if len(podName) == 0 {
		fmt.Fprintf(g.GinkgoWriter, "Unable to determine registry pod name, so we can't examine its disk usage.")
		return
	}

	out, err = oc.Run("exec").Args("-n", "default", podName, "df", "--config", KubeConfigPath()).Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n df from registry pod: \n%s\n\n", out)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n got error on reg pod df: %v\n", err)
	}
	out, err = oc.Run("exec").Args("-n", "default", podName, "du", "/registry", "--config", KubeConfigPath()).Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n du from registry pod: \n%s\n\n", out)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n got error on reg pod du: %v\n", err)
	}
}

// VarSubOnFile reads in srcFile, finds instances of ${key} from the map
// and replaces them with their associated values.
func VarSubOnFile(srcFile string, destFile string, vars map[string]string) error {
	srcData, err := ioutil.ReadFile(srcFile)
	if err == nil {
		srcString := string(srcData)
		for k, v := range vars {
			k = "${" + k + "}"
			srcString = strings.Replace(srcString, k, v, -1) // -1 means unlimited replacements
		}
		err = ioutil.WriteFile(destFile, []byte(srcString), 0644)
	}
	return err
}

// StartBuild executes OC start-build with the specified arguments. StdOut and StdErr from the process
// are returned as separate strings.
func StartBuild(oc *CLI, args ...string) (stdout, stderr string, err error) {
	stdout, stderr, err = oc.Run("start-build").Args(args...).Outputs()
	fmt.Fprintf(g.GinkgoWriter, "\n\nstart-build output with args %v:\nError>%v\nStdOut>\n%s\nStdErr>\n%s\n\n", args, err, stdout, stderr)
	return stdout, stderr, err
}

var buildPathPattern = regexp.MustCompile(`^build/([\w\-\._]+)$`)

type LogDumperFunc func(oc *CLI, br *BuildResult) (string, error)

func NewBuildResult(oc *CLI, build *buildapi.Build) *BuildResult {
	return &BuildResult{
		oc:        oc,
		BuildName: build.Name,
		BuildPath: "builds/" + build.Name,
	}
}

type BuildResult struct {
	// BuildPath is a resource qualified name (e.g. "build/test-1").
	BuildPath string
	// BuildName is the non-resource qualified name.
	BuildName string
	// StartBuildStdErr is the StdErr output generated by oc start-build.
	StartBuildStdErr string
	// StartBuildStdOut is the StdOut output generated by oc start-build.
	StartBuildStdOut string
	// StartBuildErr is the error, if any, returned by the direct invocation of the start-build command.
	StartBuildErr error
	// The buildconfig which generated this build.
	BuildConfigName string
	// Build is the resource created. May be nil if there was a timeout.
	Build *buildapi.Build
	// BuildAttempt represents that a Build resource was created.
	// false indicates a severe error unrelated to Build success or failure.
	BuildAttempt bool
	// BuildSuccess is true if the build was finshed successfully.
	BuildSuccess bool
	// BuildFailure is true if the build was finished with an error.
	BuildFailure bool
	// BuildCancelled is true if the build was canceled.
	BuildCancelled bool
	// BuildTimeout is true if there was a timeout waiting for the build to finish.
	BuildTimeout bool
	// Alternate log dumper function. If set, this is called instead of 'oc logs'
	LogDumper LogDumperFunc
	// The openshift client which created this build.
	oc *CLI
}

// DumpLogs sends logs associated with this BuildResult to the GinkgoWriter.
func (t *BuildResult) DumpLogs() {
	fmt.Fprintf(g.GinkgoWriter, "\n\n*****************************************\n")
	fmt.Fprintf(g.GinkgoWriter, "Dumping Build Result: %#v\n", *t)

	if t == nil {
		fmt.Fprintf(g.GinkgoWriter, "No build result available!\n\n")
		return
	}

	desc, err := t.oc.Run("describe").Args(t.BuildPath).Output()

	fmt.Fprintf(g.GinkgoWriter, "\n** Build Description:\n")
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "Error during description retrieval: %+v\n", err)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "%s\n", desc)
	}

	fmt.Fprintf(g.GinkgoWriter, "\n** Build Logs:\n")

	buildOuput, err := t.Logs()
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "Error during log retrieval: %+v\n", err)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "%s\n", buildOuput)
	}

	fmt.Fprintf(g.GinkgoWriter, "\n\n")

	t.dumpRegistryLogs()

	// if we suspect that we are filling up the registry file system, call ExamineDiskUsage / ExaminePodDiskUsage
	// also see if manipulations of the quota around /mnt/openshift-xfs-vol-dir exist in the extended test set up scripts
	/*
		ExamineDiskUsage()
		ExaminePodDiskUsage(t.oc)
		fmt.Fprintf(g.GinkgoWriter, "\n\n")
	*/
}

func (t *BuildResult) dumpRegistryLogs() {
	var buildStarted *time.Time
	oc := t.oc
	fmt.Fprintf(g.GinkgoWriter, "\n** Registry Logs:\n")

	if t.Build != nil && !t.Build.CreationTimestamp.IsZero() {
		buildStarted = &t.Build.CreationTimestamp.Time
	} else {
		proj, err := oc.Client().Projects().Get(oc.Namespace(), metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "Failed to get project %s: %v\n", oc.Namespace(), err)
		} else {
			buildStarted = &proj.CreationTimestamp.Time
		}
	}

	if buildStarted == nil {
		fmt.Fprintf(g.GinkgoWriter, "Could not determine test' start time\n\n\n")
		return
	}

	since := time.Now().Sub(*buildStarted)

	// Changing the namespace on the derived client still changes it on the original client
	// because the kubeFramework field is only copied by reference. Saving the original namespace
	// here so we can restore it when done with registry logs
	savedNamespace := t.oc.Namespace()
	oadm := t.oc.AsAdmin().SetNamespace("default")
	out, err := oadm.Run("logs").Args("dc/docker-registry", "--since="+since.String()).Output()
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "Error during log retrieval: %+v\n", err)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "%s\n", out)
	}
	t.oc.SetNamespace(savedNamespace)

	fmt.Fprintf(g.GinkgoWriter, "\n\n")
}

// Logs returns the logs associated with this build.
func (t *BuildResult) Logs() (string, error) {
	if t == nil || t.BuildPath == "" {
		return "", fmt.Errorf("Not enough information to retrieve logs for %#v", *t)
	}

	if t.LogDumper != nil {
		return t.LogDumper(t.oc, t)
	}

	buildOuput, err := t.oc.Run("logs").Args("-f", t.BuildPath, "--timestamps").Output()
	if err != nil {
		return "", fmt.Errorf("Error retrieving logs for %#v: %v", *t, err)
	}

	return buildOuput, nil
}

// Dumps logs and triggers a Ginkgo assertion if the build did NOT succeed.
func (t *BuildResult) AssertSuccess() *BuildResult {
	if !t.BuildSuccess {
		t.DumpLogs()
	}
	o.ExpectWithOffset(1, t.BuildSuccess).To(o.BeTrue())
	return t
}

// Dumps logs and triggers a Ginkgo assertion if the build did NOT have an error (this will not assert on timeouts)
func (t *BuildResult) AssertFailure() *BuildResult {
	if !t.BuildFailure {
		t.DumpLogs()
	}
	o.ExpectWithOffset(1, t.BuildFailure).To(o.BeTrue())
	return t
}

func StartBuildResult(oc *CLI, args ...string) (result *BuildResult, err error) {
	args = append(args, "-o=name") // ensure that the build name is the only thing send to stdout
	stdout, stderr, err := StartBuild(oc, args...)

	// Usually, with -o=name, we only expect the build path.
	// However, the caller may have added --follow which can add
	// content to stdout. So just grab the first line.
	buildPath := strings.TrimSpace(strings.Split(stdout, "\n")[0])

	result = &BuildResult{
		Build:            nil,
		BuildPath:        buildPath,
		StartBuildStdOut: stdout,
		StartBuildStdErr: stderr,
		StartBuildErr:    nil,
		BuildAttempt:     false,
		BuildSuccess:     false,
		BuildFailure:     false,
		BuildCancelled:   false,
		BuildTimeout:     false,
		oc:               oc,
	}

	// An error here does not necessarily mean we could not run start-build. For example
	// when --wait is specified, start-build returns an error if the build fails. Therefore,
	// we continue to collect build information even if we see an error.
	result.StartBuildErr = err

	matches := buildPathPattern.FindStringSubmatch(buildPath)
	if len(matches) != 2 {
		return result, fmt.Errorf("Build path output did not match expected format 'build/name' : %q", buildPath)
	}

	result.BuildName = matches[1]

	return result, nil
}

// StartBuildAndWait executes OC start-build with the specified arguments on an existing buildconfig.
// Note that start-build will be run with "-o=name" as a parameter when using this method.
// If no error is returned from this method, it means that the build attempted successfully, NOT that
// the build completed. For completion information, check the BuildResult object.
func StartBuildAndWait(oc *CLI, args ...string) (result *BuildResult, err error) {
	result, err = StartBuildResult(oc, args...)
	if err != nil {
		return result, err
	}
	return result, WaitForBuildResult(oc.Client().Builds(oc.Namespace()), result)
}

// WaitForBuildResult updates result wit the state of the build
func WaitForBuildResult(c client.BuildInterface, result *BuildResult) error {
	fmt.Fprintf(g.GinkgoWriter, "Waiting for %s to complete\n", result.BuildName)
	err := WaitForABuild(c, result.BuildName,
		func(b *buildapi.Build) bool {
			result.Build = b
			result.BuildSuccess = CheckBuildSuccessFn(b)
			return result.BuildSuccess
		},
		func(b *buildapi.Build) bool {
			result.Build = b
			result.BuildFailure = CheckBuildFailedFn(b)
			return result.BuildFailure
		},
		func(b *buildapi.Build) bool {
			result.Build = b
			result.BuildCancelled = CheckBuildCancelledFn(b)
			return result.BuildCancelled
		},
	)

	if result.Build == nil {
		// We only abort here if the build progress was unobservable. Only known cause would be severe, non-build related error in WaitForABuild.
		return fmt.Errorf("Severe error waiting for build: %v", err)
	}

	result.BuildAttempt = true
	result.BuildTimeout = !(result.BuildFailure || result.BuildSuccess || result.BuildCancelled)

	fmt.Fprintf(g.GinkgoWriter, "Done waiting for %s: %#v\n with error: %v\n", result.BuildName, *result, err)
	return nil
}

// WaitForABuild waits for a Build object to match either isOK or isFailed conditions.
func WaitForABuild(c client.BuildInterface, name string, isOK, isFailed, isCanceled func(*buildapi.Build) bool) error {
	if isOK == nil {
		isOK = CheckBuildSuccessFn
	}
	if isFailed == nil {
		isFailed = CheckBuildFailedFn
	}
	if isCanceled == nil {
		isCanceled = CheckBuildCancelledFn
	}

	// wait 2 minutes for build to exist
	err := wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
		if _, err := c.Get(name, metav1.GetOptions{}); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("Timed out waiting for build %q to be created", name)
	}
	if err != nil {
		return err
	}
	// wait longer for the build to run to completion
	err = wait.Poll(5*time.Second, 60*time.Minute, func() (bool, error) {
		list, err := c.List(metav1.ListOptions{FieldSelector: fields.Set{"name": name}.AsSelector().String()})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "error listing builds: %v", err)
			return false, err
		}
		for i := range list.Items {
			if name == list.Items[i].Name && (isOK(&list.Items[i]) || isCanceled(&list.Items[i])) {
				return true, nil
			}
			if name != list.Items[i].Name {
				return false, fmt.Errorf("While listing builds named %s, found unexpected build %#v", name, list.Items[i])
			}
			if isFailed(&list.Items[i]) {
				return false, fmt.Errorf("The build %q status is %q", name, list.Items[i].Status.Phase)
			}
		}
		return false, nil
	})
	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "WaitForABuild returning with error: %v", err)
	}
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("Timed out waiting for build %q to complete", name)
	}
	return err
}

// CheckBuildSuccessFn returns true if the build succeeded
var CheckBuildSuccessFn = func(b *buildapi.Build) bool {
	return b.Status.Phase == buildapi.BuildPhaseComplete
}

// CheckBuildFailedFn return true if the build failed
var CheckBuildFailedFn = func(b *buildapi.Build) bool {
	return b.Status.Phase == buildapi.BuildPhaseFailed || b.Status.Phase == buildapi.BuildPhaseError
}

// CheckBuildCancelledFn return true if the build was canceled
var CheckBuildCancelledFn = func(b *buildapi.Build) bool {
	return b.Status.Phase == buildapi.BuildPhaseCancelled
}

// WaitForBuilderAccount waits until the builder service account gets fully
// provisioned
func WaitForBuilderAccount(c kcoreclient.ServiceAccountInterface) error {
	waitFn := func() (bool, error) {
		sc, err := c.Get("builder", metav1.GetOptions{})
		if err != nil {
			// If we can't access the service accounts, let's wait till the controller
			// create it.
			if errors.IsForbidden(err) {
				return false, nil
			}
			return false, err
		}
		for _, s := range sc.Secrets {
			if strings.Contains(s.Name, "dockercfg") {
				return true, nil
			}
		}
		return false, nil
	}
	return wait.Poll(time.Duration(100*time.Millisecond), 1*time.Minute, waitFn)
}

// WaitForAnImageStream waits for an ImageStream to fulfill the isOK function
func WaitForAnImageStream(client client.ImageStreamInterface,
	name string,
	isOK, isFailed func(*imageapi.ImageStream) bool) error {
	for {
		list, err := client.List(metav1.ListOptions{FieldSelector: fields.Set{"name": name}.AsSelector().String()})
		if err != nil {
			return err
		}
		for i := range list.Items {
			if isOK(&list.Items[i]) {
				return nil
			}
			if isFailed(&list.Items[i]) {
				return fmt.Errorf("The image stream %q status is %q",
					name, list.Items[i].Annotations[imageapi.DockerImageRepositoryCheckAnnotation])
			}
		}

		rv := list.ResourceVersion
		w, err := client.Watch(metav1.ListOptions{FieldSelector: fields.Set{"name": name}.AsSelector().String(), ResourceVersion: rv})
		if err != nil {
			return err
		}
		defer w.Stop()

		for {
			val, ok := <-w.ResultChan()
			if !ok {
				// reget and re-watch
				break
			}
			if e, ok := val.Object.(*imageapi.ImageStream); ok {
				if isOK(e) {
					return nil
				}
				if isFailed(e) {
					return fmt.Errorf("The image stream %q status is %q",
						name, e.Annotations[imageapi.DockerImageRepositoryCheckAnnotation])
				}
			}
		}
	}
}

// WaitForAnImageStreamTag waits until an image stream with given name has non-empty history for given tag.
// Defaults to waiting for 300 seconds
func WaitForAnImageStreamTag(oc *CLI, namespace, name, tag string) error {
	return TimedWaitForAnImageStreamTag(oc, namespace, name, tag, time.Second*300)
}

// TimedWaitForAnImageStreamTag waits until an image stream with given name has non-empty history for given tag.
// Gives up waiting after the specified waitTimeout
func TimedWaitForAnImageStreamTag(oc *CLI, namespace, name, tag string, waitTimeout time.Duration) error {
	g.By(fmt.Sprintf("waiting for an is importer to import a tag %s into a stream %s", tag, name))
	start := time.Now()
	c := make(chan error)
	go func() {
		err := WaitForAnImageStream(
			oc.Client().ImageStreams(namespace),
			name,
			func(is *imageapi.ImageStream) bool {
				if history, exists := is.Status.Tags[tag]; !exists || len(history.Items) == 0 {
					return false
				}
				return true
			},
			func(is *imageapi.ImageStream) bool {
				return time.Now().After(start.Add(waitTimeout))
			})
		c <- err
	}()

	select {
	case e := <-c:
		return e
	case <-time.After(waitTimeout):
		return fmt.Errorf("timed out while waiting of an image stream tag %s/%s:%s", namespace, name, tag)
	}
}

// CheckImageStreamLatestTagPopulatedFn returns true if the imagestream has a ':latest' tag filed
var CheckImageStreamLatestTagPopulatedFn = func(i *imageapi.ImageStream) bool {
	_, ok := i.Status.Tags["latest"]
	return ok
}

// CheckImageStreamTagNotFoundFn return true if the imagestream update was not successful
var CheckImageStreamTagNotFoundFn = func(i *imageapi.ImageStream) bool {
	return strings.Contains(i.Annotations[imageapi.DockerImageRepositoryCheckAnnotation], "not") ||
		strings.Contains(i.Annotations[imageapi.DockerImageRepositoryCheckAnnotation], "error")
}

// WaitForDeploymentConfig waits for a DeploymentConfig to complete transition
// to a given version and report minimum availability.
func WaitForDeploymentConfig(kc kclientset.Interface, oc client.Interface, namespace, name string, version int64, cli *CLI) error {
	fmt.Fprintf(g.GinkgoWriter, "waiting for deploymentconfig %s/%s to be available with version %d\n", namespace, name, version)
	var dc *deployapi.DeploymentConfig

	start := time.Now()
	err := wait.Poll(time.Second, 15*time.Minute, func() (done bool, err error) {
		dc, err = oc.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// TODO re-enable this check once @mfojtik introduces a test that ensures we'll only ever get
		// exactly one deployment triggered.
		/*
			if dc.Status.LatestVersion > version {
				return false, fmt.Errorf("latestVersion %d passed %d", dc.Status.LatestVersion, version)
			}
		*/
		if dc.Status.LatestVersion < version {
			return false, nil
		}

		var progressing, available *deployapi.DeploymentCondition
		for i, condition := range dc.Status.Conditions {
			switch condition.Type {
			case deployapi.DeploymentProgressing:
				progressing = &dc.Status.Conditions[i]

			case deployapi.DeploymentAvailable:
				available = &dc.Status.Conditions[i]
			}
		}

		if progressing != nil && progressing.Status == kapi.ConditionFalse {
			return false, fmt.Errorf("not progressing")
		}

		if progressing != nil &&
			progressing.Status == kapi.ConditionTrue &&
			progressing.Reason == deployapi.NewRcAvailableReason &&
			available != nil &&
			available.Status == kapi.ConditionTrue {
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		fmt.Fprintf(g.GinkgoWriter, "got error %q when waiting for deploymentconfig %s/%s to be available with version %d\n", err, namespace, name, version)
		cli.Run("get").Args("dc", dc.Name, "-o", "yaml").Execute()

		DumpDeploymentLogs(name, version, cli)
		DumpApplicationPodLogs(name, cli)

		return err
	}

	requirement, err := labels.NewRequirement(deployapi.DeploymentLabel, selection.Equals, []string{deployutil.LatestDeploymentNameForConfig(dc)})
	if err != nil {
		return err
	}

	podnames, err := GetPodNamesByFilter(kc.CoreV1().Pods(namespace), labels.NewSelector().Add(*requirement), func(kapiv1.Pod) bool { return true })
	if err != nil {
		return err
	}

	fmt.Fprintf(g.GinkgoWriter, "deploymentconfig %s/%s available after %s\npods: %s\n", namespace, name, time.Now().Sub(start), strings.Join(podnames, ", "))

	return nil
}

func isUsageSynced(received, expected kapi.ResourceList, expectedIsUpperLimit bool) bool {
	resourceNames := quota.ResourceNames(expected)
	masked := quota.Mask(received, resourceNames)
	if len(masked) != len(expected) {
		return false
	}
	if expectedIsUpperLimit {
		if le, _ := quota.LessThanOrEqual(masked, expected); !le {
			return false
		}
	} else {
		if le, _ := quota.LessThanOrEqual(expected, masked); !le {
			return false
		}
	}
	return true
}

// WaitForResourceQuotaSync watches given resource quota until its usage is updated to desired level or a
// timeout occurs. If successful, used quota values will be returned for expected resources. Otherwise an
// ErrWaitTimeout will be returned. If expectedIsUpperLimit is true, given expected usage must compare greater
// or equal to quota's usage, which is useful for expected usage increment. Otherwise expected usage must
// compare lower or equal to quota's usage, which is useful for expected usage decrement.
func WaitForResourceQuotaSync(
	client kinternalcoreclient.ResourceQuotaInterface,
	name string,
	expectedUsage kapi.ResourceList,
	expectedIsUpperLimit bool,
	timeout time.Duration,
) (kapi.ResourceList, error) {

	startTime := time.Now()
	endTime := startTime.Add(timeout)

	expectedResourceNames := quota.ResourceNames(expectedUsage)

	list, err := client.List(metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String()})
	if err != nil {
		return nil, err
	}

	for i := range list.Items {
		used := quota.Mask(list.Items[i].Status.Used, expectedResourceNames)
		if isUsageSynced(used, expectedUsage, expectedIsUpperLimit) {
			return used, nil
		}
	}

	rv := list.ResourceVersion
	w, err := client.Watch(metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String(), ResourceVersion: rv})
	if err != nil {
		return nil, err
	}
	defer w.Stop()

	for time.Now().Before(endTime) {
		select {
		case val, ok := <-w.ResultChan():
			if !ok {
				// reget and re-watch
				continue
			}
			if rq, ok := val.Object.(*kapi.ResourceQuota); ok {
				used := quota.Mask(rq.Status.Used, expectedResourceNames)
				if isUsageSynced(used, expectedUsage, expectedIsUpperLimit) {
					return used, nil
				}
			}
		case <-time.After(endTime.Sub(time.Now())):
			return nil, wait.ErrWaitTimeout
		}
	}
	return nil, wait.ErrWaitTimeout
}

// GetPodNamesByFilter looks up pods that satisfy the predicate and returns their names.
func GetPodNamesByFilter(c kcoreclient.PodInterface, label labels.Selector, predicate func(kapiv1.Pod) bool) (podNames []string, err error) {
	podList, err := c.List(metav1.ListOptions{LabelSelector: label.String()})
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		if predicate(pod) {
			podNames = append(podNames, pod.Name)
		}
	}
	return podNames, nil
}

func WaitForAJob(c kbatchclient.JobInterface, name string, timeout time.Duration) error {
	return wait.Poll(1*time.Second, timeout, func() (bool, error) {
		j, e := c.Get(name, metav1.GetOptions{})
		if e != nil {
			return true, e
		}
		// TODO soltysh: replace this with a function once such exist, currently
		// it's private in the controller
		for _, c := range j.Status.Conditions {
			if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == kapiv1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

// WaitForPods waits until given number of pods that match the label selector and
// satisfy the predicate are found
func WaitForPods(c kcoreclient.PodInterface, label labels.Selector, predicate func(kapiv1.Pod) bool, count int, timeout time.Duration) ([]string, error) {
	var podNames []string
	err := wait.Poll(1*time.Second, timeout, func() (bool, error) {
		p, e := GetPodNamesByFilter(c, label, predicate)
		if e != nil {
			return true, e
		}
		if len(p) != count {
			return false, nil
		}
		podNames = p
		return true, nil
	})
	return podNames, err
}

// CheckPodIsRunningFn returns true if the pod is running
var CheckPodIsRunningFn = func(pod kapiv1.Pod) bool {
	return pod.Status.Phase == kapiv1.PodRunning
}

// CheckPodIsSucceededFn returns true if the pod status is "Succdeded"
var CheckPodIsSucceededFn = func(pod kapiv1.Pod) bool {
	return pod.Status.Phase == kapiv1.PodSucceeded
}

// CheckPodIsReadyFn returns true if the pod's ready probe determined that the pod is ready.
var CheckPodIsReadyFn = func(pod kapiv1.Pod) bool {
	if pod.Status.Phase != kapiv1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type != kapiv1.PodReady {
			continue
		}
		return cond.Status == kapiv1.ConditionTrue
	}
	return false
}

// WaitUntilPodIsGone waits until the named Pod will disappear
func WaitUntilPodIsGone(c kcoreclient.PodInterface, podName string, timeout time.Duration) error {
	return wait.Poll(1*time.Second, timeout, func() (bool, error) {
		_, err := c.Get(podName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
}

// GetDockerImageReference retrieves the full Docker pull spec from the given ImageStream
// and tag
func GetDockerImageReference(c client.ImageStreamInterface, name, tag string) (string, error) {
	imageStream, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	isTag, ok := imageStream.Status.Tags[tag]
	if !ok {
		return "", fmt.Errorf("ImageStream %q does not have tag %q", name, tag)
	}
	if len(isTag.Items) == 0 {
		return "", fmt.Errorf("ImageStreamTag %q is empty", tag)
	}
	return isTag.Items[0].DockerImageReference, nil
}

// GetPodForContainer creates a new Pod that runs specified container
func GetPodForContainer(container kapiv1.Container) *kapiv1.Pod {
	name := namer.GetPodName("test-pod", string(uuid.NewUUID()))
	return &kapiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapiv1.PodSpec{
			Containers:    []kapiv1.Container{container},
			RestartPolicy: kapiv1.RestartPolicyNever,
		},
	}
}

// CreatePersistentVolume creates a HostPath Persistent Volume.
func CreatePersistentVolume(name, capacity, hostPath string) *kapiv1.PersistentVolume {
	return &kapiv1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapiv1.PersistentVolumeSpec{
			PersistentVolumeSource: kapiv1.PersistentVolumeSource{
				HostPath: &kapiv1.HostPathVolumeSource{
					Path: hostPath,
				},
			},
			Capacity: kapiv1.ResourceList{
				kapiv1.ResourceStorage: resource.MustParse(capacity),
			},
			AccessModes: []kapiv1.PersistentVolumeAccessMode{
				kapiv1.ReadWriteOnce,
				kapiv1.ReadOnlyMany,
				kapiv1.ReadWriteMany,
			},
		},
	}
}

// SetupHostPathVolumes will create multiple PersistentVolumes with given capacity
func SetupHostPathVolumes(c kcoreclient.PersistentVolumeInterface, prefix, capacity string, count int) (volumes []*kapiv1.PersistentVolume, err error) {
	rootDir, err := ioutil.TempDir(TestContext.OutputDir, "persistent-volumes")
	if err != nil {
		return volumes, err
	}
	for i := 0; i < count; i++ {
		dir, err := ioutil.TempDir(rootDir, fmt.Sprintf("%0.4d", i))
		if err != nil {
			return volumes, err
		}
		if _, err = exec.LookPath("chcon"); err == nil {
			err := exec.Command("chcon", "-t", "container_file_t", dir).Run()
			if err != nil {
				return volumes, err
			}
		}
		if err = os.Chmod(dir, 0777); err != nil {
			return volumes, err
		}
		pv, err := c.Create(CreatePersistentVolume(fmt.Sprintf("%s%s-%0.4d", pvPrefix, prefix, i), capacity, dir))
		if err != nil {
			return volumes, err
		}
		volumes = append(volumes, pv)
	}
	return volumes, err
}

// CleanupHostPathVolumes removes all PersistentVolumes created by
// SetupHostPathVolumes, with a given prefix
func CleanupHostPathVolumes(c kcoreclient.PersistentVolumeInterface, prefix string) error {
	pvs, err := c.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	prefix = fmt.Sprintf("%s%s-", pvPrefix, prefix)
	for _, pv := range pvs.Items {
		if !strings.HasPrefix(pv.Name, prefix) {
			continue
		}

		pvInfo, err := c.Get(pv.Name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "WARNING: couldn't get meta info for PV %s: %v\n", pv.Name, err)
			continue
		}

		if err = c.Delete(pv.Name, nil); err != nil {
			fmt.Fprintf(g.GinkgoWriter, "WARNING: couldn't remove PV %s: %v\n", pv.Name, err)
			continue
		}

		volumeDir := pvInfo.Spec.HostPath.Path
		if err = os.RemoveAll(volumeDir); err != nil {
			fmt.Fprintf(g.GinkgoWriter, "WARNING: couldn't remove directory %q: %v\n", volumeDir, err)
			continue
		}

		parentDir := filepath.Dir(volumeDir)
		if parentDir == "." || parentDir == "/" {
			continue
		}

		if err = os.Remove(parentDir); err != nil {
			fmt.Fprintf(g.GinkgoWriter, "WARNING: couldn't remove directory %q: %v\n", parentDir, err)
			continue
		}
	}
	return nil
}

// KubeConfigPath returns the value of KUBECONFIG environment variable
func KubeConfigPath() string {
	// can't use gomega in this method since it is used outside of It()
	return os.Getenv("KUBECONFIG")
}

//ArtifactDirPath returns the value of ARTIFACT_DIR environment variable
func ArtifactDirPath() string {
	path := os.Getenv("ARTIFACT_DIR")
	o.Expect(path).NotTo(o.BeNil())
	o.Expect(path).NotTo(o.BeEmpty())
	return path
}

//ArtifactPath returns the absolute path to the fix artifact file
//The path is relative to ARTIFACT_DIR
func ArtifactPath(elem ...string) string {
	return filepath.Join(append([]string{ArtifactDirPath()}, elem...)...)
}

var (
	fixtureDirLock sync.Once
	fixtureDir     string
)

// FixturePath returns an absolute path to a fixture file in test/extended/testdata/,
// test/integration/, or examples/.
func FixturePath(elem ...string) string {
	switch {
	case len(elem) == 0:
		panic("must specify path")
	case len(elem) > 3 && elem[0] == ".." && elem[1] == ".." && elem[2] == "examples":
		elem = elem[2:]
	case len(elem) > 3 && elem[0] == ".." && elem[1] == "integration":
		elem = append([]string{"test"}, elem[1:]...)
	case elem[0] == "testdata":
		elem = append([]string{"test", "extended"}, elem...)
	default:
		panic(fmt.Sprintf("Fixtures must be in test/extended/testdata or examples not %s", path.Join(elem...)))
	}
	fixtureDirLock.Do(func() {
		dir, err := ioutil.TempDir("", "fixture-testdata-dir")
		if err != nil {
			panic(err)
		}
		fixtureDir = dir
	})
	relativePath := path.Join(elem...)
	fullPath := path.Join(fixtureDir, relativePath)
	if err := testdata.RestoreAsset(fixtureDir, relativePath); err != nil {
		if err := testdata.RestoreAssets(fixtureDir, relativePath); err != nil {
			panic(err)
		}
		if err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err := os.Chmod(path, 0640); err != nil {
				return err
			}
			if stat, err := os.Lstat(path); err == nil && stat.IsDir() {
				return os.Chmod(path, 0755)
			}
			return nil
		}); err != nil {
			panic(err)
		}
	} else {
		if err := os.Chmod(fullPath, 0640); err != nil {
			panic(err)
		}
	}

	p, err := filepath.Abs(fullPath)
	if err != nil {
		panic(err)
	}
	return p
}

// FetchURL grabs the output from the specified url and returns it.
// It will retry once per second for duration retryTimeout if an error occurs during the request.
func FetchURL(url string, retryTimeout time.Duration) (response string, err error) {
	waitFn := func() (bool, error) {
		r, err := http.Get(url)
		if err != nil || r.StatusCode != 200 {
			// lie to the poller that we didn't get an error even though we did
			// because otherwise it's going to give up.
			return false, nil
		}
		defer r.Body.Close()
		bytes, err := ioutil.ReadAll(r.Body)
		response = string(bytes)
		return true, nil
	}
	pollErr := wait.Poll(time.Duration(1*time.Second), retryTimeout, waitFn)
	if pollErr == wait.ErrWaitTimeout {
		return "", fmt.Errorf("Timed out while fetching url %q", url)
	}
	if pollErr != nil {
		return "", pollErr
	}
	return
}

// ParseLabelsOrDie turns the given string into a label selector or
// panics; for tests or other cases where you know the string is valid.
// TODO: Move this to the upstream labels package.
func ParseLabelsOrDie(str string) labels.Selector {
	ret, err := labels.Parse(str)
	if err != nil {
		panic(fmt.Sprintf("cannot parse '%v': %v", str, err))
	}
	return ret
}

// GetEndpointAddress will return an "ip:port" string for the endpoint.
func GetEndpointAddress(oc *CLI, name string) (string, error) {
	err := framework.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), name)
	if err != nil {
		return "", err
	}
	endpoint, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", endpoint.Subsets[0].Addresses[0].IP, endpoint.Subsets[0].Ports[0].Port), nil
}

// CreateExecPodOrFail creates a simple busybox pod in a sleep loop used as a
// vessel for kubectl exec commands.
// Returns the name of the created pod.
// TODO: expose upstream
func CreateExecPodOrFail(client kcoreclient.CoreV1Interface, ns, name string) string {
	framework.Logf("Creating new exec pod")
	execPod := framework.NewHostExecPodSpec(ns, name)
	created, err := client.Pods(ns).Create(execPod)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = wait.PollImmediate(framework.Poll, 5*time.Minute, func() (bool, error) {
		retrievedPod, err := client.Pods(execPod.Namespace).Get(created.Name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return retrievedPod.Status.Phase == kapiv1.PodRunning, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return created.Name
}

// CheckForBuildEvent will poll a build for up to 1 minute looking for an event with
// the specified reason and message template.
func CheckForBuildEvent(client kcoreclient.CoreV1Interface, build *buildapi.Build, reason, message string) {
	var expectedEvent *kapiv1.Event
	err := wait.PollImmediate(framework.Poll, 1*time.Minute, func() (bool, error) {
		events, err := client.Events(build.Namespace).Search(kapi.Scheme, build)
		if err != nil {
			return false, err
		}
		for _, event := range events.Items {
			framework.Logf("Found event %#v", event)
			if reason == event.Reason {
				expectedEvent = &event
				return true, nil
			}
		}
		return false, nil
	})
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred(), "Should be able to get events from the build")
	o.ExpectWithOffset(1, expectedEvent).NotTo(o.BeNil(), "Did not find a %q event on build %s/%s", reason, build.Namespace, build.Name)
	o.ExpectWithOffset(1, expectedEvent.Message).To(o.Equal(fmt.Sprintf(message, build.Namespace, build.Name)))
}

type podExecutor struct {
	client  *CLI
	podName string
}

// NewPodExecutor returns an executor capable of running commands in a Pod.
func NewPodExecutor(oc *CLI, name, image string) (*podExecutor, error) {
	out, err := oc.Run("run").Args(name, "--labels", "name="+name, "--image", image, "--restart", "Never", "--command", "--", "/bin/bash", "-c", "sleep infinity").Output()
	if err != nil {
		return nil, fmt.Errorf("error: %v\n(%s)", err, out)
	}
	_, err = WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), ParseLabelsOrDie("name="+name), CheckPodIsReadyFn, 1, 3*time.Minute)
	if err != nil {
		return nil, err
	}
	return &podExecutor{client: oc, podName: name}, nil
}

// Exec executes a single command or a bash script in the running pod. It returns the
// command output and error if the command finished with non-zero status code or the
// command took longer then 3 minutes to run.
func (r *podExecutor) Exec(script string) (string, error) {
	var out string
	waitErr := wait.PollImmediate(1*time.Second, 3*time.Minute, func() (bool, error) {
		var err error
		out, err = r.client.Run("exec").Args(r.podName, "--", "/bin/bash", "-c", script).Output()
		return true, err
	})
	return out, waitErr
}
