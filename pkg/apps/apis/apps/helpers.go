package apps

import (
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

// DeploymentToPodLogOptions builds a PodLogOptions object out of a DeploymentLogOptions.
// Currently DeploymentLogOptions.Container and DeploymentLogOptions.Previous aren't used
// so they won't be copied to PodLogOptions.
//
// Note that Previous for PodLogOptions is different from Previous for DeploymentLogOptions
// so it shouldn't be included here.
func DeploymentToPodLogOptions(opts *DeploymentLogOptions) *kapi.PodLogOptions {
	return &kapi.PodLogOptions{
		Container:    opts.Container,
		Follow:       opts.Follow,
		SinceSeconds: opts.SinceSeconds,
		SinceTime:    opts.SinceTime,
		Timestamps:   opts.Timestamps,
		TailLines:    opts.TailLines,
		LimitBytes:   opts.LimitBytes,
	}
}
