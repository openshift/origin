package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
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

// ScaleFromConfig builds a scale resource out of a deployment config.
func ScaleFromConfig(dc *DeploymentConfig) *extensions.Scale {
	return &extensions.Scale{
		ObjectMeta: kapi.ObjectMeta{
			Name:              dc.Name,
			Namespace:         dc.Namespace,
			CreationTimestamp: dc.CreationTimestamp,
		},
	}
}
