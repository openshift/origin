package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/apis/batch"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/selection"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/util/wait"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/namer"
)

const pvPrefix = "pv-"

// WaitForOpenShiftNamespaceImageStreams waits for the standard set of imagestreams to be imported
func WaitForOpenShiftNamespaceImageStreams(oc *CLI) error {
	langs := []string{"ruby", "nodejs", "perl", "php", "python", "wildfly", "mysql", "postgresql", "mongodb", "jenkins"}
	scan := func() bool {
		for _, lang := range langs {
			is, err := oc.REST().ImageStreams("openshift").Get(lang)
			if err != nil {
				return false
			}
			for tag := range is.Spec.Tags {
				if _, ok := is.Status.Tags[tag]; !ok {
					return false
				}
			}
		}
		return true
	}

	success := false
	for i := 0; i < 10; i++ {
		success = scan()
		if success {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if success {
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
		_, err := oc.REST().ImageStreams("openshift").Get(lang)
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
	out, err := oc.Run("get").Args("is", "-n", "openshift", "--config", KubeConfigPath()).Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n  imagestreams in openshift namespace: \n%s\n", out)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n  error on getting imagestreams in openshift namespace: %+v\n", err)
	}
	out, err = oc.Run("get").Args("is").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n  imagestreams in dynamic test namespace: \n%s\n", out)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n  error on getting imagestreams in dynamic test namespace: %+v\n", err)
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

func DumpNamedBuildLogs(buildName string, oc *CLI) {
	buildOuput, err := oc.Run("logs").Args("-f", "build/"+buildName, "--timestamps").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  build logs for %s: %s\n\n", buildName, buildOuput)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on build logs for %s: %+v\n\n", buildName, err)
	}
}

// DumpBuildLogs will dump the latest build logs for a BuildConfig for debug purposes
func DumpBuildLogs(bc string, oc *CLI) {
	buildOuput, err := oc.Run("logs").Args("-f", "bc/"+bc, "--timestamps").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  build logs : %s\n\n", buildOuput)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on build logs %+v\n\n", err)
	}

	// if we suspect that we are filling up the registry file system, call ExamineDiskUsage / ExaminePodDiskUsage
	// also see if manipulations of the quota around /mnt/openshift-xfs-vol-dir exist in the extended test set up scripts
	ExamineDiskUsage()
	ExaminePodDiskUsage(oc)
}

// DumpDeploymentLogs will dump the latest deployment logs for a DeploymentConfig for debug purposes
func DumpDeploymentLogs(dc string, oc *CLI) {
	out, err := oc.Run("get").Args("pods", "-o", "json").Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n Pod JSON dump: \n%s\n\n", out)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n got error on Pod JSON dump: %+v\n\n", err)
	}
	// pod logs were proving redundant .. leaving code that dumps them commented out for now in case we want to pull back in
	if err == nil {
		b := []byte(out)
		var list kapi.PodList
		err = json.Unmarshal(b, &list)
		if err == nil {
			for _, pod := range list.Items {
				fmt.Fprintf(g.GinkgoWriter, "\n\n looking at pod %s to see if it is affiliated with %s \n\n", pod.ObjectMeta.Name, dc)
				if strings.Contains(pod.ObjectMeta.Name, dc) {
					podName := pod.ObjectMeta.Name
					fmt.Fprintf(g.GinkgoWriter, "\n\n Describing pod %s \n\n", podName)
					descOutput, err := oc.Run("describe").Args("pod/" + podName).Output()
					if err == nil {
						fmt.Fprintf(g.GinkgoWriter, "\n\n  description for pod %s : %s\n\n", podName, descOutput)
					} else {
						fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on describing %s:  %v\n\n", podName, err)
					}

					fmt.Fprintf(g.GinkgoWriter, "\n\n dumping logs for pod %s \n\n", podName)
					depOutput, err := oc.Run("logs").Args("pod/" + podName).Output()
					if err == nil {
						fmt.Fprintf(g.GinkgoWriter, "\n\n  logs for pod %s : %s\n\n", podName, depOutput)
					} else {
						fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on dep logs for %s:  %v\n\n", podName, err)
					}
				}
			}
		} else {
			fmt.Fprintf(g.GinkgoWriter, "\n\n got json unmarshal err: %v\n\n", err)
		}
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on get pods: %v\n\n", err)
	}
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
		var list kapi.PodList
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

// WriteObjectToFile writes the JSON representation of runtime.Object into a temporary
// file.
func WriteObjectToFile(obj runtime.Object, filename string) error {
	content, err := runtime.Encode(kapi.Codecs.LegacyCodec(registered.EnabledVersions()...), obj)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

// VarSubOnFile reads in srcFile, finds instances inf varToSub, changes it to var, and writes out to destFile
func VarSubOnFile(srcFile, destFile, varToSub, val string) error {
	srcData, err := ioutil.ReadFile(srcFile)
	if err == nil {
		srcString := string(srcData)
		srcString = strings.Replace(srcString, varToSub, val, -1) // -1 means unlimited replacements
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

type BuildResult struct {
	// BuildPath is a resource qualified name (e.g. "build/test-1").
	BuildPath string
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
	// BuildTimeout is true if there was a timeout waiting for the build to finish.
	BuildTimeout bool
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

	// if we suspect that we are filling up the registry file system, call ExamineDiskUsage / ExaminePodDiskUsage
	// also see if manipulations of the quota around /mnt/openshift-xfs-vol-dir exist in the extended test set up scripts
	ExamineDiskUsage()
	ExaminePodDiskUsage(t.oc)

	fmt.Fprintf(g.GinkgoWriter, "\n\n")
}

// Logs returns the logs associated with this build.
func (t *BuildResult) Logs() (string, error) {
	if t == nil || t.BuildPath == "" {
		return "", fmt.Errorf("Not enough information to retrieve logs for %#v", *t)
	}

	buildOuput, err := t.oc.Run("logs").Args("-f", t.BuildPath, "--timestamps").Output()
	if err != nil {
		return "", fmt.Errorf("Error retieving logs for %#v: %v", *t, err)
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

// StartBuildAndWait executes OC start-build with the specified arguments on an existing buildconfig.
// Note that start-build will be run with "-o=name" as a parameter when using this method.
// If no error is returned from this method, it means that the build attempted successfully, NOT that
// the build completed. For completion information, check the BuildResult object.
func StartBuildAndWait(oc *CLI, args ...string) (result *BuildResult, err error) {
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

	buildName := matches[1]

	fmt.Fprintf(g.GinkgoWriter, "Waiting for %s to complete\n", buildPath)
	err = WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName,
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
	)

	if result.Build == nil {
		// We only abort here if the build progress was unobservable. Only known cause would be severe, non-build related error in WaitForABuild.
		return result, fmt.Errorf("Severe error waiting for build: %v", err)
	}

	result.BuildAttempt = true
	result.BuildTimeout = !(result.BuildFailure || result.BuildSuccess)

	fmt.Fprintf(g.GinkgoWriter, "Done waiting for %s: %#v\n", buildPath, *result)
	return result, nil
}

// WaitForABuild waits for a Build object to match either isOK or isFailed conditions.
func WaitForABuild(c client.BuildInterface, name string, isOK, isFailed func(*buildapi.Build) bool) error {
	// wait 2 minutes for build to exist
	err := wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
		if _, err := c.Get(name); err != nil {
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
		list, err := c.List(kapi.ListOptions{FieldSelector: fields.Set{"name": name}.AsSelector()})
		if err != nil {
			return false, err
		}
		for i := range list.Items {
			if name == list.Items[i].Name && isOK(&list.Items[i]) {
				return true, nil
			}
			if name != list.Items[i].Name || isFailed(&list.Items[i]) {
				return false, fmt.Errorf("The build %q status is %q", name, list.Items[i].Status.Phase)
			}
		}
		return false, nil
	})
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

// WaitForBuilderAccount waits until the builder service account gets fully
// provisioned
func WaitForBuilderAccount(c kclient.ServiceAccountsInterface) error {
	waitFn := func() (bool, error) {
		sc, err := c.Get("builder")
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
		list, err := client.List(kapi.ListOptions{FieldSelector: fields.Set{"name": name}.AsSelector()})
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
		w, err := client.Watch(kapi.ListOptions{FieldSelector: fields.Set{"name": name}.AsSelector(), ResourceVersion: rv})
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
// Defaults to waiting for 60 seconds
func WaitForAnImageStreamTag(oc *CLI, namespace, name, tag string) error {
	return TimedWaitForAnImageStreamTag(oc, namespace, name, tag, time.Second*60)
}

// TimedWaitForAnImageStreamTag waits until an image stream with given name has non-empty history for given tag.
// Gives up waiting after the specified waitTimeout
func TimedWaitForAnImageStreamTag(oc *CLI, namespace, name, tag string, waitTimeout time.Duration) error {
	g.By(fmt.Sprintf("waiting for an is importer to import a tag %s into a stream %s", tag, name))
	start := time.Now()
	c := make(chan error)
	go func() {
		err := WaitForAnImageStream(
			oc.REST().ImageStreams(namespace),
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

// compareResourceControllerNames compares names of two resource controllers. It returns:
//  -1 if rc a is older than b
//   1 if rc a is newer than b
//   0 if their names are the same
func compareResourceControllerNames(a, b string) int {
	var reDeploymentConfigName = regexp.MustCompile(`^(.*)-(\d+)$`)
	am := reDeploymentConfigName.FindStringSubmatch(a)
	bm := reDeploymentConfigName.FindStringSubmatch(b)

	if len(am) == 0 || len(bm) == 0 {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	}

	aname, averstr := am[0], am[1]
	bname, bverstr := bm[0], bm[1]

	aver, _ := strconv.Atoi(averstr)
	bver, _ := strconv.Atoi(bverstr)

	switch {
	case aname < bname || (aname == bname && aver < bver):
		return -1
	case bname < aname || (bname == aname && bver < aver):
		return 1
	default:
		return 0
	}
}

// WaitForADeployment waits for a deployment to fulfill either isOK or isFailed.
// When isOK returns true, WaitForADeployment returns nil, when isFailed returns
// true, WaitForADeployment returns an error including the deployment status.
// WaitForADeployment waits for at most a certain timeout (non-configurable).
func WaitForADeployment(client kclient.ReplicationControllerInterface, name string, isOK, isFailed func(*kapi.ReplicationController) bool, oc *CLI) error {
	timeout := 15 * time.Minute

	// closing done signals that any pending operation should be aborted.
	done := make(chan struct{})
	defer close(done)

	// okOrFailed returns whether a replication controller matches either of
	// the predicates isOK or isFailed, and the associated error in case of
	// failure.
	okOrFailed := func(rc *kapi.ReplicationController) (err error, matched bool) {
		if isOK(rc) {
			return nil, true
		}
		if isFailed(rc) {
			return fmt.Errorf("The deployment %q status is %q", name, rc.Annotations[deployapi.DeploymentStatusAnnotation]), true
		}
		return nil, false
	}

	// waitForDeployment waits until okOrFailed returns true or the done
	// channel is closed.
	waitForDeployment := func() (err error, retry bool) {
		requirement, err := labels.NewRequirement(deployapi.DeploymentConfigAnnotation, selection.Equals, sets.NewString(name))
		if err != nil {
			return fmt.Errorf("unexpected error generating label selector: %v", err), false
		}
		list, err := client.List(kapi.ListOptions{LabelSelector: labels.NewSelector().Add(*requirement)})
		if err != nil {
			return err, false
		}
		// multiple deployments are conceivable; so we look to see how the latest depoy does
		var lastRC *kapi.ReplicationController
		for _, rc := range list.Items {
			if lastRC == nil {
				lastRC = &rc
				continue
			}
			if compareResourceControllerNames(lastRC.GetName(), rc.GetName()) <= 0 {
				lastRC = &rc
			}
		}

		if lastRC != nil {
			err, matched := okOrFailed(lastRC)
			if matched {
				return err, false
			}
		}

		w, err := client.Watch(kapi.ListOptions{LabelSelector: labels.NewSelector().Add(*requirement), ResourceVersion: list.ResourceVersion})
		if err != nil {
			return err, false
		}
		defer w.Stop()
		for {
			select {
			case val, ok := <-w.ResultChan():
				if !ok {
					// watcher error, re-get and re-watch
					return nil, true
				}
				rc, ok := val.Object.(*kapi.ReplicationController)
				if !ok {
					continue
				}
				if lastRC == nil {
					lastRC = rc
				}
				// multiple deployments are conceivable; so we look to see how the latest deployment does
				if compareResourceControllerNames(lastRC.GetName(), rc.GetName()) <= 0 {
					lastRC = rc
					err, matched := okOrFailed(rc)
					if matched {
						return err, false
					}
				}
			case <-done:
				// no more time left, stop what we were doing,
				// do no retry.
				return nil, false
			}
		}
	}

	// errCh is buffered so the goroutine below never blocks on sending,
	// preventing a goroutine leak if we reach the timeout.
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		err, retry := waitForDeployment()
		for retry {
			err, retry = waitForDeployment()
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			DumpDeploymentLogs(name, oc)
		}
		return err
	case <-time.After(timeout):
		DumpDeploymentLogs(name, oc)
		// end for timing issues where we miss watch updates
		return fmt.Errorf("timed out waiting for deployment %q after %v", name, timeout)
	}
}

// WaitForADeploymentToComplete waits for a deployment to complete.
func WaitForADeploymentToComplete(client kclient.ReplicationControllerInterface, name string, oc *CLI) error {
	return WaitForADeployment(client, name, CheckDeploymentCompletedFn, CheckDeploymentFailedFn, oc)
}

// WaitForRegistry waits until a newly deployed registry becomes ready. If waitForDCVersion is given, the
// function will wait until a corresponding replica controller completes. If not give, the latest version of
// registry's deployment config will be fetched from etcd.
func WaitForRegistry(
	dcNamespacer client.DeploymentConfigsNamespacer,
	kubeClient kclient.Interface,
	waitForDCVersion *int64,
	oc *CLI,
) error {
	var latestVersion int64
	start := time.Now()

	if waitForDCVersion != nil {
		latestVersion = *waitForDCVersion
	} else {
		dc, err := dcNamespacer.DeploymentConfigs(kapi.NamespaceDefault).Get("docker-registry")
		if err != nil {
			return err
		}
		latestVersion = dc.Status.LatestVersion
	}
	fmt.Fprintf(g.GinkgoWriter, "waiting for deployment of version %d to complete\n", latestVersion)

	err := WaitForADeployment(kubeClient.ReplicationControllers(kapi.NamespaceDefault), "docker-registry",
		func(rc *kapi.ReplicationController) bool {
			if !CheckDeploymentCompletedFn(rc) {
				return false
			}
			v, err := strconv.ParseInt(rc.Annotations[deployapi.DeploymentVersionAnnotation], 10, 64)
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "failed to parse %q of replication controller %q: %v\n", deployapi.DeploymentVersionAnnotation, rc.Name, err)
				return false
			}
			return v >= latestVersion
		},
		func(rc *kapi.ReplicationController) bool {
			v, err := strconv.ParseInt(rc.Annotations[deployapi.DeploymentVersionAnnotation], 10, 64)
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "failed to parse %q of replication controller %q: %v\n", deployapi.DeploymentVersionAnnotation, rc.Name, err)
				return false
			}
			if v < latestVersion {
				return false
			}
			return CheckDeploymentFailedFn(rc)
		}, oc)
	if err != nil {
		return err
	}

	requirement, err := labels.NewRequirement(deployapi.DeploymentLabel, selection.Equals, sets.NewString(fmt.Sprintf("docker-registry-%d", latestVersion)))
	pods, err := WaitForPods(kubeClient.Pods(kapi.NamespaceDefault), labels.NewSelector().Add(*requirement), CheckPodIsReadyFn, 1, time.Minute)
	now := time.Now()
	fmt.Fprintf(g.GinkgoWriter, "deployed registry pod %s after %s\n", pods[0], now.Sub(start).String())
	return err
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
	client kclient.ResourceQuotaInterface,
	name string,
	expectedUsage kapi.ResourceList,
	expectedIsUpperLimit bool,
	timeout time.Duration,
) (kapi.ResourceList, error) {

	startTime := time.Now()
	endTime := startTime.Add(timeout)

	expectedResourceNames := quota.ResourceNames(expectedUsage)

	list, err := client.List(kapi.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector()})
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
	w, err := client.Watch(kapi.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector(), ResourceVersion: rv})
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

// CheckDeploymentCompletedFn returns true if the deployment completed
var CheckDeploymentCompletedFn = func(d *kapi.ReplicationController) bool {
	return deployutil.IsCompleteDeployment(d)
}

// CheckDeploymentFailedFn returns true if the deployment failed
var CheckDeploymentFailedFn = func(d *kapi.ReplicationController) bool {
	return deployutil.IsFailedDeployment(d)
}

// GetPodNamesByFilter looks up pods that satisfy the predicate and returns their names.
func GetPodNamesByFilter(c kclient.PodInterface, label labels.Selector, predicate func(kapi.Pod) bool) (podNames []string, err error) {
	podList, err := c.List(kapi.ListOptions{LabelSelector: label})
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

func WaitForAJob(c kclient.JobInterface, name string, timeout time.Duration) error {
	return wait.Poll(1*time.Second, timeout, func() (bool, error) {
		j, e := c.Get(name)
		if e != nil {
			return true, e
		}
		// TODO soltysh: replace this with a function once such exist, currently
		// it's private in the controller
		for _, c := range j.Status.Conditions {
			if (c.Type == batch.JobComplete || c.Type == batch.JobFailed) && c.Status == kapi.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

// WaitForPods waits until given number of pods that match the label selector and
// satisfy the predicate are found
func WaitForPods(c kclient.PodInterface, label labels.Selector, predicate func(kapi.Pod) bool, count int, timeout time.Duration) ([]string, error) {
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
var CheckPodIsRunningFn = func(pod kapi.Pod) bool {
	return pod.Status.Phase == kapi.PodRunning
}

// CheckPodIsSucceededFn returns true if the pod status is "Succdeded"
var CheckPodIsSucceededFn = func(pod kapi.Pod) bool {
	return pod.Status.Phase == kapi.PodSucceeded
}

// CheckPodIsReadyFn returns true if the pod's ready probe determined that the pod is ready.
var CheckPodIsReadyFn = func(pod kapi.Pod) bool {
	if pod.Status.Phase != kapi.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type != kapi.PodReady {
			continue
		}
		return cond.Status == kapi.ConditionTrue
	}
	return false
}

// WaitUntilPodIsGone waits until the named Pod will disappear
func WaitUntilPodIsGone(c kclient.PodInterface, podName string, timeout time.Duration) error {
	return wait.Poll(1*time.Second, timeout, func() (bool, error) {
		_, err := c.Get(podName)
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
	imageStream, err := c.Get(name)
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
func GetPodForContainer(container kapi.Container) *kapi.Pod {
	name := namer.GetPodName("test-pod", string(uuid.NewUUID()))
	return &kapi.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapi.PodSpec{
			Containers:    []kapi.Container{container},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
}

// CreatePersistentVolume creates a HostPath Persistent Volume.
func CreatePersistentVolume(name, capacity, hostPath string) *kapi.PersistentVolume {
	return &kapi.PersistentVolume{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapi.PersistentVolumeSpec{
			PersistentVolumeSource: kapi.PersistentVolumeSource{
				HostPath: &kapi.HostPathVolumeSource{
					Path: hostPath,
				},
			},
			Capacity: kapi.ResourceList{
				kapi.ResourceStorage: resource.MustParse(capacity),
			},
			AccessModes: []kapi.PersistentVolumeAccessMode{
				kapi.ReadWriteOnce,
				kapi.ReadOnlyMany,
				kapi.ReadWriteMany,
			},
		},
	}
}

// SetupHostPathVolumes will create multiple PersistentVolumes with given capacity
func SetupHostPathVolumes(c kclient.PersistentVolumeInterface, prefix, capacity string, count int) (volumes []*kapi.PersistentVolume, err error) {
	rootDir, err := ioutil.TempDir(TestContext.OutputDir, "persistent-volumes")
	if err != nil {
		return volumes, err
	}
	for i := 0; i < count; i++ {
		dir, err := ioutil.TempDir(rootDir, fmt.Sprintf("%0.4d", i))
		if err != nil {
			return volumes, err
		}
		if _, err = exec.LookPath("chcon"); err != nil {
			err := exec.Command("chcon", "-t", "svirt_sandbox_file_t", dir).Run()
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
func CleanupHostPathVolumes(c kclient.PersistentVolumeInterface, prefix string) error {
	pvs, err := c.List(kapi.ListOptions{})
	if err != nil {
		return err
	}
	prefix = fmt.Sprintf("%s%s-", pvPrefix, prefix)
	for _, pv := range pvs.Items {
		if !strings.HasPrefix(pv.Name, prefix) {
			continue
		}

		pvInfo, err := c.Get(pv.Name)
		if err != nil {
			fmt.Fprintf(g.GinkgoWriter, "WARNING: couldn't get meta info for PV %s: %v\n", pv.Name, err)
			continue
		}

		if err = c.Delete(pv.Name); err != nil {
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

// ExtendedTestPath returns absolute path to extended tests directory
func ExtendedTestPath() string {
	// can't use gomega in this method since it is used outside of It()
	return os.Getenv("EXTENDED_TEST_PATH")
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

// FixturePath returns absolute path to given fixture file
// The path is relative to EXTENDED_TEST_PATH (./test/extended/*)
func FixturePath(elem ...string) string {
	return filepath.Join(append([]string{ExtendedTestPath()}, elem...)...)
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
	err := oc.KubeFramework().WaitForAnEndpoint(name)
	if err != nil {
		return "", err
	}
	endpoint, err := oc.KubeREST().Endpoints(oc.Namespace()).Get(name)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", endpoint.Subsets[0].Addresses[0].IP, endpoint.Subsets[0].Ports[0].Port), nil
}

// GetPodForImage creates a new Pod that runs the containers from specified
// Docker image reference
func GetPodForImage(dockerImageReference string) *kapi.Pod {
	return GetPodForContainer(kapi.Container{
		Name:  "test",
		Image: dockerImageReference,
	})
}
