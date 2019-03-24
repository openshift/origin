package operators

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"

	"github.com/openshift/origin/pkg/oc/cli/admin/release"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = Describe("[Feature:Platform][Smoke] Managed cluster", func() {
	oc := exutil.NewCLIWithoutNamespace("operators")

	It("should ensure pods use images from our release image with proper ImagePullPolicy", func() {
		// find out the current installed release info
		out, err := oc.Run("adm", "release", "info").Args("--pullspecs", "-o", "json").Output()
		if err != nil {
			e2e.Failf("unable to read release payload with error: %v", err)
		}
		releaseInfo := &release.ReleaseInfo{}
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
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		// list of pods that use images not in the release payload
		invalidPodContainerImages := sets.NewString()
		invalidPodContainerImagePullPolicy := sets.NewString()
		// a pod in a namespace that begins with kube-* or openshift-* must come from our release payload
		// TODO components in openshift-operators may not come from our payload, may want to weaken restriction
		namespacePrefixes := sets.NewString("kube-", "openshift-")
		for i := range pods.Items {
			pod := pods.Items[i]
			for _, prefix := range namespacePrefixes.List() {
				if !strings.HasPrefix(pod.Namespace, prefix) {
					continue
				}
				containersToInspect := []v1.Container{}
				for j := range pod.Spec.InitContainers {
					containersToInspect = append(containersToInspect, pod.Spec.InitContainers[j])
				}
				for j := range pod.Spec.Containers {
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
			}
		}
		// log for debugging output before we ultimately fail
		e2e.Logf("Pods found with invalid container images not present in release payload: %s", strings.Join(invalidPodContainerImages.List(), "\n"))
		e2e.Logf("Pods found with invalid container image pull policy not equal to IfNotPresent: %s", strings.Join(invalidPodContainerImagePullPolicy.List(), "\n"))
		if len(invalidPodContainerImages) > 0 {
			e2e.Failf("Pods found with invalid container images not present in release payload: %s", strings.Join(invalidPodContainerImages.List(), "\n"))
		}
		if len(invalidPodContainerImagePullPolicy) > 0 {
			e2e.Failf("Pods found with invalid container image pull policy not equal to IfNotPresent: %s", strings.Join(invalidPodContainerImagePullPolicy.List(), "\n"))
		}
	})
})
