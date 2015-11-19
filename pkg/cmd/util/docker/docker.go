package docker

import (
	"os"
	"path"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

// Helper contains all the valid config options for connecting to Docker from
// a command line.
type Helper struct {
}

// NewHelper creates a Flags object with the default values set.  Use this
// to use consistent Docker client loading behavior from different contexts.
func NewHelper() *Helper {
	return &Helper{}
}

// InstallFlags installs the Docker flag helper into a FlagSet with the default
// options and default values from the Helper object.
func (_ *Helper) InstallFlags(flags *pflag.FlagSet) {
}

// GetClient returns a valid Docker client, the address of the client, or an error
// if the client couldn't be created.
func (_ *Helper) GetClient() (client *docker.Client, endpoint string, err error) {
	cfg := getDockerConfig("")
	endpoint = cfg.Endpoint

	if cfg.IsTLS() {
		client, err = docker.NewTLSClient(cfg.Endpoint, cfg.Cert(), cfg.Key(), cfg.CA())
		return
	}
	client, err = docker.NewClient(cfg.Endpoint)
	return
}

// GetClientOrExit returns a valid Docker client and the address of the client,
// or prints an error and exits.
func (h *Helper) GetClientOrExit() (*docker.Client, string) {
	client, addr, err := h.GetClient()
	if err != nil {
		glog.Fatalf("ERROR: Couldn't connect to Docker at %s.\n%v\n.", addr, err)
	}
	return client, addr
}

type dockerConfig struct {
	Endpoint string
	CertPath string
}

func (c *dockerConfig) IsTLS() bool {
	return len(c.CertPath) > 0
}

func (c *dockerConfig) Cert() string {
	return path.Join(c.CertPath, "cert.pem")
}

func (c *dockerConfig) Key() string {
	return path.Join(c.CertPath, "key.pem")
}

func (c *dockerConfig) CA() string {
	return path.Join(c.CertPath, "ca.pem")
}

func getDockerConfig(dockerEndpoint string) *dockerConfig {
	cfg := &dockerConfig{}
	if len(dockerEndpoint) > 0 {
		cfg.Endpoint = dockerEndpoint
	} else if len(os.Getenv("DOCKER_HOST")) > 0 {
		cfg.Endpoint = os.Getenv("DOCKER_HOST")
	} else {
		cfg.Endpoint = "unix:///var/run/docker.sock"
	}

	if os.Getenv("DOCKER_TLS_VERIFY") == "1" {
		cfg.CertPath = os.Getenv("DOCKER_CERT_PATH")
	}
	return cfg
}
