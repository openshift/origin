/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unversioned

import (
	"strconv"
	"time"

	"k8s.io/kubernetes/pkg/api"
)

// PodLogsNamespacer has methods to work with podLog subresources in a namespace
type PodLogsNamespacer interface {
	PodLogs(namespace string) PodLogsInterface
}

// PodLogsInterface has methods to work with podLog subresources.
type PodLogsInterface interface {
	Get(name string, opts *api.PodLogOptions) (*Request, error)
}

// podLogs implements PodLogsNamespacer interface
type podLogs struct {
	r  *Client
	ns string
}

// newPodLogs returns a podLogs
func newPodLogs(c *Client, namespace string) *podLogs {
	return &podLogs{
		r:  c,
		ns: namespace,
	}
}

// Get constructs a request for getting the logs for a pod
func (c *podLogs) Get(name string, opts *api.PodLogOptions) (*Request, error) {
	// TODO: Maybe use the queryparams converter for setting up url parameters
	req := c.r.Get().Namespace(c.ns).Name(name).Resource("pods").SubResource("log").
		Param("follow", strconv.FormatBool(opts.Follow)).
		Param("container", opts.Container).
		Param("previous", strconv.FormatBool(opts.Previous)).
		Param("timestamps", strconv.FormatBool(opts.Timestamps))

	if opts.SinceSeconds != nil {
		req.Param("sinceSeconds", strconv.FormatInt(*opts.SinceSeconds, 10))
	}
	if opts.SinceTime != nil {
		req.Param("sinceTime", opts.SinceTime.Format(time.RFC3339))
	}
	if opts.LimitBytes != nil {
		req.Param("limitBytes", strconv.FormatInt(*opts.LimitBytes, 10))
	}
	if opts.TailLines != nil {
		req.Param("tailLines", strconv.FormatInt(*opts.TailLines, 10))
	}
	return req, nil
}
