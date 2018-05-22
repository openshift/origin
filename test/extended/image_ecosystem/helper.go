package image_ecosystem

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	g "github.com/onsi/ginkgo"

	exutil "github.com/openshift/origin/test/extended/util"
)

// RunInPodContainer will run provided command in the specified pod container.
func RunInPodContainer(oc *exutil.CLI, selector labels.Selector, cmd []string) error {
	pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), selector, exutil.CheckPodIsRunning, 1, 4*time.Minute)
	if err != nil {
		return err
	}
	if len(pods) != 1 {
		return fmt.Errorf("Got %d pods for selector %v, expected 1", len(pods), selector)
	}

	pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(pods[0], metav1.GetOptions{})
	if err != nil {
		return err
	}
	args := []string{pod.Name, "-c", pod.Spec.Containers[0].Name, "--"}
	args = append(args, cmd...)
	output, err := oc.Run("exec").Args(args...).Output()
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "RunInPodContainer exec output: %s\n", output)
	}
	return err
}

// CheckPageContains makes a http request for an example application and checks
// that the result contains given string
func CheckPageContains(oc *exutil.CLI, endpoint, path, contents string) (bool, error) {
	address, err := exutil.GetEndpointAddress(oc, endpoint)
	if err != nil {
		return false, err
	}

	response, err := exutil.FetchURL(fmt.Sprintf("http://%s/%s", address, path), 3*time.Minute)
	if err != nil {
		return false, err
	}
	success := strings.Contains(response, contents)
	if !success {
		fmt.Fprintf(g.GinkgoWriter, "CheckPageContains was looking for %s but got %s\n", contents, response)
	}
	return success, nil
}

// CheckPageRegexp makes a http request for an example application and checks
// that the result satisfies a given regexp; it will also return the submatch array entry
// present at index for possible comparisons
func CheckPageRegexp(oc *exutil.CLI, endpoint, path, regex string, index int) (bool, string, error) {
	address, err := exutil.GetEndpointAddress(oc, endpoint)
	if err != nil {
		return false, "", err
	}

	response, err := exutil.FetchURL(fmt.Sprintf("http://%s/%s", address, path), 3*time.Minute)
	if err != nil {
		return false, "", err
	}

	val := ""
	r, _ := regexp.Compile(regex)
	parts := r.FindStringSubmatch(response)
	success := len(parts) > 0
	if !success {
		fmt.Fprintf(g.GinkgoWriter, "CheckPageContains was looking for %s but got %s\n", regex, response)
	} else {
		for i, part := range parts {
			if i == index {
				val = part
			}
		}
	}
	return success, val, nil
}
