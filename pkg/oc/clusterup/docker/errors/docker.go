package errors

import (
	"fmt"
	"runtime"
)

// ErrNoDockerClient is thrown when a Docker client cannot be obtained or cannot be pinged
func ErrNoDockerClient(err error) error {
	return NewError("cannot obtain a Docker client").WithCause(err).WithSolution(noDockerClientSolution())
}

// ErrKubeConfigNotWriteable is returned when the file pointed to by KUBECONFIG cannot be created or written to
func ErrKubeConfigNotWriteable(file string, err error) error {
	return NewError("KUBECONFIG is set to a file that cannot be created or modified: %s", file).WithCause(err).WithSolution(KubeConfigSolutionUnix)
}

const (
	NoDockerLinuxSolution = `
Ensure that Docker is installed and accessible in your environment.
Use your package manager or follow instructions at:

   https://docs.docker.com/linux/
`

	KubeConfigSolutionUnix = `
You can unset the KUBECONFIG variable to use the default location for it:
   unset KUBECONFIG

Or you can set its value to a file that can be written to:
   export KUBECONFIG=/path/to/file
`
)

func noDockerClientSolution() string {
	switch runtime.GOOS {
	case "linux":
		return NoDockerLinuxSolution
	}
	return fmt.Sprintf("Platform %s is not supported by this command", runtime.GOOS)
}
