package drae2e

import (
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

type DriverToolkitProbe struct {
	node      *corev1.Node
	clientset clientset.Interface
}

func (dtk DriverToolkitProbe) Verify() {

}
