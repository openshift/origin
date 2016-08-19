package dockercfg

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/pflag"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

//TODO: Remove this code once the methods in Kubernetes kubelet/dockertools/config.go are public

const (
	PushAuthType       = "PUSH_DOCKERCFG_PATH"
	PullAuthType       = "PULL_DOCKERCFG_PATH"
	PullSourceAuthType = "PULL_SOURCE_DOCKERCFG_PATH_"
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
		dockercfgPath = GetDockercfgFile(pathForAuthType)
	} else {
		dockercfgPath = GetDockercfgFile("")
	}
	if len(dockercfgPath) == 0 {
		glog.V(3).Infof("Could not locate a docker config file")
		return docker.AuthConfiguration{}, false
	}
	if _, err := os.Stat(dockercfgPath); err != nil {
		glog.V(3).Infof("Problem accessing %s: %v", dockercfgPath, err)
		return docker.AuthConfiguration{}, false
	}

	var cfg credentialprovider.DockerConfig
	var err error
	if strings.HasSuffix(dockercfgPath, kapi.DockerConfigJsonKey) || strings.HasSuffix(dockercfgPath, "config.json") {
		cfg, err = readDockerConfigJson(dockercfgPath)
	} else if strings.HasSuffix(dockercfgPath, kapi.DockerConfigKey) {
		cfg, err = readDockercfg(dockercfgPath)
	}

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
	return docker.AuthConfiguration{
		Username:      authConfs[0].Username,
		Password:      authConfs[0].Password,
		Email:         authConfs[0].Email,
		ServerAddress: authConfs[0].ServerAddress,
	}, true
}

// GetDockercfgFile returns the path to the dockercfg file
func GetDockercfgFile(path string) string {
	var cfgPath string
	if path != "" {
		cfgPath = path
		// There are 3 valid ways to specify docker config in a secret.
		// 1) with a .dockerconfigjson key pointing to a .docker/config.json file (the key used by k8s for
		//    dockerconfigjson type secrets and the new docker cfg format)
		// 2) with a .dockercfg key+file (the key used by k8s for dockercfg type secrets and the old docker format)
		// 3) with a config.json file because you created your secret using "oc secrets new mysecret .docker/config.json"
		//    so you automatically got a key named config.json containing the new docker cfg format content.
		// we will check to see which one was provided in that priority order.
		if _, err := os.Stat(filepath.Join(path, kapi.DockerConfigJsonKey)); err == nil {
			cfgPath = filepath.Join(path, kapi.DockerConfigJsonKey)
		} else if _, err := os.Stat(filepath.Join(path, kapi.DockerConfigKey)); err == nil {
			cfgPath = filepath.Join(path, kapi.DockerConfigKey)
		} else if _, err := os.Stat(filepath.Join(path, "config.json")); err == nil {
			cfgPath = filepath.Join(path, "config.json")
		}
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

// readDockerConfigJson reads the contents of a .docker/config.json file into a map
// with server name keys and AuthEntry values
func readDockerConfigJson(filePath string) (cfg credentialprovider.DockerConfig, err error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	var config credentialprovider.DockerConfigJson
	if err = json.Unmarshal(content, &config); err != nil {
		return
	}
	cfg = config.Auths
	return
}
