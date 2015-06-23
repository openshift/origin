package errors

import (
	"fmt"
)

// Common STI errors
const (
	InspectImageError int = 1 + iota
	PullImageError
	SaveArtifactsError
	AssembleError
	WorkdirError
	BuildError
	TarTimeoutError
	DownloadError
	ScriptsInsideImageError
	InstallError
	InstallErrorRequired
	URLHandlerError
	STIContainerError
	SourcePathError
)

// Error represents an error thrown during STI execution
type Error struct {
	Message    string
	Details    error
	ErrorCode  int
	Suggestion string
}

// ContainerError is an error returned when a container exits with a non-zero code.
// ExitCode contains the exit code from the container
type ContainerError struct {
	Message    string
	Output     string
	ErrorCode  int
	Suggestion string
	ExitCode   int
}

// Error returns a string for a given error
func (s Error) Error() string {
	return s.Message
}

// Error returns a string for the given error
func (s ContainerError) Error() string {
	return s.Message
}

// NewInspectImageError returns a new error which indicates there was a problem
// inspecting the image
func NewInspectImageError(name string, err error) error {
	return Error{
		Message:    fmt.Sprintf("unable to get metadata for %s", name),
		Details:    err,
		ErrorCode:  InspectImageError,
		Suggestion: "check image name",
	}
}

// NewPullImageError returns a new error which indicates there was a problem
// pulling the image
func NewPullImageError(name string, err error) error {
	return Error{
		Message:    fmt.Sprintf("unable to get %s", name),
		Details:    err,
		ErrorCode:  PullImageError,
		Suggestion: "check image name, or if using local image add --force-pull=false flag",
	}
}

// NewSaveArtifactsError returns a new error which indicates there was a problem
// calling save-artifacts script
func NewSaveArtifactsError(name, output string, err error) error {
	return Error{
		Message:    fmt.Sprintf("saving artifacts for %s failed:\n%s", name, output),
		Details:    err,
		ErrorCode:  SaveArtifactsError,
		Suggestion: "check the save-artifacts script for errors",
	}
}

// NewAssembleError returns a new error which indicates there was a problem
// running assemble script
func NewAssembleError(name, output string, err error) error {
	return Error{
		Message:    fmt.Sprintf("assemble for %s failed:\n%s", name, output),
		Details:    err,
		ErrorCode:  AssembleError,
		Suggestion: "check the assemble script output for errors",
	}
}

// NewWorkDirError returns a new error which indicates there was a problem
// when creating working directory
func NewWorkDirError(dir string, err error) error {
	return Error{
		Message:    fmt.Sprintf("creating temporary directory %s failed", dir),
		Details:    err,
		ErrorCode:  WorkdirError,
		Suggestion: "check if you have access to your system's temporary directory",
	}
}

// NewBuildError returns a new error which indicates there was a problem
// building the image
func NewBuildError(name string, err error) error {
	return Error{
		Message:    fmt.Sprintf("building %s failed", name),
		Details:    err,
		ErrorCode:  BuildError,
		Suggestion: "check the build output for errors",
	}
}

// NewTarTimeoutError returns a new error which indicates there was a problem
// when sending or receiving tar stream
func NewTarTimeoutError() error {
	return Error{
		Message:    fmt.Sprintf("timeout waiting for tar stream"),
		Details:    nil,
		ErrorCode:  TarTimeoutError,
		Suggestion: "check the Source-To-Image scripts if it accepts tar stream for assemble and sends for save-artifacts",
	}
}

// NewDownloadError returns a new error which indicates there was a problem
// when downloading a file
func NewDownloadError(url string, code int) error {
	return Error{
		Message:    fmt.Sprintf("failed to retrieve %s, response code %d", url, code),
		Details:    nil,
		ErrorCode:  DownloadError,
		Suggestion: "check the availability of the address",
	}
}

// NewScriptsInsideImageError returns a new error which informs of scripts
// being placed inside the image
func NewScriptsInsideImageError(url string) error {
	return Error{
		Message:    fmt.Sprintf("scripts inside the image: %s", url),
		Details:    nil,
		ErrorCode:  ScriptsInsideImageError,
		Suggestion: "",
	}
}

// NewInstallError returns a new error which indicates there was a problem
// when downloading a script
func NewInstallError(script string) error {
	return Error{
		Message:    fmt.Sprintf("failed to install %v", script),
		Details:    nil,
		ErrorCode:  InstallError,
		Suggestion: "provide URL with Source-To-Image scripts with -s flag or check the image if it contains io.s2i.scripts-url label set",
	}
}

// NewInstallRequiredError returns a new error which indicates there was a problem
// when downloading a required script
func NewInstallRequiredError(scripts []string) error {
	return Error{
		Message:    fmt.Sprintf("failed to install %v", scripts),
		Details:    nil,
		ErrorCode:  InstallErrorRequired,
		Suggestion: "provide URL with Source-To-Image scripts with -s flag or check the image if it contains io.s2i.scripts-url label set",
	}
}

// NewURLHandlerError returns a new error which indicates there was a problem
// when trying to read scripts URL
func NewURLHandlerError(url string) error {
	return Error{
		Message:    fmt.Sprintf("no URL handler for %s", url),
		Details:    nil,
		ErrorCode:  URLHandlerError,
		Suggestion: "check the URL",
	}
}

// NewContainerError return a new error which indicates there was a problem
// invoking command inside container
func NewContainerError(name string, code int, output string) error {
	return ContainerError{
		Message:    fmt.Sprintf("non-zero (%d) exit code from %s", code, name),
		Output:     output,
		ErrorCode:  STIContainerError,
		Suggestion: "check the container logs for more information on the failure",
		ExitCode:   code,
	}
}

// NewSourcePathError returns a new error which indicates there was a problem
// when accessing the source code from the local filesystem
func NewSourcePathError(path string) error {
	return Error{
		Message:    fmt.Sprintf("Local filesystem source path does not exist: %s", path),
		Details:    nil,
		ErrorCode:  SourcePathError,
		Suggestion: "check the source code path on the local filesystem",
	}
}
