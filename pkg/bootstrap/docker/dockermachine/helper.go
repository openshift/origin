package dockermachine

import (
	"bufio"
	"bytes"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	dockerclient "github.com/docker/engine-api/client"
	"github.com/docker/go-connections/tlsconfig"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/bootstrap/docker/localcmd"
	"k8s.io/kubernetes/pkg/util/net"
)

const (
	defaultMachineMemory     = 2048
	defaultMachineProcessors = 2
)

// Builder can be used to create a new Docker machine on the local system
type Builder struct {
	name       string
	memory     int
	processors int
}

// NewBuilder creates a Docker machine Builder object used to create a Docker machine
func NewBuilder() *Builder {
	return &Builder{}
}

// Name sets the name of the Docker machine to build
func (b *Builder) Name(name string) *Builder {
	b.name = name
	return b
}

// Memory sets the amount of memory (in MB) to give a Docker machine when creating it
func (b *Builder) Memory(mem int) *Builder {
	b.memory = mem
	return b
}

// Processors sets the number of processors to give a Docker machine when creating it
func (b *Builder) Processors(proc int) *Builder {
	b.processors = proc
	return b
}

// Create creates a new Docker machine
func (b *Builder) Create() error {
	if Exists(b.name) {
		return ErrDockerMachineExists
	}
	if IsAvailable() {
		return ErrDockerMachineNotAvailable
	}
	mem := b.memory
	if mem == 0 {
		mem = determineMachineMemory()
	}
	proc := b.processors
	if proc == 0 {
		proc = determineMachineProcessors()
	}
	return localcmd.New(dockerMachineBinary()).Args(
		"create",
		"--driver", "virtualbox",
		"--virtualbox-cpu-count", strconv.Itoa(proc),
		"--virtualbox-memory", strconv.Itoa(mem),
		"--engine-insecure-registry", "172.30.0.0/16",
		b.name).Run()
}

// IsRunning returns true if a Docker machine is running
func IsRunning(name string) bool {
	err := localcmd.New(dockerMachineBinary()).Args("ip", name).Run()
	return err == nil
}

// IP returns the IP address of the Docker machine
func IP(name string) (string, error) {
	output, _, err := localcmd.New(dockerMachineBinary()).Args("ip", name).Output()
	if err != nil {
		return "", ErrDockerMachineExec("ip", err)
	}
	return strings.TrimSpace(output), nil
}

// Exists returns true if a Docker machine exists
func Exists(name string) bool {
	err := localcmd.New(dockerMachineBinary()).Args("inspect", name).Run()
	return err == nil
}

// Start starts up an existing Docker machine
func Start(name string) error {
	err := localcmd.New(dockerMachineBinary()).Args("start", name).Run()
	if err != nil {
		return ErrDockerMachineExec("start", err)
	}
	return nil
}

// Client returns a Docker client for the given Docker machine
func Client(name string) (*docker.Client, *dockerclient.Client, error) {
	output, _, err := localcmd.New(dockerMachineBinary()).Args("env", name).Output()
	if err != nil {
		return nil, nil, ErrDockerMachineExec("env", err)
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(output))
	var (
		dockerHost, certPath string
		tlsVerify            bool
	)
	prefix := "export "
	if runtime.GOOS == "windows" {
		prefix = "SET "
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			line = strings.TrimPrefix(line, prefix)
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			switch strings.ToUpper(parts[0]) {
			case "DOCKER_HOST":
				dockerHost = strings.Trim(parts[1], "\"")
			case "DOCKER_CERT_PATH":
				certPath = strings.Trim(parts[1], "\"")
			case "DOCKER_TLS_VERIFY":
				tlsVerify = len(parts[1]) > 0
			}
		}
	}
	var client *docker.Client
	if len(certPath) > 0 {
		cert := filepath.Join(certPath, "cert.pem")
		key := filepath.Join(certPath, "key.pem")
		ca := filepath.Join(certPath, "ca.pem")
		client, err = docker.NewVersionedTLSClient(dockerHost, cert, key, ca, "")
	} else {
		client, err = docker.NewVersionedClient(dockerHost, "")
	}
	if err != nil {
		return nil, nil, errors.NewError("could not get Docker client for machine %s", name).WithCause(err)
	}
	client.SkipServerVersionCheck = true

	var httpClient *http.Client
	if len(certPath) > 0 {
		tlscOptions := tlsconfig.Options{
			CAFile:             filepath.Join(certPath, "ca.pem"),
			CertFile:           filepath.Join(certPath, "cert.pem"),
			KeyFile:            filepath.Join(certPath, "key.pem"),
			InsecureSkipVerify: !tlsVerify,
		}
		tlsc, tlsErr := tlsconfig.Client(tlscOptions)
		if tlsErr != nil {
			return nil, nil, errors.NewError("could not create TLS config client for machine %s", name).WithCause(tlsErr)
		}
		httpClient = &http.Client{
			Transport: net.SetTransportDefaults(&http.Transport{
				TLSClientConfig: tlsc,
			}),
		}
	}

	engineAPIClient, err := dockerclient.NewClient(dockerHost, "", httpClient, nil)
	if err != nil {
		return nil, nil, errors.NewError("cannot create Docker engine API client").WithCause(err)
	}

	return client, engineAPIClient, nil
}

// IsAvailable returns true if the docker-machine executable can be found in the PATH
func IsAvailable() bool {
	_, err := exec.LookPath(dockerMachineBinary())
	return err != nil
}

// determineMachineMemory determines a reasonable default for machine memory
// TODO: implement linux & windows
func determineMachineMemory() int {
	if runtime.GOOS == "darwin" {
		output, _, err := localcmd.New("sysctl").Args("-n", "hw.memsize").Output()
		if err == nil {
			mem, perr := strconv.ParseInt(strings.TrimSpace(output), 10, 64)
			if perr == nil {
				return int(mem / (1024 * 1024 * 2)) // half of available megs
			}
		}
	}
	return defaultMachineMemory
}

// determineMachineProcs determines a reasonable default for machine processors
// TODO: implement linux & windows
func determineMachineProcessors() int {
	if runtime.GOOS == "darwin" {
		output, _, err := localcmd.New("sysctl").Args("-n", "hw.logicalcpu").Output()
		if err == nil {
			cpus, aerr := strconv.Atoi(strings.TrimSpace(output))
			if aerr == nil {
				return cpus // use all cpus
			}
		}
	}
	return defaultMachineProcessors
}

func dockerMachineBinary() string {
	if runtime.GOOS == "windows" {
		return "docker-machine.exe"
	}
	return "docker-machine"
}
