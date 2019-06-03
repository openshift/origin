package dockerv1client

import (
	"strings"
	"time"
)

// DockerImage is the type representing a docker image and its various properties when
// retrieved from the Docker client API.
type DockerImage struct {
	ID              string        `json:"Id"`
	Parent          string        `json:"Parent,omitempty"`
	Comment         string        `json:"Comment,omitempty"`
	Created         time.Time     `json:"Created,omitempty"`
	Container       string        `json:"Container,omitempty"`
	ContainerConfig DockerConfig  `json:"ContainerConfig,omitempty"`
	DockerVersion   string        `json:"DockerVersion,omitempty"`
	Author          string        `json:"Author,omitempty"`
	Config          *DockerConfig `json:"Config,omitempty"`
	Architecture    string        `json:"Architecture,omitempty"`
	Size            int64         `json:"Size,omitempty"`
}

// Port represents the port number and the protocol, in the form
// <number>/<protocol>. For example: 80/tcp.
type Port string

// Port returns the number of the port.
func (p Port) Port() string {
	return strings.Split(string(p), "/")[0]
}

// Proto returns the name of the protocol.
func (p Port) Proto() string {
	parts := strings.Split(string(p), "/")
	if len(parts) == 1 {
		return "tcp"
	}
	return parts[1]
}

// HealthConfig holds configuration settings for the HEALTHCHECK feature
//
// It has been added in the version 1.24 of the Docker API, available since
// Docker 1.12.
type HealthConfig struct {
	// Test is the test to perform to check that the container is healthy.
	// An empty slice means to inherit the default.
	// The options are:
	// {} : inherit healthcheck
	// {"NONE"} : disable healthcheck
	// {"CMD", args...} : exec arguments directly
	// {"CMD-SHELL", command} : run command with system's default shell
	Test []string `json:"Test,omitempty" yaml:"Test,omitempty" toml:"Test,omitempty"`

	// Zero means to inherit. Durations are expressed as integer nanoseconds.
	Interval time.Duration `json:"Interval,omitempty" yaml:"Interval,omitempty" toml:"Interval,omitempty"` // Interval is the time to wait between checks.
	Timeout  time.Duration `json:"Timeout,omitempty" yaml:"Timeout,omitempty" toml:"Timeout,omitempty"`    // Timeout is the time to wait before considering the check to have hung.

	// Retries is the number of consecutive failures needed to consider a container as unhealthy.
	// Zero means inherit.
	Retries int `json:"Retries,omitempty" yaml:"Retries,omitempty" toml:"Retries,omitempty"`
}

// Mount represents a mount point in the container.
//
// It has been added in the version 1.20 of the Docker API, available since
// Docker 1.8.
type Mount struct {
	Name        string
	Source      string
	Destination string
	Driver      string
	Mode        string
	RW          bool
}

// Config is the list of configuration options used when creating a container.
// Config does not contain the options that are specific to starting a container on a
// given host.  Those are contained in HostConfig
type LegacyConfig struct {
	Hostname          string              `json:"Hostname,omitempty" yaml:"Hostname,omitempty" toml:"Hostname,omitempty"`
	Domainname        string              `json:"Domainname,omitempty" yaml:"Domainname,omitempty" toml:"Domainname,omitempty"`
	User              string              `json:"User,omitempty" yaml:"User,omitempty" toml:"User,omitempty"`
	Memory            int64               `json:"Memory,omitempty" yaml:"Memory,omitempty" toml:"Memory,omitempty"`
	MemorySwap        int64               `json:"MemorySwap,omitempty" yaml:"MemorySwap,omitempty" toml:"MemorySwap,omitempty"`
	MemoryReservation int64               `json:"MemoryReservation,omitempty" yaml:"MemoryReservation,omitempty" toml:"MemoryReservation,omitempty"`
	KernelMemory      int64               `json:"KernelMemory,omitempty" yaml:"KernelMemory,omitempty" toml:"KernelMemory,omitempty"`
	CPUShares         int64               `json:"CpuShares,omitempty" yaml:"CpuShares,omitempty" toml:"CpuShares,omitempty"`
	CPUSet            string              `json:"Cpuset,omitempty" yaml:"Cpuset,omitempty" toml:"Cpuset,omitempty"`
	PortSpecs         []string            `json:"PortSpecs,omitempty" yaml:"PortSpecs,omitempty" toml:"PortSpecs,omitempty"`
	ExposedPorts      map[Port]struct{}   `json:"ExposedPorts,omitempty" yaml:"ExposedPorts,omitempty" toml:"ExposedPorts,omitempty"`
	PublishService    string              `json:"PublishService,omitempty" yaml:"PublishService,omitempty" toml:"PublishService,omitempty"`
	StopSignal        string              `json:"StopSignal,omitempty" yaml:"StopSignal,omitempty" toml:"StopSignal,omitempty"`
	Env               []string            `json:"Env,omitempty" yaml:"Env,omitempty" toml:"Env,omitempty"`
	Cmd               []string            `json:"Cmd" yaml:"Cmd" toml:"Cmd"`
	Healthcheck       *HealthConfig       `json:"Healthcheck,omitempty" yaml:"Healthcheck,omitempty" toml:"Healthcheck,omitempty"`
	DNS               []string            `json:"Dns,omitempty" yaml:"Dns,omitempty" toml:"Dns,omitempty"` // For Docker API v1.9 and below only
	Image             string              `json:"Image,omitempty" yaml:"Image,omitempty" toml:"Image,omitempty"`
	Volumes           map[string]struct{} `json:"Volumes,omitempty" yaml:"Volumes,omitempty" toml:"Volumes,omitempty"`
	VolumeDriver      string              `json:"VolumeDriver,omitempty" yaml:"VolumeDriver,omitempty" toml:"VolumeDriver,omitempty"`
	WorkingDir        string              `json:"WorkingDir,omitempty" yaml:"WorkingDir,omitempty" toml:"WorkingDir,omitempty"`
	MacAddress        string              `json:"MacAddress,omitempty" yaml:"MacAddress,omitempty" toml:"MacAddress,omitempty"`
	Entrypoint        []string            `json:"Entrypoint" yaml:"Entrypoint" toml:"Entrypoint"`
	SecurityOpts      []string            `json:"SecurityOpts,omitempty" yaml:"SecurityOpts,omitempty" toml:"SecurityOpts,omitempty"`
	OnBuild           []string            `json:"OnBuild,omitempty" yaml:"OnBuild,omitempty" toml:"OnBuild,omitempty"`
	Mounts            []Mount             `json:"Mounts,omitempty" yaml:"Mounts,omitempty" toml:"Mounts,omitempty"`
	Labels            map[string]string   `json:"Labels,omitempty" yaml:"Labels,omitempty" toml:"Labels,omitempty"`
	AttachStdin       bool                `json:"AttachStdin,omitempty" yaml:"AttachStdin,omitempty" toml:"AttachStdin,omitempty"`
	AttachStdout      bool                `json:"AttachStdout,omitempty" yaml:"AttachStdout,omitempty" toml:"AttachStdout,omitempty"`
	AttachStderr      bool                `json:"AttachStderr,omitempty" yaml:"AttachStderr,omitempty" toml:"AttachStderr,omitempty"`
	ArgsEscaped       bool                `json:"ArgsEscaped,omitempty" yaml:"ArgsEscaped,omitempty" toml:"ArgsEscaped,omitempty"`
	Tty               bool                `json:"Tty,omitempty" yaml:"Tty,omitempty" toml:"Tty,omitempty"`
	OpenStdin         bool                `json:"OpenStdin,omitempty" yaml:"OpenStdin,omitempty" toml:"OpenStdin,omitempty"`
	StdinOnce         bool                `json:"StdinOnce,omitempty" yaml:"StdinOnce,omitempty" toml:"StdinOnce,omitempty"`
	NetworkDisabled   bool                `json:"NetworkDisabled,omitempty" yaml:"NetworkDisabled,omitempty" toml:"NetworkDisabled,omitempty"`

	// This is no longer used and has been kept here for backward
	// compatibility, please use HostConfig.VolumesFrom.
	VolumesFrom string `json:"VolumesFrom,omitempty" yaml:"VolumesFrom,omitempty" toml:"VolumesFrom,omitempty"`
}

// DockerConfig is the list of configuration options used when creating a container.
type DockerConfig struct {
	Hostname        string              `json:"Hostname,omitempty"`
	Domainname      string              `json:"Domainname,omitempty"`
	User            string              `json:"User,omitempty"`
	Memory          int64               `json:"Memory,omitempty"`
	MemorySwap      int64               `json:"MemorySwap,omitempty"`
	CPUShares       int64               `json:"CpuShares,omitempty"`
	CPUSet          string              `json:"Cpuset,omitempty"`
	AttachStdin     bool                `json:"AttachStdin,omitempty"`
	AttachStdout    bool                `json:"AttachStdout,omitempty"`
	AttachStderr    bool                `json:"AttachStderr,omitempty"`
	PortSpecs       []string            `json:"PortSpecs,omitempty"`
	ExposedPorts    map[string]struct{} `json:"ExposedPorts,omitempty"`
	Tty             bool                `json:"Tty,omitempty"`
	OpenStdin       bool                `json:"OpenStdin,omitempty"`
	StdinOnce       bool                `json:"StdinOnce,omitempty"`
	Env             []string            `json:"Env,omitempty"`
	Cmd             []string            `json:"Cmd,omitempty"`
	DNS             []string            `json:"Dns,omitempty"` // For Docker API v1.9 and below only
	Image           string              `json:"Image,omitempty"`
	Volumes         map[string]struct{} `json:"Volumes,omitempty"`
	VolumesFrom     string              `json:"VolumesFrom,omitempty"`
	WorkingDir      string              `json:"WorkingDir,omitempty"`
	Entrypoint      []string            `json:"Entrypoint,omitempty"`
	NetworkDisabled bool                `json:"NetworkDisabled,omitempty"`
	SecurityOpts    []string            `json:"SecurityOpts,omitempty"`
	OnBuild         []string            `json:"OnBuild,omitempty"`
	Labels          map[string]string   `json:"Labels,omitempty"`
}

// Descriptor describes targeted content. Used in conjunction with a blob
// store, a descriptor can be used to fetch, store and target any kind of
// blob. The struct also describes the wire protocol format. Fields should
// only be added but never changed.
type Descriptor struct {
	// MediaType describe the type of the content. All text based formats are
	// encoded as utf-8.
	MediaType string `json:"mediaType,omitempty"`

	// Size in bytes of content.
	Size int64 `json:"size,omitempty"`

	// Digest uniquely identifies the content. A byte stream can be verified
	// against against this digest.
	Digest string `json:"digest,omitempty"`
}

// DockerImageManifest represents the Docker v2 image format.
type DockerImageManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType,omitempty"`

	// schema1
	Name         string          `json:"name"`
	Tag          string          `json:"tag"`
	Architecture string          `json:"architecture"`
	FSLayers     []DockerFSLayer `json:"fsLayers"`
	History      []DockerHistory `json:"history"`

	// schema2
	Layers []Descriptor `json:"layers"`
	Config Descriptor   `json:"config"`
}

// DockerFSLayer is a container struct for BlobSums defined in an image manifest
type DockerFSLayer struct {
	// DockerBlobSum is the tarsum of the referenced filesystem image layer
	// TODO make this digest.Digest once docker/distribution is in Godeps
	DockerBlobSum string `json:"blobSum"`
}

// DockerHistory stores unstructured v1 compatibility information
type DockerHistory struct {
	// DockerV1Compatibility is the raw v1 compatibility information
	DockerV1Compatibility string `json:"v1Compatibility"`
}

// DockerV1CompatibilityImage represents the structured v1
// compatibility information.
type DockerV1CompatibilityImage struct {
	ID              string        `json:"id"`
	Parent          string        `json:"parent,omitempty"`
	Comment         string        `json:"comment,omitempty"`
	Created         time.Time     `json:"created"`
	Container       string        `json:"container,omitempty"`
	ContainerConfig DockerConfig  `json:"container_config,omitempty"`
	DockerVersion   string        `json:"docker_version,omitempty"`
	Author          string        `json:"author,omitempty"`
	Config          *DockerConfig `json:"config,omitempty"`
	Architecture    string        `json:"architecture,omitempty"`
	Size            int64         `json:"size,omitempty"`
}

// DockerV1CompatibilityImageSize represents the structured v1
// compatibility information for size
type DockerV1CompatibilityImageSize struct {
	Size int64 `json:"size,omitempty"`
}

// DockerImageConfig stores the image configuration
type DockerImageConfig struct {
	ID              string                `json:"id"`
	Parent          string                `json:"parent,omitempty"`
	Comment         string                `json:"comment,omitempty"`
	Created         time.Time             `json:"created"`
	Container       string                `json:"container,omitempty"`
	ContainerConfig DockerConfig          `json:"container_config,omitempty"`
	DockerVersion   string                `json:"docker_version,omitempty"`
	Author          string                `json:"author,omitempty"`
	Config          *DockerConfig         `json:"config,omitempty"`
	Architecture    string                `json:"architecture,omitempty"`
	Size            int64                 `json:"size,omitempty"`
	RootFS          *DockerConfigRootFS   `json:"rootfs,omitempty"`
	History         []DockerConfigHistory `json:"history,omitempty"`
	OS              string                `json:"os,omitempty"`
	OSVersion       string                `json:"os.version,omitempty"`
	OSFeatures      []string              `json:"os.features,omitempty"`
}

// DockerConfigHistory stores build commands that were used to create an image
type DockerConfigHistory struct {
	Created    time.Time `json:"created"`
	Author     string    `json:"author,omitempty"`
	CreatedBy  string    `json:"created_by,omitempty"`
	Comment    string    `json:"comment,omitempty"`
	EmptyLayer bool      `json:"empty_layer,omitempty"`
}

// DockerConfigRootFS describes images root filesystem
type DockerConfigRootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids,omitempty"`
}
