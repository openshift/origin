package util

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/namer"
)

var TestContext e2e.TestContextType

// WriteObjectToFile writes the JSON representation of runtime.Object into a temporary
// file.
func WriteObjectToFile(obj runtime.Object, filename string) error {
	content, err := latest.Codec.Encode(obj)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

// WaitForABuild waits for a Build object to match either isOK or isFailed conditions
func WaitForABuild(c client.BuildInterface, name string, isOK, isFailed func(*buildapi.Build) bool) error {
	for {
		list, err := c.List(labels.Everything(), fields.Set{"name": name}.AsSelector())
		if err != nil {
			return err
		}
		for i := range list.Items {
			if name == list.Items[i].Name && isOK(&list.Items[i]) {
				return nil
			}
			if name != list.Items[i].Name || isFailed(&list.Items[i]) {
				return fmt.Errorf("The build %q status is %q", name, &list.Items[i].Status.Phase)
			}
		}

		rv := list.ResourceVersion
		w, err := c.Watch(labels.Everything(), fields.Set{"name": name}.AsSelector(), rv)
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
			if e, ok := val.Object.(*buildapi.Build); ok {
				if name == e.Name && isOK(e) {
					return nil
				}
				if name != e.Name || isFailed(e) {
					return fmt.Errorf("The build %q status is %q", name, e.Status.Phase)
				}
			}
		}
	}
}

// CheckBuildSuccessFunc returns true if the build succeeded
var CheckBuildSuccessFunc = func(b *buildapi.Build) bool {
	return b.Status.Phase == buildapi.BuildPhaseComplete
}

// CheckBuildFailedFunc return true if the build failed
var CheckBuildFailedFunc = func(b *buildapi.Build) bool {
	return b.Status.Phase == buildapi.BuildPhaseFailed || b.Status.Phase == buildapi.BuildPhaseError
}

// WaitForBuilderAccount waits until the builder service account gets fully
// provisioned
func WaitForBuilderAccount(c kclient.ServiceAccountsInterface) error {
	waitFunc := func() (bool, error) {
		sc, err := c.Get("builder")
		if err != nil {
			return false, err
		}
		for _, s := range sc.Secrets {
			if strings.Contains(s.Name, "dockercfg") {
				return true, nil
			}
		}
		return false, nil
	}
	return wait.Poll(time.Duration(100*time.Millisecond), time.Duration(60*time.Second), waitFunc)
}

// WaitForAnImageStream waits for an ImageStream to fulfill the isOK function
func WaitForAnImageStream(client client.ImageStreamInterface,
	name string,
	isOK, isFailed func(*imageapi.ImageStream) bool) error {
	for {
		list, err := client.List(labels.Everything(), fields.Set{"name": name}.AsSelector())
		if err != nil {
			return err
		}
		for i := range list.Items {
			if isOK(&list.Items[i]) {
				return nil
			}
			if isFailed(&list.Items[i]) {
				return fmt.Errorf("The deployment %q status is %q",
					name, list.Items[i].Annotations[imageapi.DockerImageRepositoryCheckAnnotation])
			}
		}

		rv := list.ResourceVersion
		w, err := client.Watch(labels.Everything(), fields.Set{"name": name}.AsSelector(), rv)
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

// CheckImageStreamLatestTagPopulatedFunc returns true if the imagestream has a ':latest' tag filed
var CheckImageStreamLatestTagPopulatedFunc = func(i *imageapi.ImageStream) bool {
	_, ok := i.Status.Tags["latest"]
	return ok
}

// CheckImageStreamTagNotFoundFunc return true if the imagestream update was not successful
var CheckImageStreamTagNotFoundFunc = func(i *imageapi.ImageStream) bool {
	return strings.Contains(i.Annotations[imageapi.DockerImageRepositoryCheckAnnotation], "not") ||
		strings.Contains(i.Annotations[imageapi.DockerImageRepositoryCheckAnnotation], "error")
}

// WaitForADeployment waits for a Deployment to fulfill the isOK function
func WaitForADeployment(client kclient.ReplicationControllerInterface,
	name string,
	isOK, isFailed func(*kapi.ReplicationController) bool) error {
	for {
		requirement, err := labels.NewRequirement(deployapi.DeploymentConfigAnnotation, labels.EqualsOperator, sets.NewString(name))
		if err != nil {
			return fmt.Errorf("unexpected error generating label selector: %v", err)
		}

		list, err := client.List(labels.LabelSelector{*requirement})
		if err != nil {
			return err
		}
		for i := range list.Items {
			if isOK(&list.Items[i]) {
				return nil
			}
			if isFailed(&list.Items[i]) {
				return fmt.Errorf("The deployment %q status is %q",
					name, list.Items[i].Annotations[deployapi.DeploymentStatusAnnotation])
			}
		}

		rv := list.ResourceVersion
		w, err := client.Watch(labels.LabelSelector{*requirement}, fields.Everything(), rv)
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
			if e, ok := val.Object.(*kapi.ReplicationController); ok {
				if isOK(e) {
					return nil
				}
				if isFailed(e) {
					return fmt.Errorf("The deployment %q status is %q",
						name, e.Annotations[deployapi.DeploymentStatusAnnotation])
				}
			}
		}
	}
}

// CheckDeploymentCompletedFunc returns true if the deployment completed
var CheckDeploymentCompletedFunc = func(d *kapi.ReplicationController) bool {
	return d.Annotations[deployapi.DeploymentStatusAnnotation] == string(deployapi.DeploymentStatusComplete)
}

// CheckDeploymentFailedFunc returns true if the deployment failed
var CheckDeploymentFailedFunc = func(d *kapi.ReplicationController) bool {
	return d.Annotations[deployapi.DeploymentStatusAnnotation] == string(deployapi.DeploymentStatusFailed)
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

// CreatePodForImage creates a Pod for the given image name. The dockerImageReference
// must be full docker pull spec.
func CreatePodForImage(dockerImageReference string) *kapi.Pod {
	podName := namer.GetPodName("test-pod", string(kutil.NewUUID()))
	return &kapi.Pod{
		TypeMeta: kapi.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"name": podName},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  podName,
					Image: dockerImageReference,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
}

// KubeConfigPath returns the value of KUBECONFIG environment variable
func KubeConfigPath() string {
	return os.Getenv("KUBECONFIG")
}

// ExtendedTestPath returns absolute path to extended tests directory
func ExtendedTestPath() string {
	return os.Getenv("EXTENDED_TEST_PATH")
}

// FixturePath returns absolute path to given fixture file
// The path is relative to EXTENDED_TEST_PATH (./test/extended/*)
func FixturePath(elem ...string) string {
	return filepath.Join(append([]string{ExtendedTestPath()}, elem...)...)
}

// FetchURL grabs the output from the specified url and returns it.
// It will retry once per second for duration retryTimeout if an error occurs during the request.
func FetchURL(url string, retryTimeout time.Duration) (response string, err error) {

	waitFunc := func() (bool, error) {
		r, err := http.Get(url)
		if err != nil {
			// lie to the poller that we didn't get an error even though we did
			// because otherwise it's going to give up.
			return false, nil
		}
		defer r.Body.Close()
		bytes, err := ioutil.ReadAll(r.Body)
		response = string(bytes)
		return true, nil
	}
	pollErr := wait.Poll(time.Duration(1*time.Second), retryTimeout, waitFunc)
	if pollErr != nil {
		return "", pollErr
	}
	return
}
