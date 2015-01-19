package errors

import (
	"path/filepath"
)

// GenerationError is an error returned from config generators
type GenerationError int

const (
	NoGit GenerationError = iota + 1
	SourceDirAndURL
	InvalidSourceDir
	CouldNotDetect
	NoBuilderFound
	InvalidDockerfile
	ImageNotFound
)

func (e GenerationError) Error() string {
	switch e {
	case NoGit:
		return "Git was not detected in your system. It is needed for build config generation."
	case SourceDirAndURL:
		return "A source directory and a source URL were specified. Please only specify one."
	case InvalidSourceDir:
		return "The source directory is not readable or is invalid."
	case CouldNotDetect:
		return "Could not detect a build type from the source."
	case NoBuilderFound:
		return "Could not find a builder to match the STI source repository."
	case InvalidDockerfile:
		return "Invalid Dockerfile. Does not contain a FROM clause."
	case ImageNotFound:
		return "Image data could not be found."
	}
	return ""
}

// MultipleDockerfiles creates an error caused by multiple Dockerfiles existing in a repository
func NewMultipleDockerfilesErr(paths []string) error {
	err := multipleDockerFilesError{}
	err = append(err, paths...)
	return err
}

type multipleDockerFilesError []string

func (e multipleDockerFilesError) Error() string {
	result := "Error: Multiple Dockerfile(s) found.\nSpecify one of the following flags:\n"
	for _, f := range e {
		dir := filepath.Dir(f)
		if dir == "" {
			dir = "."
		}
		result += "--context=\"" + dir + "\""
		result += "\n"
	}
	return result
}
