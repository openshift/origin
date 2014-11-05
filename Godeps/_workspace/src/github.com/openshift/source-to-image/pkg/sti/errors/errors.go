package errors

import (
	"fmt"
)

// StiError represents an error thrown during STI execution
type StiError int

const (
	ErrPullImageFailed StiError = iota
	ErrScriptsDownloadFailed
	ErrSaveArtifactsFailed
	ErrBuildFailed
)

// Error returns a string for a given error
func (s StiError) Error() string {
	switch s {
	case ErrPullImageFailed:
		return "Couldn't pull image"
	case ErrScriptsDownloadFailed:
		return "Scripts download failed"
	case ErrSaveArtifactsFailed:
		return "Error saving artifacts for incremental build"
	case ErrBuildFailed:
		return "Running assemble in base image failed"
	default:
		return "Unknown error"
	}
}

// StiContainerError is an error returned when a container
// exits with a non-zero code. ExitCode contains the exit code for the
// container
type StiContainerError struct {
	ExitCode int
}

// Error returns a string for the given error
func (e StiContainerError) Error() string {
	return fmt.Sprintf("Container exited with exit code: %d", e.ExitCode)
}
