package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
)

// DockerImage is the type representing a docker image and its various properties when
// retrieved from the Docker client API.
type DockerImage struct {
	kapi.TypeMeta `json:",inline"`

	ID              string        `json:"Id"`
	Parent          string        `json:"Parent,omitempty"`
	Comment         string        `json:"Comment,omitempty"`
	Created         util.Time     `json:"Created,omitempty"`
	Container       string        `json:"Container,omitempty"`
	ContainerConfig DockerConfig  `json:"ContainerConfig,omitempty"`
	DockerVersion   string        `json:"DockerVersion,omitempty"`
	Author          string        `json:"Author,omitempty"`
	Config          *DockerConfig `json:"Config,omitempty"`
	Architecture    string        `json:"Architecture,omitempty"`
	Size            int64         `json:"Size,omitempty"`
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

// DockerImageManifest represents the Docker v2 image format.
type DockerImageManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	Name          string          `json:"name"`
	Tag           string          `json:"tag"`
	Architecture  string          `json:"architecture"`
	FSLayers      []DockerFSLayer `json:"fsLayers"`
	History       []DockerHistory `json:"history"`
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
	Created         util.Time     `json:"created"`
	Container       string        `json:"container,omitempty"`
	ContainerConfig DockerConfig  `json:"container_config,omitempty"`
	DockerVersion   string        `json:"docker_version,omitempty"`
	Author          string        `json:"author,omitempty"`
	Config          *DockerConfig `json:"config,omitempty"`
	Architecture    string        `json:"architecture,omitempty"`
	Size            int64         `json:"size,omitempty"`
}
