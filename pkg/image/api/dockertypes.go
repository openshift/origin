package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// Image is the type representing a docker image and its various properties when
// retrieved from the Docker client API.
type DockerImage struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`

	ID              string       `json:"Id" yaml:"Id"`
	Parent          string       `json:"Parent,omitempty" yaml:"Parent,omitempty"`
	Comment         string       `json:"Comment,omitempty" yaml:"Comment,omitempty"`
	Created         util.Time    `json:"Created,omitempty" yaml:"Created,omitempty"`
	Container       string       `json:"Container,omitempty" yaml:"Container,omitempty"`
	ContainerConfig DockerConfig `json:"ContainerConfig,omitempty" yaml:"ContainerConfig,omitempty"`
	DockerVersion   string       `json:"DockerVersion,omitempty" yaml:"DockerVersion,omitempty"`
	Author          string       `json:"Author,omitempty" yaml:"Author,omitempty"`
	Config          DockerConfig `json:"Config,omitempty" yaml:"Config,omitempty"`
	Architecture    string       `json:"Architecture,omitempty" yaml:"Architecture,omitempty"`
	Size            int64        `json:"Size,omitempty" yaml:"Size,omitempty"`
}

// DockerConfig is the list of configuration options used when creating a container.
type DockerConfig struct {
	Hostname        string              `json:"Hostname,omitempty" yaml:"Hostname,omitempty"`
	Domainname      string              `json:"Domainname,omitempty" yaml:"Domainname,omitempty"`
	User            string              `json:"User,omitempty" yaml:"User,omitempty"`
	Memory          int64               `json:"Memory,omitempty" yaml:"Memory,omitempty"`
	MemorySwap      int64               `json:"MemorySwap,omitempty" yaml:"MemorySwap,omitempty"`
	CPUShares       int64               `json:"CpuShares,omitempty" yaml:"CpuShares,omitempty"`
	CPUSet          string              `json:"Cpuset,omitempty" yaml:"Cpuset,omitempty"`
	AttachStdin     bool                `json:"AttachStdin,omitempty" yaml:"AttachStdin,omitempty"`
	AttachStdout    bool                `json:"AttachStdout,omitempty" yaml:"AttachStdout,omitempty"`
	AttachStderr    bool                `json:"AttachStderr,omitempty" yaml:"AttachStderr,omitempty"`
	PortSpecs       []string            `json:"PortSpecs,omitempty" yaml:"PortSpecs,omitempty"`
	ExposedPorts    map[string]struct{} `json:"ExposedPorts,omitempty" yaml:"ExposedPorts,omitempty"`
	Tty             bool                `json:"Tty,omitempty" yaml:"Tty,omitempty"`
	OpenStdin       bool                `json:"OpenStdin,omitempty" yaml:"OpenStdin,omitempty"`
	StdinOnce       bool                `json:"StdinOnce,omitempty" yaml:"StdinOnce,omitempty"`
	Env             []string            `json:"Env,omitempty" yaml:"Env,omitempty"`
	Cmd             []string            `json:"Cmd,omitempty" yaml:"Cmd,omitempty"`
	DNS             []string            `json:"Dns,omitempty" yaml:"Dns,omitempty"` // For Docker API v1.9 and below only
	Image           string              `json:"Image,omitempty" yaml:"Image,omitempty"`
	Volumes         map[string]struct{} `json:"Volumes,omitempty" yaml:"Volumes,omitempty"`
	VolumesFrom     string              `json:"VolumesFrom,omitempty" yaml:"VolumesFrom,omitempty"`
	WorkingDir      string              `json:"WorkingDir,omitempty" yaml:"WorkingDir,omitempty"`
	Entrypoint      []string            `json:"Entrypoint,omitempty" yaml:"Entrypoint,omitempty"`
	NetworkDisabled bool                `json:"NetworkDisabled,omitempty" yaml:"NetworkDisabled,omitempty"`
}
