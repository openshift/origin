package dockercfg

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

//TODO: Remove this code once the methods in Kubernetes kubelet/dockertools/config.go are public

// Default docker registry server
const (
	defaultRegistryServer = "https://index.docker.io/v1/"
	PushAuthType          = "PUSH_DOCKERCFG_PATH"
	PullAuthType          = "PULL_DOCKERCFG_PATH"
)

// Helper contains all the valid config options for reading the local dockercfg file
type Helper struct {
}

// NewHelper creates a Flags object with the default values set.
func NewHelper() *Helper {
	return &Helper{}
}

// InstallFlags installs the Docker flag helper into a FlagSet with the default
// options and default values from the Helper object.
func (h *Helper) InstallFlags(flags *pflag.FlagSet) {
}

// GetDockerAuth returns a valid Docker AuthConfiguration entry, and whether it was read
// from the local dockercfg file
func (h *Helper) GetDockerAuth(imageName, authType string) (docker.AuthConfiguration, bool) {
	glog.V(3).Infof("Locating docker auth for image %s and type %s", imageName, authType)
	var dockercfgPath string
	if pathForAuthType := os.Getenv(authType); len(pathForAuthType) > 0 {
		dockercfgPath = getDockercfgFile(pathForAuthType)
	} else {
		dockercfgPath = getDockercfgFile("")
	}
	if _, err := os.Stat(dockercfgPath); err != nil {
		glog.V(3).Infof("Problem accessing %s: %v", dockercfgPath, err)
		return docker.AuthConfiguration{}, false
	}
	cfg, err := readDockercfg(dockercfgPath)
	if err != nil {
		glog.Errorf("Reading %s failed: %v", dockercfgPath, err)
		return docker.AuthConfiguration{}, false
	}
	keyring := credentialprovider.BasicDockerKeyring{}
	keyring.Add(cfg)
	authConfs, found := keyring.Lookup(imageName)
	if !found || len(authConfs) == 0 {
		return docker.AuthConfiguration{}, false
	}
	glog.V(3).Infof("Using %s user for Docker authentication for image %s", authConfs[0].Username, imageName)
	return authConfs[0], true
}

// getDockercfgFile returns the path to the dockercfg file
func getDockercfgFile(path string) string {
	var cfgPath string
	if path != "" {
		cfgPath = path
	} else if os.Getenv("DOCKERCFG_PATH") != "" {
		cfgPath = os.Getenv("DOCKERCFG_PATH")
	} else if currentUser, err := user.Current(); err == nil {
		cfgPath = filepath.Join(currentUser.HomeDir, ".docker", "config.json")
	}
	glog.V(5).Infof("Using Docker authentication configuration in '%s'", cfgPath)
	return cfgPath
}

// readDockercfg reads the contents of a .dockercfg file into a map
// with server name keys and AuthEntry values
func readDockercfg(filePath string) (cfg credentialprovider.DockerConfig, err error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}
	return
}

// getCredentials parses an auth string inside a dockercfg file into
// a username and password
func getCredentials(auth string) (username, password string, err error) {
	creds, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return
	}
	unamepass := strings.Split(string(creds), ":")
	username = unamepass[0]
	password = unamepass[1]
	return
}
