package errors

import (
	"fmt"

	"github.com/openshift/source-to-image/pkg/api"
)

// Common STI errors
const (
	ErrInspectImage int = 1 + iota
	ErrPullImage
	ErrScriptDownload
	ErrSaveArtifacts
	ErrBuild
	ErrSTIContainer
	ErrTarTimeout
	ErrWorkDir
	ErrDownload
	ErrURLHandler
	ErrDefaultScriptsURL
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
		ErrorCode:  ErrInspectImage,
		Suggestion: "check image name",
	}
}

// NewPullImageError returns a new error which indicates there was a problem
// pulling the image
func NewPullImageError(name string, err error) error {
	return Error{
		Message:    fmt.Sprintf("unable to get %s", name),
		Details:    err,
		ErrorCode:  ErrPullImage,
		Suggestion: "check image name, or if using local image add --forcePull=false flag",
	}
}

// NewScriptDownloadError returns a new error which indicates there was a problem
// downloading a script
func NewScriptDownloadError(name api.Script, err error) error {
	return Error{
		Message:    fmt.Sprintf("%s script download failed", name),
		Details:    err,
		ErrorCode:  ErrScriptDownload,
		Suggestion: "provide URL with STI scripts with -s flag or check the image if it contains STI_SCRIPTS_URL variable set",
	}
}

// NewSaveArtifactsError returns a new error which indicates there was a problem
// calling save-artifacts script
func NewSaveArtifactsError(name, output string, err error) error {
	return Error{
		Message:    fmt.Sprintf("saving artifacts for %s failed:\n%s", name, output),
		Details:    err,
		ErrorCode:  ErrSaveArtifacts,
		Suggestion: "check the save-artifacts script for errors",
	}
}

// NewAssembleError returns a new error which indicates there was a problem
// running assemble script
func NewAssembleError(name, output string, err error) error {
	return Error{
		Message:    fmt.Sprintf("assemble for %s failed:\n%s", name, output),
		Details:    err,
		ErrorCode:  ErrBuild,
		Suggestion: "check the assemble script output for errors",
	}
}

// NewBuildError returns a new error which indicates there was a problem
// building the image
func NewBuildError(name string, err error) error {
	return Error{
		Message:    fmt.Sprintf("building %s failed", name),
		Details:    err,
		ErrorCode:  ErrBuild,
		Suggestion: "check the build output for errors",
	}
}

// NewTarTimeoutError returns a new error which indicates there was a problem
// when sending or receiving tar stream
func NewTarTimeoutError() error {
	return Error{
		Message:    fmt.Sprintf("timeout waiting for tar stream"),
		Details:    nil,
		ErrorCode:  ErrTarTimeout,
		Suggestion: "check the sti-helper script if it accepts tar stream for assemble and sends for save-artifacts",
	}
}

// NewWorkDirError returns a new error which indicates there was a problem
// when creating working directory
func NewWorkDirError(dir string, err error) error {
	return Error{
		Message:    fmt.Sprintf("creating temporary directory %s failed", dir),
		Details:    err,
		ErrorCode:  ErrWorkDir,
		Suggestion: "check if you have access to your system's temporary directory",
	}
}

// NewDownloadError returns a new error which indicates there was a problem
// when downloading a file
func NewDownloadError(url string, code int) error {
	return Error{
		Message:    fmt.Sprintf("failed to retrieve %s, response code %d", url, code),
		Details:    nil,
		ErrorCode:  ErrDownload,
		Suggestion: "check the availability of the address",
	}
}

// NewURLHandlerError returns a new error which indicates there was a problem
// when trying to read scripts URL
func NewURLHandlerError(url string) error {
	return Error{
		Message:    fmt.Sprintf("no URL handler for %s", url),
		Details:    nil,
		ErrorCode:  ErrURLHandler,
		Suggestion: "check the URL",
	}
}

// NewDefaultScriptsURLError return a new error which indicates there was a problem
// when trying to read STI_SCRIPTS_URL
func NewDefaultScriptsURLError(err error) error {
	return Error{
		Message:    fmt.Sprintf("error reading STI_SCRIPTS_URL"),
		Details:    err,
		ErrorCode:  ErrDefaultScriptsURL,
		Suggestion: "check the image",
	}
}

// NewContainerError return a new error which indicates there was a problem
// invoking command inside container
func NewContainerError(name string, code int, output string) error {
	return ContainerError{
		Message:    fmt.Sprintf("non-zero (%d) exit code from %s", code, name),
		Output:     output,
		ErrorCode:  ErrSTIContainer,
		Suggestion: "check the container logs for more information on the failure",
		ExitCode:   code,
	}
}
