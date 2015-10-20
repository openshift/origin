package images

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/pkg/labels"
)

// GetImageLabels retrieves Docker labels from image from image repository name and
// image reference
func GetImageLabels(c client.ImageStreamImageInterface, imageRepoName, imageRef string) (map[string]string, error) {
	_, imageID, err := imagestreamimage.ParseNameAndID(imageRef)
	image, err := c.Get(imageRepoName, imageID)

	if err != nil {
		return map[string]string{}, err
	}
	return image.Image.DockerImageMetadata.Config.Labels, nil
}

// RunInPodContainer will run provided command in the specified pod container.
func RunInPodContainer(oc *exutil.CLI, selector labels.Selector, cmd []string) error {
	pods, err := exutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), selector, exutil.CheckPodIsRunningFunc, 1, 120*time.Second)
	if err != nil {
		return err
	}
	if len(pods) != 1 {
		return fmt.Errorf("got %d pods for selector %v, expected 1", len(pods), selector)
	}

	pod, err := oc.KubeREST().Pods(oc.Namespace()).Get(pods[0])
	if err != nil {
		return err
	}
	args := []string{pod.Name, "-c", pod.Spec.Containers[0].Name, "--"}
	args = append(args, cmd...)
	return oc.Run("exec").Args(args...).Execute()
}

// CheckPageContains makes a http request for an example application and checks
// that the result contains given string
func CheckPageContains(oc *exutil.CLI, endpoint, path, contents string) (bool, error) {
	address, err := exutil.GetEndpointAddress(oc, endpoint)
	if err != nil {
		return false, err
	}

	response, err := exutil.FetchURL(fmt.Sprintf("http://%s/%s", address, path), 60*time.Second)
	if err != nil {
		return false, err
	}
	return strings.Contains(response, contents), nil
}
