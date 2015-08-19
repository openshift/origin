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

package volume

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/mount"
)

// VolumeOptions contains option information about a volume.
//
// Currently, this struct containers only a single field for the
// rootcontext of the volume.  This is a temporary measure in order
// to set the rootContext of tmpfs mounts correctly; it will be replaced
// and expanded on by future SecurityContext work.
type VolumeOptions struct {
	// The rootcontext to use when performing mounts for a volume.
	RootContext string
}

// VolumePlugin is an interface to volume plugins that can be used on a
// kubernetes node (e.g. by kubelet) to instantiate and manage volumes.
type VolumePlugin interface {
	// Init initializes the plugin.  This will be called exactly once
	// before any New* calls are made - implementations of plugins may
	// depend on this.
	Init(host VolumeHost)

	// Name returns the plugin's name.  Plugins should use namespaced names
	// such as "example.com/volume".  The "kubernetes.io" namespace is
	// reserved for plugins which are bundled with kubernetes.
	Name() string

	// CanSupport tests whether the plugin supports a given volume
	// specification from the API.  The spec pointer should be considered
	// const.
	CanSupport(spec *Spec) bool

	// NewBuilder creates a new volume.Builder from an API specification.
	// Ownership of the spec pointer in *not* transferred.
	// - spec: The api.Volume spec
	// - pod: The enclosing pod
	NewBuilder(spec *Spec, podRef *api.Pod, opts VolumeOptions, mounter mount.Interface) (Builder, error)

	// NewCleaner creates a new volume.Cleaner from recoverable state.
	// - name: The volume name, as per the api.Volume spec.
	// - podUID: The UID of the enclosing pod
	NewCleaner(name string, podUID types.UID, mounter mount.Interface) (Cleaner, error)
}

// PersistentVolumePlugin is an extended interface of VolumePlugin and is used
// by volumes that want to provide long term persistence of data
type PersistentVolumePlugin interface {
	VolumePlugin
	// GetAccessModes describes the ways a given volume can be accessed/mounted.
	GetAccessModes() []api.PersistentVolumeAccessMode
}

// RecyclableVolumePlugin is an extended interface of VolumePlugin and is used
// by persistent volumes that want to be recycled before being made available again to new claims
type RecyclableVolumePlugin interface {
	VolumePlugin
	// NewRecycler creates a new volume.Recycler which knows how to reclaim this resource
	// after the volume's release from a PersistentVolumeClaim
	NewRecycler(spec *Spec) (Recycler, error)
}

// VolumeHost is an interface that plugins can use to access the kubelet.
type VolumeHost interface {
	// GetPluginDir returns the absolute path to a directory under which
	// a given plugin may store data.  This directory might not actually
	// exist on disk yet.  For plugin data that is per-pod, see
	// GetPodPluginDir().
	GetPluginDir(pluginName string) string

	// GetPodVolumeDir returns the absolute path a directory which
	// represents the named volume under the named plugin for the given
	// pod.  If the specified pod does not exist, the result of this call
	// might not exist.
	GetPodVolumeDir(podUID types.UID, pluginName string, volumeName string) string

	// GetPodPluginDir returns the absolute path to a directory under which
	// a given plugin may store data for a given pod.  If the specified pod
	// does not exist, the result of this call might not exist.  This
	// directory might not actually exist on disk yet.
	GetPodPluginDir(podUID types.UID, pluginName string) string

	// GetKubeClient returns a client interface
	GetKubeClient() client.Interface

	// NewWrapperBuilder finds an appropriate plugin with which to handle
	// the provided spec.  This is used to implement volume plugins which
	// "wrap" other plugins.  For example, the "secret" volume is
	// implemented in terms of the "emptyDir" volume.
	NewWrapperBuilder(spec *Spec, pod *api.Pod, opts VolumeOptions, mounter mount.Interface) (Builder, error)

	// NewWrapperCleaner finds an appropriate plugin with which to handle
	// the provided spec.  See comments on NewWrapperBuilder for more
	// context.
	NewWrapperCleaner(spec *Spec, podUID types.UID, mounter mount.Interface) (Cleaner, error)
}

// VolumePluginMgr tracks registered plugins.
type VolumePluginMgr struct {
	mutex   sync.Mutex
	plugins map[string]VolumePlugin
}

// Spec is an internal representation of a volume.  All API volume types translate to Spec.
type Spec struct {
	Name             string
	Volume           *api.Volume
	PersistentVolume *api.PersistentVolume
	ReadOnly         bool
}

// VolumeConfig contains any configuration item required by a plugin.  Config is passed to the volume plugin
// in the ProbeVolumePlugins func, which allows configuration by the binary hosting the volume plugins.
// Usage of this struct directly is possible but NewVolumeConfig is recommended because it will contain default values.
type VolumeConfig struct {
	// the default scrub pod is a template pod used by plugins when creating new Recyclers.
	// The plugin is required to change the pod's VolumeSource to match the volume of the plugin and set any
	// necessary information on the scrubber, such as path, server, or timeout.
	PersistentVolumeRecyclerDefaultScrubPod *api.Pod
	// the minimum ActiveDeadlineSeconds for an NFS scrubber pod
	PersistentVolumeRecyclerMinTimeoutNfs int64
	// the increment of time added per Gi to ActiveDeadlineSeconds for an NFS scrubber pod
	PersistentVolumeRecyclerTimeoutIncrementNfs int64
	// the minimum ActiveDeadlineSeconds for an HostPath scrubber pod
	PersistentVolumeRecyclerMinTimeoutHostPath int64
	// the increment of time added per Gi to ActiveDeadlineSeconds for an HostPath scrubber pod
	PersistentVolumeRecyclerTimeoutIncrementHostPath int64
}

// NewVolumeConfig creates a VolumeConfig with default values.
func NewVolumeConfig() *VolumeConfig {
	return &VolumeConfig{
		PersistentVolumeRecyclerDefaultScrubPod: createDefaultScrubberPodTemplate(),
	}
}

// NewSpecFromVolume creates an Spec from an api.Volume
func NewSpecFromVolume(vs *api.Volume) *Spec {
	return &Spec{
		Name:   vs.Name,
		Volume: vs,
	}
}

// NewSpecFromPersistentVolume creates an Spec from an api.PersistentVolume
func NewSpecFromPersistentVolume(pv *api.PersistentVolume, readOnly bool) *Spec {
	return &Spec{
		Name:             pv.Name,
		PersistentVolume: pv,
		ReadOnly:         readOnly,
	}
}

// InitPlugins initializes each plugin.  All plugins must have unique names.
// This must be called exactly once before any New* methods are called on any
// plugins.
func (pm *VolumePluginMgr) InitPlugins(plugins []VolumePlugin, host VolumeHost) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.plugins == nil {
		pm.plugins = map[string]VolumePlugin{}
	}

	allErrs := []error{}
	for _, plugin := range plugins {
		name := plugin.Name()
		if !util.IsQualifiedName(name) {
			allErrs = append(allErrs, fmt.Errorf("volume plugin has invalid name: %#v", plugin))
			continue
		}

		if _, found := pm.plugins[name]; found {
			allErrs = append(allErrs, fmt.Errorf("volume plugin %q was registered more than once", name))
			continue
		}
		plugin.Init(host)
		pm.plugins[name] = plugin
		glog.V(1).Infof("Loaded volume plugin %q", name)
	}
	return errors.NewAggregate(allErrs)
}

// FindPluginBySpec looks for a plugin that can support a given volume
// specification.  If no plugins can support or more than one plugin can
// support it, return error.
func (pm *VolumePluginMgr) FindPluginBySpec(spec *Spec) (VolumePlugin, error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	matches := []string{}
	for k, v := range pm.plugins {
		if v.CanSupport(spec) {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no volume plugin matched")
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple volume plugins matched: %s", strings.Join(matches, ","))
	}
	return pm.plugins[matches[0]], nil
}

// FindPluginByName fetches a plugin by name or by legacy name.  If no plugin
// is found, returns error.
func (pm *VolumePluginMgr) FindPluginByName(name string) (VolumePlugin, error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Once we can get rid of legacy names we can reduce this to a map lookup.
	matches := []string{}
	for k, v := range pm.plugins {
		if v.Name() == name {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no volume plugin matched")
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple volume plugins matched: %s", strings.Join(matches, ","))
	}
	return pm.plugins[matches[0]], nil
}

// FindPersistentPluginBySpec looks for a persistent volume plugin that can support a given volume
// specification.  If no plugin is found, return an error
func (pm *VolumePluginMgr) FindPersistentPluginBySpec(spec *Spec) (PersistentVolumePlugin, error) {
	volumePlugin, err := pm.FindPluginBySpec(spec)
	if err != nil {
		return nil, fmt.Errorf("Could not find volume plugin for spec: %+v", spec)
	}
	if persistentVolumePlugin, ok := volumePlugin.(PersistentVolumePlugin); ok {
		return persistentVolumePlugin, nil
	}
	return nil, fmt.Errorf("no persistent volume plugin matched")
}

// FindPersistentPluginByName fetches a persistent volume plugin by name.  If no plugin
// is found, returns error.
func (pm *VolumePluginMgr) FindPersistentPluginByName(name string) (PersistentVolumePlugin, error) {
	volumePlugin, err := pm.FindPluginByName(name)
	if err != nil {
		return nil, err
	}
	if persistentVolumePlugin, ok := volumePlugin.(PersistentVolumePlugin); ok {
		return persistentVolumePlugin, nil
	}
	return nil, fmt.Errorf("no persistent volume plugin matched: %+v")
}

// FindRecyclablePluginByName fetches a persistent volume plugin by name.  If no plugin
// is found, returns error.
func (pm *VolumePluginMgr) FindRecyclablePluginBySpec(spec *Spec) (RecyclableVolumePlugin, error) {
	volumePlugin, err := pm.FindPluginBySpec(spec)
	if err != nil {
		return nil, err
	}
	if recyclableVolumePlugin, ok := volumePlugin.(RecyclableVolumePlugin); ok {
		return recyclableVolumePlugin, nil
	}
	return nil, fmt.Errorf("no recyclable volume plugin matched")
}

// createDefaultScrubberPodTemplate creates a template for a scrubber pod.  Most attributes are correct for scrubbers,
// but other plugins are required to change the VolumeSource.  Plugins should also consider changing the pod's
// ActiveDeadlineSeconds timeout and GenerateName. See the NFS plugin and its overrides.
func createDefaultScrubberPodTemplate() *api.Pod {
	timeout := int64(60)
	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{
			GenerateName: "pv-scrubber-hostpath-",
			Namespace:    api.NamespaceDefault,
		},
		Spec: api.PodSpec{
			ActiveDeadlineSeconds: &timeout,
			RestartPolicy:         api.RestartPolicyNever,
			Volumes: []api.Volume{
				{
					Name: "vol",
					// IMPORTANT!  All plugins using this template must override the VolumeSource
					// and make it applicable to the PersistentVolume being recycled.
					// See HostPath and NFS implementations for examples.
					VolumeSource: api.VolumeSource{
						HostPath: &api.HostPathVolumeSource{"/thePathToScrub"},
					},
				},
			},
			Containers: []api.Container{
				{
					Name:    "scrubber",
					Image:   "gcr.io/google_containers/busybox",
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", "test -e /scrub && echo $(date) > /scrub/trash.txt && rm -rf /scrub/* && test -z \"$(ls -A /scrub)\" || exit 1"},
					VolumeMounts: []api.VolumeMount{
						{
							Name:      "vol",
							MountPath: "/scrub",
						},
					},
				},
			},
		},
	}
	return pod
}

func InitScrubPod(filePath string) (*api.Pod, error) {
	if filePath == "" {
		return nil, fmt.Errorf("PersistentVolume scrub file path not specified")
	}
	podDef, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Error reading PersistentVolume scrub file path %s: %+v", filePath, err)
	}
	if len(podDef) == 0 {
		return nil, fmt.Errorf("PersistentVolume scrub file was empty: %s", filePath)
	}
	pod := &api.Pod{}
	if err := latest.Codec.DecodeInto(podDef, pod); err != nil {
		return nil, fmt.Errorf("Error decoding PersistentVolume scrub file: %v", err)
	}
	return pod, nil
}
