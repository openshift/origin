package docker

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/docker/docker/cliconfig"
	"github.com/docker/engine-api/client"
	"github.com/openshift/origin/pkg/image/reference"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/errors"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
	"github.com/openshift/source-to-image/pkg/util/user"
)

var (
	// glog is a placeholder until the builders pass an output stream down
	// client facing libraries should not be using glog
	glog = utilglog.StderrLog

	// DefaultEntrypoint is the default entry point used when starting containers
	DefaultEntrypoint = []string{"/usr/bin/env"}
)

// AuthConfigurations maps a registry name to an AuthConfig, as used for example
// in the .dockercfg file
type AuthConfigurations struct {
	Configs map[string]api.AuthConfig
}

type dockerConfig struct {
	Auth  string `json:"auth"`
	Email string `json:"email"`
}

const (
	// maxErrorOutput is the maximum length of the error output saved for processing
	maxErrorOutput  = 1024
	defaultRegistry = "https://index.docker.io/v1/"
)

// GetImageRegistryAuth retrieves the appropriate docker client authentication object for a given
// image name and a given set of client authentication objects.
func GetImageRegistryAuth(auths *AuthConfigurations, imageName string) api.AuthConfig {
	glog.V(5).Infof("Getting docker credentials for %s", imageName)
	ref, err := reference.ParseNamedDockerImageReference(imageName)
	if err != nil {
		glog.V(0).Infof("error: Failed to parse docker reference %s", imageName)
		return api.AuthConfig{}
	}

	if ref.Registry != "" {
		if auth, ok := auths.Configs[ref.Registry]; ok {
			glog.V(5).Infof("Using %s[%s] credentials for pulling %s", auth.Email, ref.Registry, imageName)
			return auth
		}
	}
	if auth, ok := auths.Configs[defaultRegistry]; ok {
		glog.V(5).Infof("Using %s credentials for pulling %s", auth.Email, imageName)
		return auth
	}
	return api.AuthConfig{}
}

// LoadImageRegistryAuth loads and returns the set of client auth objects from a docker config
// json file.
func LoadImageRegistryAuth(dockerCfg io.Reader) *AuthConfigurations {
	auths, err := NewAuthConfigurations(dockerCfg)
	if err != nil {
		glog.V(0).Infof("error: Unable to load docker config")
		return nil
	}
	return auths
}

// begin next 3 methods borrowed from go-dockerclient

// NewAuthConfigurations finishes creating the auth config array s2i pulls
// from any auth config file it is pointed to when started from the command line
func NewAuthConfigurations(r io.Reader) (*AuthConfigurations, error) {
	var auth *AuthConfigurations
	confs, err := parseDockerConfig(r)
	if err != nil {
		return nil, err
	}
	auth, err = authConfigs(confs)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

// parseDockerConfig does the json unmarshalling of the data we read from the file
func parseDockerConfig(r io.Reader) (map[string]dockerConfig, error) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	byteData := buf.Bytes()

	confsWrapper := struct {
		Auths map[string]dockerConfig `json:"auths"`
	}{}
	if err := json.Unmarshal(byteData, &confsWrapper); err == nil {
		if len(confsWrapper.Auths) > 0 {
			return confsWrapper.Auths, nil
		}
	}

	var confs map[string]dockerConfig
	if err := json.Unmarshal(byteData, &confs); err != nil {
		return nil, err
	}
	return confs, nil
}

// authConfigs converts a dockerConfigs map to a AuthConfigurations object.
func authConfigs(confs map[string]dockerConfig) (*AuthConfigurations, error) {
	c := &AuthConfigurations{
		Configs: make(map[string]api.AuthConfig),
	}
	for reg, conf := range confs {
		data, err := base64.StdEncoding.DecodeString(conf.Auth)
		if err != nil {
			return nil, err
		}
		userpass := strings.SplitN(string(data), ":", 2)
		if len(userpass) != 2 {
			return nil, fmt.Errorf("cannot parse username/password from %s", userpass)
		}
		c.Configs[reg] = api.AuthConfig{
			Email:         conf.Email,
			Username:      userpass[0],
			Password:      userpass[1],
			ServerAddress: reg,
		}
	}
	return c, nil
}

// end block of 3 methods borrowed from go-dockerclient

// LoadAndGetImageRegistryAuth loads the set of client auth objects from a docker config file
// and returns the appropriate client auth object for a given image name.
func LoadAndGetImageRegistryAuth(dockerCfg io.Reader, imageName string) api.AuthConfig {
	auths, err := NewAuthConfigurations(dockerCfg)
	if err != nil {
		glog.V(0).Infof("error: Unable to load docker config")
		return api.AuthConfig{}
	}
	return GetImageRegistryAuth(auths, imageName)
}

// StreamContainerIO takes data from the Reader and redirects to the log function (typically we pass in
// glog.Error for stderr and glog.Info for stdout. The caller should wrap glog functions in a closure
// to ensure accurate line numbers are reported: https://github.com/openshift/source-to-image/issues/558 .
func StreamContainerIO(errStream io.Reader, errOutput *string, log func(...interface{})) {
	scanner := bufio.NewReader(errStream)
	for {
		text, err := scanner.ReadString('\n')
		if err != nil {
			// we're ignoring ErrClosedPipe, as this is information
			// the docker container ended streaming logs
			if glog.Is(2) && err != io.ErrClosedPipe && err != io.EOF {
				glog.V(0).Infof("error: Error reading docker stderr, %#v", err)
			}
			break
		}
		log(text)
		if errOutput != nil && len(*errOutput) < maxErrorOutput {
			*errOutput += text + "\n"
		}
	}
}

// TODO remove (base, tag, id)
func parseRepositoryTag(repos string) (string, string, string) {
	n := strings.Index(repos, "@")
	if n >= 0 {
		parts := strings.Split(repos, "@")
		return parts[0], "", parts[1]
	}
	n = strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, "", ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag, ""
	}
	return repos, "", ""
}

// PullImage pulls the Docker image specifies by name taking the pull policy
// into the account.
// TODO: The 'force' option will be removed
func PullImage(name string, d Docker, policy api.PullPolicy, force bool) (*PullResult, error) {
	// TODO: Remove this after we deprecate --force-pull
	if force {
		policy = api.PullAlways
	}

	if len(policy) == 0 {
		return nil, fmt.Errorf("the policy for pull image must be set")
	}

	var (
		image *api.Image
		err   error
	)
	switch policy {
	case api.PullIfNotPresent:
		image, err = d.CheckAndPullImage(name)
	case api.PullAlways:
		glog.Infof("Pulling image %q ...", name)
		image, err = d.PullImage(name)
	case api.PullNever:
		glog.Infof("Checking if image %q is available locally ...", name)
		image, err = d.CheckImage(name)
	}
	return &PullResult{Image: image, OnBuild: d.IsImageOnBuild(name)}, err
}

// CheckAllowedUser retrieves the user for a Docker image and checks that user against
// an allowed range of uids.
// - If the range of users is not empty, then the user on the Docker image needs to be a numeric user
// - The user's uid must be contained by the range(s) specified by the uids Rangelist
// - If the image contains ONBUILD instructions and those instructions also contain a USER directive,
//   then the user specified by that USER directive must meet the uid range criteria as well.
func CheckAllowedUser(d Docker, imageName string, uids user.RangeList, isOnbuild bool) error {
	if uids == nil || uids.Empty() {
		return nil
	}
	imageUserSpec, err := d.GetImageUser(imageName)
	if err != nil {
		return err
	}
	imageUser := extractUser(imageUserSpec)
	if !user.IsUserAllowed(imageUser, &uids) {
		return errors.NewUserNotAllowedError(imageName, false)
	}
	if isOnbuild {
		cmds, err := d.GetOnBuild(imageName)
		if err != nil {
			return err
		}
		if !isOnbuildAllowed(cmds, &uids) {
			return errors.NewUserNotAllowedError(imageName, true)
		}
	}
	return nil
}

var dockerLineDelim = regexp.MustCompile(`[\t\v\f\r ]+`)

// isOnbuildAllowed checks a list of Docker ONBUILD instructions for
// user directives. It ensures that any users specified by the directives
// falls within the specified range list of users.
func isOnbuildAllowed(directives []string, allowed *user.RangeList) bool {
	for _, line := range directives {
		parts := dockerLineDelim.Split(line, 2)
		if strings.ToLower(parts[0]) != "user" {
			continue
		}
		uname := extractUser(parts[1])
		if !user.IsUserAllowed(uname, allowed) {
			return false
		}
	}
	return true
}

func extractUser(userSpec string) string {
	user := userSpec
	if strings.Contains(user, ":") {
		parts := strings.SplitN(userSpec, ":", 2)
		user = parts[0]
	}
	return strings.TrimSpace(user)
}

// CheckReachable returns if the Docker daemon is reachable from s2i
func CheckReachable(config *api.Config) error {
	d, err := New(config.DockerConfig, config.PullAuthentication)
	if err != nil {
		return err
	}
	_, err = d.Version()
	return err
}

func pullAndCheck(image string, docker Docker, pullPolicy api.PullPolicy, config *api.Config, forcePull bool) (*PullResult, error) {
	r, err := PullImage(image, docker, pullPolicy, forcePull)
	if err != nil {
		return nil, err
	}

	err = CheckAllowedUser(docker, image, config.AllowedUIDs, r.OnBuild)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// GetBuilderImage processes the config and performs operations necessary to make
// the Docker image specified as BuilderImage available locally.
// It returns information about the base image, containing metadata necessary
// for choosing the right STI build strategy.
func GetBuilderImage(config *api.Config) (*PullResult, error) {
	d, err := New(config.DockerConfig, config.PullAuthentication)
	if err != nil {
		return nil, err
	}

	return pullAndCheck(config.BuilderImage, d, config.BuilderPullPolicy, config, config.ForcePull)
}

// GetRebuildImage obtains the metadata information for the image
// specified in a s2i rebuild operation.  Assumptions are made that
// the build is available locally since it should have been previously built.
func GetRebuildImage(config *api.Config) (*PullResult, error) {
	d, err := New(config.DockerConfig, config.PullAuthentication)
	if err != nil {
		return nil, err
	}

	return pullAndCheck(config.Tag, d, config.BuilderPullPolicy, config, config.ForcePull)
}

// GetRuntimeImage processes the config and performs operations necessary to make
// the Docker image specified as RuntimeImage available locally.
func GetRuntimeImage(config *api.Config, docker Docker) error {
	pullPolicy := config.RuntimeImagePullPolicy
	if len(pullPolicy) == 0 {
		pullPolicy = api.DefaultRuntimeImagePullPolicy
	}

	_, err := pullAndCheck(config.RuntimeImage, docker, pullPolicy, config, false)
	return err
}

// GetDefaultDockerConfig checks relevant Docker environment variables to
// provide defaults for our command line flags
func GetDefaultDockerConfig() *api.DockerConfig {
	cfg := &api.DockerConfig{}

	if cfg.Endpoint = os.Getenv("DOCKER_HOST"); cfg.Endpoint == "" {
		cfg.Endpoint = client.DefaultDockerHost

		// TODO: remove this when we bump engine-api to >= cf82c64276ebc2501e72b241f9fdc1e21e421743
		if runtime.GOOS == "darwin" {
			cfg.Endpoint = "unix:///var/run/docker.sock"
		}
	}

	certPath := os.Getenv("DOCKER_CERT_PATH")
	if certPath == "" {
		certPath = cliconfig.ConfigDir()
	}

	cfg.CertFile = filepath.Join(certPath, "cert.pem")
	cfg.KeyFile = filepath.Join(certPath, "key.pem")
	cfg.CAFile = filepath.Join(certPath, "ca.pem")

	if tlsVerify := os.Getenv("DOCKER_TLS_VERIFY"); tlsVerify != "" {
		cfg.TLSVerify = true
	}

	return cfg
}
