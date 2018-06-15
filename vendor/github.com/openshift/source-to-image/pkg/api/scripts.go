package api

import (
	"os"
)

const (
	// Assemble is the name of the script responsible for the build process of the resulting image.
	Assemble = "assemble"
	// AssembleRuntime is the name of the script responsible for the preparation process of the resulting image.
	AssembleRuntime = "assemble-runtime"
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
	UserScripts = "downloads" + string(os.PathSeparator) + "scripts"
	// DefaultScripts is the location of scripts downloaded from default location (io.openshift.s2i.scripts-url label).
	DefaultScripts = "downloads" + string(os.PathSeparator) + "defaultScripts"
	// SourceScripts is the location of scripts downloaded with application sources.
	SourceScripts = "upload" + string(os.PathSeparator) + "src" + string(os.PathSeparator) + ".s2i" + string(os.PathSeparator) + "bin"
	// UploadScripts is the location of scripts that will be uploaded to the image during STI build.
	UploadScripts = "upload" + string(os.PathSeparator) + "scripts"
	// Source is the location of application sources.
	Source = "upload" + string(os.PathSeparator) + "src"

	// ContextTmp is the location of applications sources off of a supplied context dir
	ContextTmp = "upload" + string(os.PathSeparator) + "tmp"

	// RuntimeArtifactsDir is the location of application artifacts and scripts that will be copied into a runtime image.
	RuntimeArtifactsDir = "upload" + string(os.PathSeparator) + "runtimeArtifacts"

	// IgnoreFile is the s2i version for ignore files like we see with .gitignore or .dockerignore .. initial impl mirrors documented .dockerignore capabilities
	IgnoreFile = ".s2iignore"
)
