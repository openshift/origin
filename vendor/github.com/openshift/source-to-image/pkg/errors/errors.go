package errors

import (
	"fmt"
	"os"

	"github.com/openshift/source-to-image/pkg/api/constants"
	utillog "github.com/openshift/source-to-image/pkg/util/log"
)

// Common S2I errors
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
	UserNotAllowedError
	EmptyGitRepositoryError
)

// Error represents an error thrown during S2I execution
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
		Suggestion: fmt.Sprintf("check image name, or if using a local image set the builder image pull policy to %q", "never"),
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

// NewCommitError returns a new error which indicates there was a problem
// committing the image
func NewCommitError(name string, err error) error {
	return Error{
		Message:    fmt.Sprintf("building %s failed when committing the image due to error: %v", name, err),
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
		Suggestion: fmt.Sprintf("set the scripts URL parameter with the location of the S2I scripts, or check if the image has the %q label set", constants.ScriptsURLLabel),
	}
}

// NewInstallRequiredError returns a new error which indicates there was a problem
// when downloading a required script
func NewInstallRequiredError(scripts []string, label string) error {
	return Error{
		Message:    fmt.Sprintf("failed to install %v", scripts),
		Details:    nil,
		ErrorCode:  InstallErrorRequired,
		Suggestion: fmt.Sprintf("set the scripts URL parameter with the location of the S2I scripts, or check if the image has the %q label set", constants.ScriptsURLLabel),
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

// NewUserNotAllowedError returns a new error that indicates that the build
// could not run because the image uses a user outside of the range of allowed users
func NewUserNotAllowedError(image string, onbuild bool) error {
	var msg string
	if onbuild {
		msg = fmt.Sprintf("image %q includes at least one ONBUILD instruction that sets the user to a user that is not allowed", image)
	} else {
		msg = fmt.Sprintf("image %q must specify a user that is numeric and within the range of allowed users", image)
	}
	return Error{
		Message:    msg,
		ErrorCode:  UserNotAllowedError,
		Suggestion: fmt.Sprintf("modify image %q to use a numeric user within the allowed range, or build without the allowed UIDs paremeter set", image),
	}
}

// NewAssembleUserNotAllowedError returns a new error that indicates that the build
// could not run because the build or image uses an assemble user outside of the range
// of allowed users.
func NewAssembleUserNotAllowedError(image string, usesConfig bool) error {
	var msg, suggestion string
	if usesConfig {
		msg = "assemble user must be numeric and within the range of allowed users"
		suggestion = "build without the allowed UIDs or assemble user configurations set"
	} else {
		msg = fmt.Sprintf("image %q includes the %q label whose value is not within the allowed range", image, constants.AssembleUserLabel)
		suggestion = fmt.Sprintf("modify the %q label in image %q to use a numeric user within the allowed range, or build without the allowed UIDs configuration set", constants.AssembleUserLabel, image)
	}
	return Error{
		Message:    msg,
		ErrorCode:  UserNotAllowedError,
		Suggestion: suggestion,
	}
}

// NewEmptyGitRepositoryError returns a new error which indicates that a found
// .git directory has no tracking information, e.g. if the user simply used
// `git init` and forgot about the repository
func NewEmptyGitRepositoryError(source string) error {
	return Error{
		Message:    fmt.Sprintf("The git repository \"%s\" has no tracking information or commits", source),
		ErrorCode:  EmptyGitRepositoryError,
		Suggestion: "Either commit files to the Git repository, remove the .git directory from the project, or force copy of source files to ignore the repository.",
	}
}

// log is a placeholder until the builders pass an output stream down
// client facing libraries should not be using log
var log = utillog.StderrLog

// CheckError checks input error.
// 1. if the input error is nil, the function does nothing but return.
// 2. if the input error is a kind of Error which is thrown during S2I execution,
// the function handle it with Suggestion and Details.
// 3. if the input error is a kind of system Error which is unknown, the function exit with 1.
func CheckError(err error) {
	if err == nil {
		return
	}

	if e, ok := err.(Error); ok {
		log.Errorf("An error occurred: %v", e)
		log.Errorf("Suggested solution: %v", e.Suggestion)
		if e.Details != nil {
			log.V(1).Infof("Details: %v", e.Details)
		}
		log.Error("If the problem persists consult the docs at https://github.com/openshift/source-to-image/tree/master/docs. " +
			"Eventually reach us on freenode #openshift or file an issue at https://github.com/openshift/source-to-image/issues " +
			"providing us with a log from your build using log output level 3.")
		os.Exit(e.ErrorCode)
	} else {
		log.Errorf("An error occurred: %v", err)
		os.Exit(1)
	}
}

// UsageError checks command usage error.
func UsageError(msg string) error {
	return fmt.Errorf("%s", msg)
}
