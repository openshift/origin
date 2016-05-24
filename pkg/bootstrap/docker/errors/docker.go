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

func ErrNoDockerMachineClient(name string, err error) error {
	return NewError("cannot obtain a client for Docker machine %q", name).WithCause(err).WithSolution(noDockerMachineClientSolution())
}

func ErrCannotPingDocker(err error) error {
	return NewError("cannot communicate with Docker").WithCause(err).WithSolution(noDockerClientSolution())
}

// ErrNoInsecureRegistryArgument is thrown when an --insecure-registry argument cannot be detected
// on the Docker daemon process
func ErrNoInsecureRegistryArgument() error {
	return NewError("did not detect an --insecure-registry argument on the Docker daemon").WithSolution(noInsecureRegistryArgSolution())
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

	NoInsecureRegistryArgSolution = `
Ensure that the Docker daemon is running with the following argument:
	--insecure-registry 172.30.0.0/16
`

	NoInsecureRegistryArgSolutionDockerMachine = NoInsecureRegistryArgSolution + `
You can run this command with --create-machine to create a machine with the
right argument.
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

func noInsecureRegistryArgSolution() string {
	switch runtime.GOOS {
	case "darwin":
		if hasDockerMachine() {
			return NoInsecureRegistryArgSolutionDockerMachine
		}
	case "windows":
		if hasDockerMachine() {
			return NoInsecureRegistryArgSolutionDockerMachine
		}
	}
	return NoInsecureRegistryArgSolution
}
