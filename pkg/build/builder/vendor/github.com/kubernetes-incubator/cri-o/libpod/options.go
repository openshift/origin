package libpod

import (
	"fmt"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/kubernetes-incubator/cri-o/libpod/ctr"
	"github.com/kubernetes-incubator/cri-o/libpod/pod"
)

var (
	errRuntimeFinalized = fmt.Errorf("runtime has already been finalized")
	ctrNotImplemented   = func(c *ctr.Container) error {
		return fmt.Errorf("NOT IMPLEMENTED")
	}
)

const (
	// IPCNamespace represents the IPC namespace
	IPCNamespace = "ipc"
	// MountNamespace represents the mount namespace
	MountNamespace = "mount"
	// NetNamespace represents the network namespace
	NetNamespace = "net"
	// PIDNamespace represents the PID namespace
	PIDNamespace = "pid"
	// UserNamespace represents the user namespace
	UserNamespace = "user"
	// UTSNamespace represents the UTS namespace
	UTSNamespace = "uts"
)

// Runtime Creation Options

// WithStorageConfig uses the given configuration to set up container storage
// If this is not specified, the system default configuration will be used
// instead
func WithStorageConfig(config storage.StoreOptions) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.StorageConfig.RunRoot = config.RunRoot
		rt.config.StorageConfig.GraphRoot = config.GraphRoot
		rt.config.StorageConfig.GraphDriverName = config.GraphDriverName

		rt.config.StorageConfig.GraphDriverOptions = make([]string, len(config.GraphDriverOptions))
		copy(rt.config.StorageConfig.GraphDriverOptions, config.GraphDriverOptions)

		rt.config.StorageConfig.UIDMap = make([]idtools.IDMap, len(config.UIDMap))
		copy(rt.config.StorageConfig.UIDMap, config.UIDMap)

		rt.config.StorageConfig.GIDMap = make([]idtools.IDMap, len(config.UIDMap))
		copy(rt.config.StorageConfig.GIDMap, config.GIDMap)

		return nil
	}
}

// WithImageConfig uses the given configuration to set up image handling
// If this is not specified, the system default configuration will be used
// instead
func WithImageConfig(defaultTransport string, insecureRegistries, registries []string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.ImageDefaultTransport = defaultTransport

		rt.config.InsecureRegistries = make([]string, len(insecureRegistries))
		copy(rt.config.InsecureRegistries, insecureRegistries)

		rt.config.Registries = make([]string, len(registries))
		copy(rt.config.Registries, registries)

		return nil
	}
}

// WithSignaturePolicy specifies the path of a file which decides how trust is
// managed for images we've pulled.
// If this is not specified, the system default configuration will be used
// instead
func WithSignaturePolicy(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.SignaturePolicyPath = path

		return nil
	}
}

// WithOCIRuntime specifies an OCI runtime to use for running containers
func WithOCIRuntime(runtimePath string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.RuntimePath = runtimePath

		return nil
	}
}

// WithConmonPath specifies the path to the conmon binary which manages the
// runtime
func WithConmonPath(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.ConmonPath = path

		return nil
	}
}

// WithConmonEnv specifies the environment variable list for the conmon process
func WithConmonEnv(environment []string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.ConmonEnvVars = make([]string, len(environment))
		copy(rt.config.ConmonEnvVars, environment)

		return nil
	}
}

// WithCgroupManager specifies the manager implementation name which is used to
// handle cgroups for containers
func WithCgroupManager(manager string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.CgroupManager = manager

		return nil
	}
}

// WithSELinux enables SELinux on the container server
func WithSELinux() RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.SelinuxEnabled = true

		return nil
	}
}

// WithPidsLimit specifies the maximum number of processes each container is
// restricted to
func WithPidsLimit(limit int64) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return errRuntimeFinalized
		}

		rt.config.PidsLimit = limit

		return nil
	}
}

// Container Creation Options

// WithRootFSFromPath uses the given path as a container's root filesystem
// No further setup is performed on this path
func WithRootFSFromPath(path string) CtrCreateOption {
	return ctrNotImplemented
}

// WithRootFSFromImage sets up a fresh root filesystem using the given image
// If useImageConfig is specified, image volumes, environment variables, and
// other configuration from the image will be added to the config
func WithRootFSFromImage(image string, useImageConfig bool) CtrCreateOption {
	return ctrNotImplemented
}

// WithSharedNamespaces sets a container to share namespaces with another
// container. If the from container belongs to a pod, the new container will
// be added to the pod.
// By default no namespaces are shared. To share a namespace, add the Namespace
// string constant to the map as a key
func WithSharedNamespaces(from *ctr.Container, namespaces map[string]string) CtrCreateOption {
	return ctrNotImplemented
}

// WithPod adds the container to a pod
func WithPod(pod *pod.Pod) CtrCreateOption {
	return ctrNotImplemented
}

// WithLabels adds labels to the pod
func WithLabels(labels map[string]string) CtrCreateOption {
	return ctrNotImplemented
}

// WithAnnotations adds annotations to the pod
func WithAnnotations(annotations map[string]string) CtrCreateOption {
	return ctrNotImplemented
}

// WithName sets the container's name
func WithName(name string) CtrCreateOption {
	return ctrNotImplemented
}

// WithStopSignal sets the signal that will be sent to stop the container
func WithStopSignal(signal uint) CtrCreateOption {
	return ctrNotImplemented
}
