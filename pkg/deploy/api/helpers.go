package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// DeploymentToPodLogOptions builds a PodLogOptions object out of a DeploymentLogOptions.
// Currently DeploymentLogOptions.Container and DeploymentLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
func DeploymentToPodLogOptions(opts *DeploymentLogOptions) *kapi.PodLogOptions {
	return &kapi.PodLogOptions{
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}
