package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// Build encapsulates the inputs needed to produce a new deployable image, as well as
// the status of the operation and a reference to the Pod which runs the build.
type Build struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Labels       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Input is the set of inputs used to configure the build
	Input BuildInput `json:"input,omitempty" yaml:"input,omitempty"`

	// Status is the current status of the build
	Status BuildStatus `json:"status,omitempty" yaml:"status,omitempty"`

	// PodID is the id of the pod that is used to execute the build
	PodID string `json:"podID,omitempty" yaml:"podID,omitempty"`
}

// BuildInput defines the type of build and input parameters for a given build
type BuildInput struct {
	// Type is the type of build to execute
	Type BuildType `json:"type,omitempty" yaml:"type,omitempty"`

	// SourceURI points to the source that will be built. The structure of the source
	// will depend on the type of build to run
	SourceURI string `json:"sourceURI,omitempty" yaml:"sourceURI,omitempty"`

	// ImageTag is the tag to give to the image resulting from the build
	ImageTag string `json:"imageTag,omitempty" yaml:"imageTag,omitempty"`

	// BuilderImage is the image used to execute the build when running STI builds
	BuilderImage string `json:"builderImage,omitempty" yaml:"builderImage,omitempty"`
}

// BuildConfig contains the inputs needed to produce a new deployable image
type BuildConfig struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Labels       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`

	// DesiredInput is the input used to create builds from this configuration
	DesiredInput BuildInput
}

// BuildType is a type of build (docker, sti, etc)
type BuildType string

// Valid build types
const (
	// DockerBuildType is a build based on a Dockerfile with associated artifacts
	DockerBuildType BuildType = "docker"

	// STIBuildType is a build using Source to Image using a git repository
	// and a builder image
	STIBuildType BuildType = "sti"
)

// BuildStatus represents the status of a Build at a point in time.
type BuildStatus string

// Valid build status values
const (
	// BuildNew is automatically assigned to a newly created build
	BuildNew BuildStatus = "new"

	// BuildPending indicates that a pod name has been assigned and a build is
	// about to start running
	BuildPending BuildStatus = "pending"

	// BuildRunning indicates that a pod has been created and a build is running
	BuildRunning BuildStatus = "running"

	// BuildComplete indicates that a build has been successful
	BuildComplete BuildStatus = "complete"

	// BuildFailed indicates that a build has executed and failed
	BuildFailed BuildStatus = "failed"

	// BuildError indicates that an error prevented the build from
	// executing
	BuildError BuildStatus = "error"
)

// BuildList is a collection of Builds.
type BuildList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []Build `json:"items,omitempty" yaml:"items,omitempty"`
}

// BuildConfigList is a collection of BuildConfigs.
type BuildConfigList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []BuildConfig `json:"items,omitempty" yaml:"items,omitempty"`
}
