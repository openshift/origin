//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kube

import (
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/heketi/heketi/pkg/kubernetes"
)

type describable interface {
	String() string
}

// target contains attributes common to all target types.
type target struct {
	Namespace string
}

// TargetLabel represents resources in the cluster where the
// specified key and value parameters are equal. These can
// be used to determine a pod with a given label.
type TargetLabel struct {
	target
	Key   string
	Value string
}

// String describes the target label.
func (t TargetLabel) String() string {
	return fmt.Sprintf("label:%v=%v", t.Key, t.Value)
}

// GetTargetPod uses the target label's Key and Value parameters
// to determine a unique matching pod. If no pod with the
// matching label is found an error is returned. If more than one
// pod with the matching label is found an error is returned.
func (t TargetLabel) GetTargetPod(k *KubeConn) (TargetPod, error) {
	var (
		tp  TargetPod
		err error
		ns  = t.Namespace
	)
	if ns == "" {
		ns, err = kubernetes.GetNamespace()
		if err != nil {
			return tp, fmt.Errorf("Namespace must be provided in configuration: %v", err)
		}
	}
	tp.Namespace = ns
	tp.origin = t
	// Get a list of pods
	pods, err := k.kube.CoreV1().Pods(ns).List(v1.ListOptions{
		LabelSelector: t.Key + "==" + t.Value,
	})
	if err != nil {
		k.logger.Err(err)
		return tp, fmt.Errorf("Failed to get list of pods")
	}

	numPods := len(pods.Items)
	if numPods == 0 {
		// No pods found with that label
		err := fmt.Errorf("No pods with the label '%v=%v' were found",
			t.Key, t.Value)
		k.logger.Critical(err.Error())
		return tp, err

	} else if numPods > 1 {
		// There are more than one pod with the same label
		names := make([]string, numPods)
		for i, p := range pods.Items {
			names[i] = p.Name
		}
		err := fmt.Errorf("Found %v pods sharing the same label '%v=%v': %v",
			numPods, t.Key, t.Value, strings.Join(names, ", "))
		k.logger.Critical(err.Error())
		return tp, err
	}

	tp.PodName = pods.Items[0].ObjectMeta.Name
	return tp, nil
}

// TargetDaemonSet represents resources in the cluster where the
// member of a daemonset is running on the given host node.
// These can be used to determine a pod within a given daemonset.
type TargetDaemonSet struct {
	target
	Host     string
	Selector string
}

// String describes the target daemonset parameters.
func (t TargetDaemonSet) String() string {
	return fmt.Sprintf("host:%v selector:%v", t.Host, t.Selector)
}

// GetTargetPod uses the target's selector and host values
// to determine a unique matching pod. If no pod with the
// matching selector and host is found an error is returned.
// If more than one pod with the matching label is found
// an error is returned.
func (t TargetDaemonSet) GetTargetPod(k *KubeConn) (TargetPod, error) {
	var (
		tp  TargetPod
		err error
		ns  = t.Namespace
	)
	if ns == "" {
		ns, err = kubernetes.GetNamespace()
		if err != nil {
			return tp, fmt.Errorf("Namespace must be provided in configuration: %v", err)
		}
	}
	tp.Namespace = ns
	tp.origin = t

	// Get a list of pods
	pods, err := k.kube.CoreV1().Pods(ns).List(v1.ListOptions{
		LabelSelector: t.Selector,
	})
	if err != nil {
		k.logger.Err(err)
		return tp, k.logger.LogError("Failed to get list of pods")
	}

	// Go through the pods looking for the node
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == t.Host {
			tp.PodName = pod.ObjectMeta.Name
		}
	}
	if tp.PodName == "" {
		return tp, k.logger.LogError("Unable to find a GlusterFS pod on host %v "+
			"with a label key %v", t.Host, t.Selector)
	}
	return tp, nil
}

// TargetPod represents a single pod in the cluster.
type TargetPod struct {
	target
	PodName string

	// support for backtracking to original target
	origin describable
}

func (t TargetPod) resourceName() string {
	return "pods"
}

func (t TargetPod) String() string {
	s := fmt.Sprintf("pod:%v ns:%v", t.PodName, t.Namespace)
	if t.origin != nil {
		s = fmt.Sprintf("%s (from %s)", s, t.origin.String())
	}
	return s
}

// FirstContainer looks up the first container within the
// target pod. If the container name can not be determined
// this function will return an error.
func (t TargetPod) FirstContainer(k *KubeConn) (TargetContainer, error) {
	tc := TargetContainer{TargetPod: t}
	podSpec, err := k.kube.CoreV1().Pods(t.Namespace).Get(t.PodName, v1.GetOptions{})
	if err != nil {
		return tc, fmt.Errorf("Unable to get pod spec for %v: %v",
			t.PodName, err)
	}
	tc.ContainerName = podSpec.Spec.Containers[0].Name
	return tc, nil
}

// GetTargetPod returns the current target pod. This function
// exists to allow the TargetPod type to provide the same
// functional interface as the TargetLabel and TargetDaemonSet types.
func (t TargetPod) GetTargetPod(k *KubeConn) (TargetPod, error) {
	return t, nil
}

// TargetContainer represents a single container within a given
// pod in the cluster.
type TargetContainer struct {
	TargetPod
	ContainerName string
}

func (t TargetContainer) String() string {
	s := fmt.Sprintf("pod:%v c:%v ns:%v", t.PodName, t.ContainerName, t.Namespace)
	if t.origin != nil {
		s = fmt.Sprintf("%s (from %s)", s, t.origin.String())
	}
	return s
}
