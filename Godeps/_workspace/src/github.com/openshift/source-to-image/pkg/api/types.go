package api

// Request contains essential fields for any request.
type Request struct {

	// BaseImage describes which image is used for building the result images.
	BaseImage string

	// DockerSocket describes how to access host docker daemon.
	DockerSocket string

	// PreserveWorkingDir describes if working directory should be left after processing.
	PreserveWorkingDir bool

	// Source URL describing the location of sources used to build the result image.
	Source string

	// Ref is a tag/branch to be used for build.
	Ref string

	// Tag is a result image tag name.
	Tag string

	// Clean describes whether to perform full build even if the build is eligible for incremental build.
	Clean bool

	// RemovePreviousImage describes if previous image should be removed after successful build.
	// This applies only to incremental builds.
	RemovePreviousImage bool

	// Environment is a map of environment variables to be passed to the image.
	Environment map[string]string

	// CallbackURL is a URL which is called upon successful build to inform about that fact.
	CallbackURL string

	// ScriptsURL is a URL describing the localization of STI scripts used during build process.
	ScriptsURL string

	// Location specifies a location where the untar operation will place its artifacts.
	Location string

	// ForcePull describes if the builder should pull the images from registry prior to building.
	ForcePull bool

	// Incremental describes incremental status of current build
	Incremental bool

	// WorkingDir describes temporary directory used for downloading sources, scripts and tar operations.
	WorkingDir string

	// ExternalRequiredScripts describes if required scripts are from external URL.
	ExternalRequiredScripts bool

	// ExternalOptionalScripts describes if optional scripts are from external URL.
	ExternalOptionalScripts bool

	// LayeredBuild describes if this is build which layered scripts and sources on top of BaseImage.
	LayeredBuild bool

	// InstallDestination allows to override the default destination of the STI
	// scripts. It allows to place the scripts into application root directory
	// (see ONBUILD strategy). The default value is "upload/scripts".
	InstallDestination string

	// Specify a relative directory inside the application repository that should
	// be used as a root directory for the application.
	ContextDir string
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
