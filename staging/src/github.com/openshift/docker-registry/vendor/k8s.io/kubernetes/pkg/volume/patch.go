package volume

import "k8s.io/kubernetes/pkg/api/v1"

// we carry a patch which allows us to reset this function selectively when we run openshift start, but *not*
// when we run `kube-controller-manager`.  We have it in this separate file to avoid conflicts during a rebase
var NewPersistentVolumeRecyclerPodTemplate func() *v1.Pod = newPersistentVolumeRecyclerPodTemplate
