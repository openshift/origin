package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/namer"
)

var TestContext e2e.TestContextType

const pvPrefix = "pv-"

// DumpBuildLogs will dump the latest build logs for a BuildConfig for debug purposes
func DumpBuildLogs(bc string, oc *CLI) {
	bldOuput, err := oc.Run("logs").Args("-f", "bc/"+bc).Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  build logs : %s\n\n", bldOuput)
	} else {
		fmt.Fprintf(g.GinkgoWriter, "\n\n  got error on bld logs %v\n\n", err)
	}

	// if we suspect that we are filling up the registry file syste, call ExamineDiskUsage / ExaminePodDiskUsage
	// also see if manipulations of the quota around /mnt/openshift-xfs-vol-dir exist in the extended test set up scripts
	//ExamineDiskUsage()
	//ExaminePodDiskUsage(oc)
}

// DumpDeploymentLogs will dump the latest deployment logs for a DeploymentConfig for debug purposes
func DumpDeploymentLogs(dc string, oc *CLI) {
	out, err := oc.Run("get").Args("pods", "-o", "json").Output()
	if err == nil {
		b := []byte(out)
		var list kapi.PodList
		err = json.Unmarshal(b, &list)
		if err == nil {
			for _, pod := range list.Items {
				fmt.Fprintf(g.GinkgoWriter, "\n\n looking at pod %s to see if it is affiliated with %s \n\n", pod.ObjectMeta.Name, dc)
				if strings.Contains(pod.ObjectMeta.Name, dc) {
					podName := pod.ObjectMeta.Name

					fmt.Fprintf(g.GinkgoWriter, "\n\n dumping logs for pod %s \n\n", podName)
					depOuput, err := oc.Run("logs").Args("-f", "pod/"+podName).Output()
					if err == nil {
						fmt.Fprintf(g.GinkgoWriter, "\n\n  logs for pod %s : %s\n\n", podName, depOuput)
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

// WaitForADeployment waits for a deployment to fulfill either isOK or isFailed.
// When isOK returns true, WaitForADeployment returns nil, when isFailed returns
// true, WaitForADeployment returns an error including the deployment status.
// WaitForADeployment waits for at most a certain timeout (non-configurable).
func WaitForADeployment(client kclient.ReplicationControllerInterface, name string, isOK, isFailed func(*kapi.ReplicationController) bool) error {
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
		requirement, err := labels.NewRequirement(deployapi.DeploymentConfigAnnotation, labels.EqualsOperator, sets.NewString(name))
		if err != nil {
			return fmt.Errorf("unexpected error generating label selector: %v", err), false
		}
		list, err := client.List(kapi.ListOptions{LabelSelector: labels.NewSelector().Add(*requirement)})
		if err != nil {
			return err, false
		}
		for _, rc := range list.Items {
			err, matched := okOrFailed(&rc)
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
				if rc, ok := val.Object.(*kapi.ReplicationController); ok {
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
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for deployment %q after %v", name, timeout)
	}
}

// WaitForADeploymentToComplete waits for a deployment to complete.
func WaitForADeploymentToComplete(client kclient.ReplicationControllerInterface, name string) error {
	return WaitForADeployment(client, name, CheckDeploymentCompletedFn, CheckDeploymentFailedFn)
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

func isLimitSynced(received, expected kapi.ResourceList) bool {
	resourceNames := quota.ResourceNames(expected)
	masked := quota.Mask(received, resourceNames)
	if len(masked) != len(expected) {
		return false
	}
	if le, _ := quota.LessThanOrEqual(masked, expected); !le {
		return false
	}
	if le, _ := quota.LessThanOrEqual(expected, masked); !le {
		return false
	}
	return true
}

// WaitForResourceQuotaSync watches given resource quota until its hard limit is updated to match the desired
// spec or timeout occurs.
func WaitForResourceQuotaLimitSync(
	client kclient.ResourceQuotaInterface,
	name string,
	hardLimit kapi.ResourceList,
	timeout time.Duration,
) error {

	startTime := time.Now()
	endTime := startTime.Add(timeout)

	expectedResourceNames := quota.ResourceNames(hardLimit)

	list, err := client.List(kapi.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector()})
	if err != nil {
		return err
	}

	for i := range list.Items {
		used := quota.Mask(list.Items[i].Status.Hard, expectedResourceNames)
		if isLimitSynced(used, hardLimit) {
			return nil
		}
	}

	rv := list.ResourceVersion
	w, err := client.Watch(kapi.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector(), ResourceVersion: rv})
	if err != nil {
		return err
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
				used := quota.Mask(rq.Status.Hard, expectedResourceNames)
				if isLimitSynced(used, hardLimit) {
					return nil
				}
			}
		case <-time.After(endTime.Sub(time.Now())):
			return wait.ErrWaitTimeout
		}
	}
	return wait.ErrWaitTimeout
}

// CheckDeploymentCompletedFn returns true if the deployment completed
var CheckDeploymentCompletedFn = func(d *kapi.ReplicationController) bool {
	return d.Annotations[deployapi.DeploymentStatusAnnotation] == string(deployapi.DeploymentStatusComplete)
}

// CheckDeploymentFailedFn returns true if the deployment failed
var CheckDeploymentFailedFn = func(d *kapi.ReplicationController) bool {
	return d.Annotations[deployapi.DeploymentStatusAnnotation] == string(deployapi.DeploymentStatusFailed)
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
	name := namer.GetPodName("test-pod", string(kutil.NewUUID()))
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
		if strings.HasPrefix(pv.Name, prefix) {
			c.Delete(pv.Name)
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
