package dockercfg

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/pflag"
)

//TODO: Remove this code once the methods in Kubernetes kubelet/dockertools/config.go are public

// Default docker registry server
const defaultRegistryServer = "https://index.docker.io/v1/"

// Helper contains all the valid config options for reading the local dockercfg file
type Helper struct {
}

// NewHelper creates a Flags object with the default values set.
func NewHelper() *Helper {
	return &Helper{}
}

// InstallFlags installs the Docker flag helper into a FlagSet with the default
// options and default values from the Helper object.
func (_ *Helper) InstallFlags(flags *pflag.FlagSet) {
}

// GetDockerAuth returns a valid Docker AuthConfiguration entry, and whether it was read
// from the local dockercfg file
func (_ *Helper) GetDockerAuth(registry string) (docker.AuthConfiguration, bool) {
	var authCfg docker.AuthConfiguration
	dockercfgPath := getDockercfgFile("")
	if _, err := os.Stat(dockercfgPath); err != nil {
		return authCfg, false
	}
	cfg, err := readDockercfg(dockercfgPath)
	if err != nil {
		return authCfg, false
	}
	server := registry
	if server == "" {
		server = defaultRegistryServer
	}
	entry, ok := cfg[server]
	if !ok {
		return authCfg, false
	}
	uname, pass, err := getCredentials(entry.Auth)
	if err != nil {
		return authCfg, false
	}
	authCfg.Username = uname
	authCfg.Password = pass
	return authCfg, true
}

// getDockercfgFile returns the path to the dockercfg file
func getDockercfgFile(path string) string {
	var cfgPath string
	if path != "" {
		cfgPath = path
	} else if os.Getenv("DOCKERCFG_PATH") != "" {
		cfgPath = os.Getenv("DOCKERCFG_PATH")
	} else if currentUser, err := user.Current(); err == nil {
		cfgPath = filepath.Join(currentUser.HomeDir, ".dockercfg")
	}
	return cfgPath
}

// authEntry is a single entry for a given server in a
// .dockercfg file
type authEntry struct {
	Auth  string `json:auth`
	Email string `json:email`
}

// dockercfg represents the contents of a .dockercfg file
type dockercfg map[string]authEntry

// readDockercfg reads the contents of a .dockercfg file into a map
// with server name keys and AuthEntry values
func readDockercfg(filePath string) (cfg dockercfg, err error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	cfg = dockercfg{}
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
