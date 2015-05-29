package api

const (
	// Assemble is the name of the script responsible for build process of the resulting image.
	Assemble = "assemble"
	// Run is the name of the script responsible for running the final application.
	Run = "run"
	// SaveArtifacts is the name of the script responsible for storing dependencies etc. between builds.
	SaveArtifacts = "save-artifacts"
	// Usage is the name of the script responsible for printing the builder image's short info.
	Usage = "usage"

	// Environment contains list of key value pairs that will be set during the
	// STI build. Users can use this file to provide extra configuration
	// depending on the builder image used.
	Environment = "environment"
)

const (
	// UserScripts is the location of scripts downloaded from user provided URL (-s flag).
	UserScripts = "downloads/scripts"
	// DefaultScripts is the location of scripts downloaded from default location (io.openshift.sti.scripts-url label).
	DefaultScripts = "downloads/defaultScripts"
	// SourceScripts is the location of scripts downloaded with application sources.
	SourceScripts = "upload/src/.sti/bin"

	// UploadScripts is the location of scripts that will be uploaded to the image during STI build.
	UploadScripts = "upload/scripts"
	// Source is the location of application sources.
	Source = "upload/src"
)
