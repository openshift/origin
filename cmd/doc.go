// Package cmd contains the executables for OpenShift 3.
package cmd

import (
	// for hyperkube
	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
	_ "k8s.io/kubernetes/cmd/cloud-controller-manager/app"
	_ "k8s.io/kubernetes/cmd/kube-apiserver/app"
	_ "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	_ "k8s.io/kubernetes/cmd/kube-proxy/app"
	_ "k8s.io/kubernetes/cmd/kube-scheduler/app"
	_ "k8s.io/kubernetes/cmd/kubelet/app"
	_ "k8s.io/kubernetes/pkg/kubectl/cmd"
)
