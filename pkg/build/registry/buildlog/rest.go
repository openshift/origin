package buildlog

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/build"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	BuildRegistry build.Registry
	PodControl    PodControlInterface
}

type PodControlInterface interface {
	getPod(namespace, name string) (*kapi.Pod, error)
}

type RealPodControl struct {
	podsNamspacer kclient.PodsNamespacer
}

func (r RealPodControl) getPod(namespace, name string) (*kapi.Pod, error) {
	return r.podsNamspacer.Pods(namespace).Get(name)
}

// NewREST creates a new REST for BuildLog
// Takes build registry and pod client to get necessary attributes to assemble
// URL to which the request shall be redirected in order to get build logs.
func NewREST(b build.Registry, pn kclient.PodsNamespacer) apiserver.RESTStorage {
	return &REST{
		BuildRegistry: b,
		PodControl:    RealPodControl{pn},
	}
}

// Redirector implementation
func (r *REST) ResourceLocation(ctx kapi.Context, id string) (string, error) {
	build, err := r.BuildRegistry.GetBuild(ctx, id)
	if err != nil {
		return "", errors.NewFieldNotFound("Build", id)
	}

	// TODO: these must be status errors, not field errors
	// TODO: choose a more appropriate "try again later" status code, like 202
	if len(build.PodName) == 0 {
		return "", errors.NewFieldRequired("Build.PodName")
	}

	pod, err := r.PodControl.getPod(build.Namespace, build.PodName)
	if err != nil {
		return "", errors.NewFieldNotFound("Pod.Name", build.PodName)
	}

	buildPodID := build.PodName
	buildPodHost := pod.Status.Host
	buildPodNamespace := pod.Namespace
	// Build will take place only in one container
	buildContainerName := pod.Spec.Containers[0].Name

	location := fmt.Sprintf("%s:%d/containerLogs/%s/%s/%s", buildPodHost, kubernetes.NodePort, buildPodNamespace, buildPodID, buildContainerName)

	// Pod in which build take place can't be in the Pending or Unknown phase,
	// cause no containers are present in the Pod in those phases.
	if pod.Status.Phase == kapi.PodPending || pod.Status.Phase == kapi.PodUnknown {
		return "", errors.NewFieldInvalid("Pod.Status", pod.Status.Phase, "must be Running, Succeeded or Failed")
	}

	switch build.Status {
	case api.BuildStatusRunning:
		location += "?follow=1"
	case api.BuildStatusComplete, api.BuildStatusFailed:
		// Do not follow the Complete and Failed logs as the streaming already finished.
	default:
		return "", errors.NewFieldInvalid("build.Status", build.Status, "must be Running, Complete or Failed")
	}

	return location, nil
}

func (r *REST) New() runtime.Object {
	return &api.BuildLog{}
}
