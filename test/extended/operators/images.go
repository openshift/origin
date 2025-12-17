package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = Describe("[sig-arch] Managed cluster", func() {
	oc := exutil.NewCLIWithoutNamespace("operators")
	It("should ensure pods use downstream images from our release image with proper ImagePullPolicy [apigroup:config.openshift.io]", Label("Size:M"), func() {
		imagePullSecret, err := oc.KubeFramework().ClientSet.CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to get pull secret for cluster: %v", err)
		}

		// cache file to local temp location
		imagePullFile, err := ioutil.TempFile("", "image-pull-secret")
		if err != nil {
			e2e.Failf("unable to create a temporary file: %v", err)
		}
		defer os.Remove(imagePullFile.Name())

		// write the content
		imagePullSecretBytes := imagePullSecret.Data[".dockerconfigjson"]
		if _, err := imagePullFile.Write(imagePullSecretBytes); err != nil {
			e2e.Failf("unable to write pull secret to temp file: %v", err)
		}
		if err := imagePullFile.Close(); err != nil {
			e2e.Failf("unable to close file: %v", err)
		}

		// find out the current installed release info using the temp file
		out, _, err := oc.Run("adm", "release", "info").Args("--pullspecs", "-o", "json", "--registry-config", imagePullFile.Name()).Outputs()
		if err != nil {
			// TODO need to determine why release tests are not having access to read payload
			e2e.Logf("unable to read release payload with error: %v", err)
			return
		}
		releaseInfo := &ReleaseInfo{}
		if err := json.Unmarshal([]byte(out), &releaseInfo); err != nil {
			e2e.Failf("unable to decode release payload with error: %v", err)
		}
		e2e.Logf("Release Info image=%s", releaseInfo.Image)
		e2e.Logf("Release Info number of tags %v", len(releaseInfo.References.Spec.Tags))

		// valid images include the release image and all its tagged references
		validImages := sets.NewString()
		validImages.Insert(releaseInfo.Image)
		if releaseInfo.References == nil {
			e2e.Failf("no references found")
		}
		for i := range releaseInfo.References.Spec.Tags {
			tag := releaseInfo.References.Spec.Tags[i]
			if tag.From != nil && tag.From.Kind == "DockerImage" {
				validImages.Insert(tag.From.Name)
			}
		}

		// iterate over the references to find valid images
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		// skip containers that are known to be already succeeded when this test run and they always result
		// into error: cannot exec into a container in a completed pod; current phase is Succeeded.
		skipPodContainersNames := sets.NewString(
			"installer",
			"pruner",
		)

		// list of pods that use images not in the release payload
		invalidPodContainerImages := sets.NewString()
		invalidPodContainerImagePullPolicy := sets.NewString()
		invalidPodContainerDownstreamImages := sets.NewString()
		for i := range pods.Items {
			pod := pods.Items[i]
			if ignoredNamespace(pod.Namespace) {
				continue
			}
			containersToInspect := []v1.Container{}
			for j := range pod.Spec.InitContainers {
				if skipPodContainersNames.Has(pod.Spec.InitContainers[j].Name) {
					continue
				}
				containersToInspect = append(containersToInspect, pod.Spec.InitContainers[j])
			}
			for j := range pod.Spec.Containers {
				if skipPodContainersNames.Has(pod.Spec.Containers[j].Name) {
					continue
				}
				containersToInspect = append(containersToInspect, pod.Spec.Containers[j])
			}
			for j := range containersToInspect {
				container := containersToInspect[j]
				if !validImages.Has(container.Image) {
					invalidPodContainerImages.Insert(fmt.Sprintf("%s/%s/%s image=%s", pod.Namespace, pod.Name, container.Name, container.Image))
				}

				if container.ImagePullPolicy != v1.PullIfNotPresent {
					invalidPodContainerImagePullPolicy.Insert(fmt.Sprintf("%s/%s/%s imagePullPolicy=%s", pod.Namespace, pod.Name, container.Name, container.ImagePullPolicy))
				}
			}
			// check if the container's image from the downstream.
			for j := range pod.Spec.Containers {
				containerName := pod.Spec.Containers[j].Name
				commands := []string{
					"exec",
					pod.Name,
					"-c",
					containerName,
					"--",
					"cat",
					"/etc/redhat-release",
				}
				oc.SetNamespace(pod.Namespace)
				result, err := oc.AsAdmin().Run(commands...).Args().Output()
				if err != nil {
					e2e.Logf("unable to run command:%v with error: %v", commands, err)
					continue
				}
				e2e.Logf("Image release info: %s", result)
				if !strings.Contains(result, "Red Hat Enterprise Linux") {
					invalidPodContainerDownstreamImages.Insert(fmt.Sprintf("%s/%s invalid downstream image!", pod.Name, containerName))
				}
			}
		}
		// log for debugging output before we ultimately fail
		e2e.Logf("Pods found with invalid container images not present in release payload: %s", strings.Join(invalidPodContainerImages.List(), "\n"))
		e2e.Logf("Pods found with invalid container image pull policy not equal to IfNotPresent: %s", strings.Join(invalidPodContainerImagePullPolicy.List(), "\n"))
		e2e.Logf("Pods with invalid dowstream images: %s", strings.Join(invalidPodContainerDownstreamImages.List(), "\n"))
		if len(invalidPodContainerImages) > 0 {
			e2e.Failf("Pods found with invalid container images not present in release payload: %s", strings.Join(invalidPodContainerImages.List(), "\n"))
		}
		if len(invalidPodContainerImagePullPolicy) > 0 {
			e2e.Failf("Pods found with invalid container image pull policy not equal to IfNotPresent: %s", strings.Join(invalidPodContainerImagePullPolicy.List(), "\n"))
		}
		if len(invalidPodContainerDownstreamImages) > 0 {
			e2e.Failf("Pods with invalid dowstream images: %s", strings.Join(invalidPodContainerDownstreamImages.List(), "\n"))
		}
	})
})

// ignoredNamespace() returns true if the namespace is to be ignored by the test
func ignoredNamespace(namespace string) bool {
	// a pod in a namespace that begins with kube-* or openshift-* must come from our release payload
	// TODO components in openshift-operators may not come from our payload, may want to weaken restriction
	namespacePrefixes := sets.NewString("kube-", "openshift-")
	ignoredNamespacePrefixes := sets.NewString("openshift-marketplace", "openshift-must-gather-")
	for _, prefix := range namespacePrefixes.List() {
		if !strings.HasPrefix(namespace, prefix) {
			return true
		}
	}
	for _, prefix := range ignoredNamespacePrefixes.List() {
		if strings.HasPrefix(namespace, prefix) {
			return true
		}
	}
	return false

}
