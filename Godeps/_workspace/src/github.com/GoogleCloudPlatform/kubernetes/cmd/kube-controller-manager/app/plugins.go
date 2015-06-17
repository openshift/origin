/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package app

import (
	// This file exists to force the desired plugin implementations to be linked.
	// This should probably be part of some configuration fed into the build for a
	// given binary target.

	//Cloud providers
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/aws"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/gce"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/mesos"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/openstack"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/ovirt"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/rackspace"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/cloudprovider/vagrant"

	// Volume plugins
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/host_path"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/nfs"
)

// ProbeRecyclableVolumePlugins collects all persistent volume plugins into an easy to use list.
func ProbeRecyclableVolumePlugins() []volume.VolumePlugin {
	allPlugins := []volume.VolumePlugin{}

	// The list of plugins to probe is decided by the kubelet binary, not
	// by dynamic linking or other "magic".  Plugins will be analyzed and
	// initialized later.
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins(defaultRecycler(60))...)
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins(defaultRecycler(300))...)
	return allPlugins
}

// the default recycler:
//		1. validates the /scrub directory exists
// 		2. creates a text file to be scrubbed
//		3. performs rm -rf on the directory
//		4. tests to see if the directory is empty
// the pod fails if the error code is returned
func defaultRecycler(timeoutInSeconds int64) *volume.RecyclableVolumeConfig {
	return &volume.RecyclableVolumeConfig{
		ImageName: "gcr.io/google_containers/busybox",
		Command:   []string{"/bin/sh"},
		Args:      []string{"-c", "test -e /scrub && echo $(date) > /scrub/trash.txt && rm -rf /scrub/* && test -z \"$(ls -A /scrub)\" || exit 1"},
		Timeout:   timeoutInSeconds,
	}
}
