package libpod

import (
	"fmt"
	"sync"

	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/kubernetes-incubator/cri-o/libpod/ctr"
	"github.com/kubernetes-incubator/cri-o/libpod/pod"
	"github.com/kubernetes-incubator/cri-o/server/apparmor"
	"github.com/kubernetes-incubator/cri-o/server/seccomp"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/ulule/deepcopier"
)

// A RuntimeOption is a functional option which alters the Runtime created by
// NewRuntime
type RuntimeOption func(*Runtime) error

// Runtime is the core libpod runtime
type Runtime struct {
	config          *RuntimeConfig
	store           storage.Store
	imageContext    *types.SystemContext
	apparmorEnabled bool
	seccompEnabled  bool
	valid           bool
	lock            sync.RWMutex
}

// RuntimeConfig contains configuration options used to set up the runtime
type RuntimeConfig struct {
	StorageConfig         storage.StoreOptions
	ImageDefaultTransport string
	InsecureRegistries    []string
	Registries            []string
	SignaturePolicyPath   string
	RuntimePath           string
	ConmonPath            string
	ConmonEnvVars         []string
	CgroupManager         string
	SelinuxEnabled        bool
	PidsLimit             int64
}

var (
	defaultRuntimeConfig = RuntimeConfig{
		// Leave this empty so containers/storage will use its defaults
		StorageConfig:         storage.StoreOptions{},
		ImageDefaultTransport: "docker://",
		RuntimePath:           "/usr/bin/runc",
		ConmonPath:            "/usr/local/libexec/crio/conmon",
		ConmonEnvVars: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		CgroupManager:  "cgroupfs",
		SelinuxEnabled: false,
		PidsLimit:      1024,
	}
)

// NewRuntime creates a new container runtime
// Options can be passed to override the default configuration for the runtime
func NewRuntime(options ...RuntimeOption) (*Runtime, error) {
	runtime := new(Runtime)
	runtime.config = new(RuntimeConfig)

	// Copy the default configuration
	deepcopier.Copy(defaultRuntimeConfig).To(runtime.config)

	// Overwrite it with user-given configuration options
	for _, opt := range options {
		if err := opt(runtime); err != nil {
			return nil, errors.Wrapf(err, "error configuring runtime")
		}
	}

	// Set up containers/storage
	store, err := storage.GetStore(runtime.config.StorageConfig)
	if err != nil {
		return nil, err
	}
	runtime.store = store

	// Set up containers/image
	runtime.imageContext = &types.SystemContext{
		SignaturePolicyPath: runtime.config.SignaturePolicyPath,
	}

	runtime.seccompEnabled = seccomp.IsEnabled()
	runtime.apparmorEnabled = apparmor.IsEnabled()

	// Mark the runtime as valid - ready to be used, cannot be modified
	// further
	runtime.valid = true

	return runtime, nil
}

// GetConfig returns a copy of the configuration used by the runtime
func (r *Runtime) GetConfig() *RuntimeConfig {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil
	}

	config := new(RuntimeConfig)

	// Copy so the caller won't be able to modify the actual config
	deepcopier.Copy(r.config).To(config)

	return config
}

// Shutdown shuts down the runtime and associated containers and storage
// If force is true, containers and mounted storage will be shut down before
// cleaning up; if force is false, an error will be returned if there are
// still containers running or mounted
func (r *Runtime) Shutdown(force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return fmt.Errorf("runtime has already been shut down")
	}

	_, err := r.store.Shutdown(force)
	return err
}

// Container API

// A CtrCreateOption is a functional option which alters the Container created
// by NewContainer
type CtrCreateOption func(*ctr.Container) error

// ContainerFilter is a function to determine whether a container is included
// in command output. Containers to be outputted are tested using the function.
// A true return will include the container, a false return will exclude it.
type ContainerFilter func(*ctr.Container) bool

// NewContainer creates a new container from a given OCI config
func (r *Runtime) NewContainer(spec *spec.Spec, options ...CtrCreateOption) (*ctr.Container, error) {
	return nil, ctr.ErrNotImplemented
}

// RemoveContainer removes the given container
// If force is specified, the container will be stopped first
// Otherwise, RemoveContainer will return an error if the container is running
func (r *Runtime) RemoveContainer(c *ctr.Container, force bool) error {
	return ctr.ErrNotImplemented
}

// GetContainer retrieves a container by its ID
func (r *Runtime) GetContainer(id string) (*ctr.Container, error) {
	return nil, ctr.ErrNotImplemented
}

// LookupContainer looks up a container by its name or a partial ID
// If a partial ID is not unique, an error will be returned
func (r *Runtime) LookupContainer(idOrName string) (*ctr.Container, error) {
	return nil, ctr.ErrNotImplemented
}

// GetContainers retrieves all containers from the state
// Filters can be provided which will determine what containers are included in
// the output. Multiple filters are handled by ANDing their output, so only
// containers matching all filters are returned
func (r *Runtime) GetContainers(filters ...ContainerFilter) ([]*ctr.Container, error) {
	return nil, ctr.ErrNotImplemented
}

// Pod API

// PodFilter is a function to determine whether a pod is included in command
// output. Pods to be outputted are tested using the function. A true return
// will include the pod, a false return will exclude it.
type PodFilter func(*pod.Pod) bool

// NewPod makes a new, empty pod
func (r *Runtime) NewPod() (*pod.Pod, error) {
	return nil, ctr.ErrNotImplemented
}

// RemovePod removes a pod and all containers in it
// If force is specified, all containers in the pod will be stopped first
// Otherwise, RemovePod will return an error if any container in the pod is running
// Remove acts atomically, removing all containers or no containers
func (r *Runtime) RemovePod(p *pod.Pod, force bool) error {
	return ctr.ErrNotImplemented
}

// GetPod retrieves a pod by its ID
func (r *Runtime) GetPod(id string) (*pod.Pod, error) {
	return nil, ctr.ErrNotImplemented
}

// LookupPod retrieves a pod by its name or a partial ID
// If a partial ID is not unique, an error will be returned
func (r *Runtime) LookupPod(idOrName string) (*pod.Pod, error) {
	return nil, ctr.ErrNotImplemented
}

// GetPods retrieves all pods
// Filters can be provided which will determine which pods are included in the
// output. Multiple filters are handled by ANDing their output, so only pods
// matching all filters are returned
func (r *Runtime) GetPods(filters ...PodFilter) ([]*pod.Pod, error) {
	return nil, ctr.ErrNotImplemented
}
