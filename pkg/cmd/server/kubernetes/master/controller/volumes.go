package controller

import (
	"k8s.io/kubernetes/pkg/api/v1"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/volume"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// newPersistentVolumeRecyclerPodTemplate provides a function which makes our recycler pod template for use in the kube-controller-manager
// this is a stop-gap until the kube-controller-manager take a pod manifest
func newPersistentVolumeRecyclerPodTemplate(recyclerImageName string) func() *v1.Pod {
	oldTemplateFunc := volume.NewPersistentVolumeRecyclerPodTemplate
	return func() *v1.Pod {
		uid := int64(0)
		defaultScrubPod := oldTemplateFunc()
		// TODO: Move the recycler pods to dedicated namespace instead of polluting openshift-infra.
		defaultScrubPod.Namespace = "openshift-infra"
		defaultScrubPod.Spec.ServiceAccountName = bootstrappolicy.InfraPersistentVolumeRecyclerControllerServiceAccountName
		defaultScrubPod.Spec.Containers[0].Image = recyclerImageName
		defaultScrubPod.Spec.Containers[0].Command = []string{"/usr/bin/openshift-recycle"}
		defaultScrubPod.Spec.Containers[0].Args = []string{"/scrub"}
		defaultScrubPod.Spec.Containers[0].SecurityContext = &kapiv1.SecurityContext{RunAsUser: &uid}
		defaultScrubPod.Spec.Containers[0].ImagePullPolicy = kapiv1.PullIfNotPresent

		return defaultScrubPod
	}
}
