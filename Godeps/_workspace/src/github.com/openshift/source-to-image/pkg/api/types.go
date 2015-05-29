package api

import docker "github.com/fsouza/go-dockerclient"

// Config contains essential fields for performing build.
type Config struct {

	// BuilderImage describes which image is used for building the result images.
	BuilderImage string

	// DockerConfig describes how to access host docker daemon.
	DockerConfig *DockerConfig

	// DockerCfgPath provides the path to the .dockercfg file
	DockerCfgPath string

	// PullAuthentication holds the authentication information for pulling the
	// Docker images from private repositories
	PullAuthentication docker.AuthConfiguration

	// PreserveWorkingDir describes if working directory should be left after processing.
	PreserveWorkingDir bool

	// Source URL describing the location of sources used to build the result image.
	Source string

	// Ref is a tag/branch to be used for build.
	Ref string

	// Tag is a result image tag name.
	Tag string

	// Incremental describes whether to try to perform incremental build.
	Incremental bool

	// RemovePreviousImage describes if previous image should be removed after successful build.
	// This applies only to incremental builds.
	RemovePreviousImage bool

	// Environment is a map of environment variables to be passed to the image.
	Environment map[string]string

	// EnvironmentFile provides the path to a file with list of environment
	// variables.
	EnvironmentFile string

	// CallbackURL is a URL which is called upon successful build to inform about that fact.
	CallbackURL string

	// ScriptsURL is a URL describing the localization of STI scripts used during build process.
	ScriptsURL string

	// Location specifies a location where the untar operation will place its artifacts.
	Location string

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool

	// WorkingDir describes temporary directory used for downloading sources, scripts and tar operations.
	WorkingDir string

	// LayeredBuild describes if this is build which layered scripts and sources on top of BuilderImage.
	LayeredBuild bool

	// Operate quietly. Progress and assemble script output are not reported, only fatal errors.
	// (default: false).
	Quiet bool

	// Specify a relative directory inside the application repository that should
	// be used as a root directory for the application.
	ContextDir string

	// DNS servers to use when running build container
	DNS []string

	// DNS search suffixes to use when running build container
	DNSSearch []string
}

// DockerConfig contains the configuration for a Docker connection
type DockerConfig struct {
	// Endpoint is the docker network endpoint or socket
	Endpoint string

	// CertFile is the certificate file path for a TLS connection
	CertFile string

	// KeyFile is the key file path for a TLS connection
	KeyFile string

	// CAFile is the certificate authority file path for a TLS connection
	CAFile string
}

// Result structure contains information from build process.
type Result struct {

	// Success describes whether the build was successful.
	Success bool

	// Messages is a list of messages from build process.
	Messages []string

	// WorkingDir describes temporary directory used for downloading sources, scripts and tar operations.
	WorkingDir string

	// ImageID describes resulting image ID.
	ImageID string
}

// InstallResult structure describes the result of install operation
type InstallResult struct {

	// Script describes which script this result refers to
	Script string

	// URL describes from where the script was taken
	URL string

	// Downloaded describes if download operation happened, this will be true for
	// external scripts, but false for scripts from inside the image
	Downloaded bool

	// Installed describes if script was installed to upload directory
	Installed bool

	// Error describes last error encountered during install operation
	Error error
}
