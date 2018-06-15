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
	return NewError("KUBECONFIG is set to a file that cannot be created or modified: %s", file).WithCause(err).WithSolution(kubeConfigSolution())
}

// ErrNoInsecureRegistryArgument is thrown when an --insecure-registry argument cannot be detected
// on the Docker daemon process
func ErrNoInsecureRegistryArgument() error {
	return NewError("did not detect an --insecure-registry argument on the Docker daemon").WithSolution(invalidInsecureRegistryArgSolution())
}

// ErrInvalidInsecureRegistryArgument is thrown when an --insecure-registry argument is found, but does not allow sufficient access
// for our services to operate
func ErrInvalidInsecureRegistryArgument() error {
	return NewError("did not detect a sufficient --insecure-registry argument on the Docker daemon").WithSolution(invalidInsecureRegistryArgSolution())
}

const (
	NoDockerMacSolution = `
Please install Docker tools by following instructions at:

   https://docs.docker.com/mac/
`
	NoDockerWindowsSolution = `
Please install Docker tools by following instructions at:

   https://docs.docker.com/windows/
`
	NoDockerLinuxSolution = `
Ensure that Docker is installed and accessible in your environment.
Use your package manager or follow instructions at:

   https://docs.docker.com/linux/
`
	InvalidInsecureRegistryArgSolution = `
Ensure that the Docker daemon is running with the following argument:
	--insecure-registry 172.30.0.0/16
`

	KubeConfigSolutionUnix = `
You can unset the KUBECONFIG variable to use the default location for it:
   unset KUBECONFIG

Or you can set its value to a file that can be written to:
   export KUBECONFIG=/path/to/file
`

	KubeConfigSolutionWindows = `
You can clear the KUBECONFIG variable to use the default location for it:
   set KUBECONFIG=

Or you can set its value to a file that can be written to:
   set KUBECONFIG=c:\path\to\file
`
)

func noDockerClientSolution() string {
	switch runtime.GOOS {
	case "darwin":
		return NoDockerMacSolution
	case "windows":
		return NoDockerWindowsSolution
	case "linux":
		return NoDockerLinuxSolution
	}
	return fmt.Sprintf("Platform %s is not supported by this command", runtime.GOOS)
}

func kubeConfigSolution() string {
	switch runtime.GOOS {
	case "windows":
		return KubeConfigSolutionWindows
	default:
		return KubeConfigSolutionUnix
	}
}

func invalidInsecureRegistryArgSolution() string {
	return InvalidInsecureRegistryArgSolution
}
