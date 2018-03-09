package dockermachine

import (
	"bufio"
	"bytes"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/localcmd"
	"k8s.io/apimachinery/pkg/util/net"
)

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

// Start starts up an existing Docker machine
func Start(name string) error {
	err := localcmd.New(dockerMachineBinary()).Args("start", name).Run()
	if err != nil {
		return ErrDockerMachineExec("start", err)
	}
	return nil
}

// Client returns a Docker client for the given Docker machine
func Client(name string) (dockerhelper.Interface, error) {
	output, _, err := localcmd.New(dockerMachineBinary()).Args("env", name).Output()
	if err != nil {
		return nil, ErrDockerMachineExec("env", err)
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
			return nil, errors.NewError("could not create TLS config client for machine %s", name).WithCause(tlsErr)
		}
		httpClient = &http.Client{
			Transport: net.SetTransportDefaults(&http.Transport{
				TLSClientConfig: tlsc,
			}),
		}
	}

	engineAPIClient, err := dockerclient.NewClient(dockerHost, "1.24", httpClient, nil)
	if err != nil {
		return nil, errors.NewError("cannot create Docker engine API client").WithCause(err)
	}
	return dockerhelper.NewClient(dockerHost, engineAPIClient), nil
}

func dockerMachineBinary() string {
	if runtime.GOOS == "windows" {
		return "docker-machine.exe"
	}
	return "docker-machine"
}
