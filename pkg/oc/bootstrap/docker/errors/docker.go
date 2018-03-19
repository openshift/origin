package errors

import (
	"fmt"
	"os/exec"
	"runtime"
)

// ErrNoDockerClient is thrown when a Docker client cannot be obtained or cannot be pinged
func ErrNoDockerClient(err error) error {
	return NewError("cannot obtain a Docker client").WithCause(err).WithSolution(noDockerClientSolution())
}

// ErrNoDockerMachineClient is returned when a Docker client cannot be obtained from the given Docker machine
func ErrNoDockerMachineClient(name string, err error) error {
	return NewError("cannot obtain a client for Docker machine %q", name).WithCause(err).WithSolution(noDockerMachineClientSolution())
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

Once installed, run this command with the --create-machine
argument to create a new Docker machine that will run OpenShift.
`
	NoDockerMachineMacSolution = `
To create a new Docker machine to run OpenShift, run this command again with
the --create-machine argument. This will create a Docker machine named
'openshift'.

To use a different machine name, specify the --machine-name=NAME argument.

If you wish to use an existing Docker machine, enable it before running this
command by executing:

   eval $(docker-machine env NAME)

where NAME is the name of your Docker machine.
`
	NoDockerWindowsSolution = `
Please install Docker tools by following instructions at:

   https://docs.docker.com/windows/

Once installed, run this command with the --create-machine argument to create a
new Docker machine that will run OpenShift.
`
	NoDockerMachineWindowsSolution = `
To create a new Docker machine to run OpenShift, run this command again with
the --create-machine argument. This will create a Docker machine named
'openshift'.

To use a different machine name, specify the --machine-name=NAME argument.

If you wish to use an existing Docker machine, enable it before running this
command by executing:

   docker-machine env

where NAME is the name of your Docker machine.
`
	NoDockerLinuxSolution = `
Ensure that Docker is installed and accessible in your environment.
Use your package manager or follow instructions at:

   https://docs.docker.com/linux/
`

	NoDockerMachineClientSolution = `
Ensure that the Docker machine is available and running. You can also create a
new Docker machine by specifying the --create-machine flag.
`

	InvalidInsecureRegistryArgSolution = `
Ensure that the Docker daemon is running with the following argument:
	--insecure-registry 172.30.0.0/16
`

	InvalidInsecureRegistryArgSolutionDockerMachine = InvalidInsecureRegistryArgSolution + `
You can run this command with --create-machine to create a machine with the
right argument.
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

func hasDockerMachine() bool {
	binary := "docker-machine"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	_, err := exec.LookPath(binary)
	return err == nil
}

func noDockerClientSolution() string {
	switch runtime.GOOS {
	case "darwin":
		if hasDockerMachine() {
			return NoDockerMachineMacSolution
		}
		return NoDockerMacSolution
	case "windows":
		if hasDockerMachine() {
			return NoDockerMachineWindowsSolution
		}
		return NoDockerWindowsSolution
	case "linux":
		return NoDockerLinuxSolution
	}
	return fmt.Sprintf("Platform %s is not supported by this command", runtime.GOOS)
}

func noDockerMachineClientSolution() string {
	return NoDockerMachineClientSolution
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
	if hasDockerMachine() {
		return InvalidInsecureRegistryArgSolutionDockerMachine
	}
	return InvalidInsecureRegistryArgSolution
}
