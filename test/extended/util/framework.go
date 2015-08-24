package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/util/namer"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e"
)

var TestContext e2e.TestContextType

// The build succeeded
var CheckBuildSuccessFunc = func(b *buildapi.Build) bool {
	return b.Status.Phase == buildapi.BuildPhaseComplete
}

// The build failed
var CheckBuildFailedFunc = func(b *buildapi.Build) bool {
	return b.Status.Phase == buildapi.BuildPhaseFailed || b.Status.Phase == buildapi.BuildPhaseError
}

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
	return wait.Poll(60, time.Duration(1*time.Second), waitFunc)
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
