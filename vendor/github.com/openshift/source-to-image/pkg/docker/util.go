package docker

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/distribution/reference"
	cliconfig "github.com/docker/docker/cli/config"
	"github.com/docker/docker/client"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/constants"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	utillog "github.com/openshift/source-to-image/pkg/util/log"
	"github.com/openshift/source-to-image/pkg/util/user"
)

var (
	// log is a placeholder until the builders pass an output stream down client
	// facing libraries should not be using log
	log = utillog.StderrLog

	// DefaultEntrypoint is the default entry point used when starting containers
	DefaultEntrypoint = []string{"/usr/bin/env"}
)

// AuthConfigurations maps a registry name to an AuthConfig, as used for
// example in the .dockercfg file
type AuthConfigurations struct {
	Configs map[string]api.AuthConfig
}

type dockerConfig struct {
	Auth  string `json:"auth"`
	Email string `json:"email"`
}

const (
	// maxErrorOutput is the maximum length of the error output saved for
	// processing
	maxErrorOutput  = 1024
	defaultRegistry = "https://index.docker.io/v1/"
)

// GetImageRegistryAuth retrieves the appropriate docker client authentication
// object for a given image name and a given set of client authentication
// objects.
func GetImageRegistryAuth(auths *AuthConfigurations, imageName string) api.AuthConfig {
	log.V(5).Infof("Getting docker credentials for %s", imageName)
	if auths == nil {
		return api.AuthConfig{}
	}
	ref, err := parseNamedDockerImageReference(imageName)
	if err != nil {
		log.V(0).Infof("error: Failed to parse docker reference %s", imageName)
		return api.AuthConfig{}
	}
	if ref.Registry != "" {
		if auth, ok := auths.Configs[ref.Registry]; ok {
			log.V(5).Infof("Using %s[%s] credentials for pulling %s", auth.Email, ref.Registry, imageName)
			return auth
		}
	}
	if auth, ok := auths.Configs[defaultRegistry]; ok {
		log.V(5).Infof("Using %s credentials for pulling %s", auth.Email, imageName)
		return auth
	}
	return api.AuthConfig{}
}

// namedDockerImageReference points to a Docker image.
type namedDockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}

// parseNamedDockerImageReference parses a Docker pull spec string into a
// NamedDockerImageReference.
func parseNamedDockerImageReference(spec string) (namedDockerImageReference, error) {
	var ref namedDockerImageReference

	namedRef, err := reference.ParseNormalizedNamed(spec)
	if err != nil {
		return ref, err
	}

	name := namedRef.Name()
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ":.") && name[:i] != "localhost") {
		ref.Name = name
	} else {
		ref.Registry, ref.Name = name[:i], name[i+1:]
	}

	if named, ok := namedRef.(reference.NamedTagged); ok {
		ref.Tag = named.Tag()
	}

	if named, ok := namedRef.(reference.Canonical); ok {
		ref.ID = named.Digest().String()
	}

	// It's not enough just to use the reference.ParseNamed(). We have to fill
	// ref.Namespace from ref.Name
	if i := strings.IndexRune(ref.Name, '/'); i != -1 {
		ref.Namespace, ref.Name = ref.Name[:i], ref.Name[i+1:]
	}

	return ref, nil
}

// LoadImageRegistryAuth loads and returns the set of client auth objects from
// a docker config json file.
func LoadImageRegistryAuth(dockerCfg io.Reader) *AuthConfigurations {
	auths, err := NewAuthConfigurations(dockerCfg)
	if err != nil {
		log.V(0).Infof("error: Unable to load docker config: %v", err)
		return nil
	}
	return auths
}

// begin next 3 methods borrowed from go-dockerclient

// NewAuthConfigurations finishes creating the auth config array s2i pulls from
// any auth config file it is pointed to when started from the command line
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

// parseDockerConfig does the json unmarshalling of the data we read from the
// file
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
		if len(conf.Auth) == 0 {
			continue
		}
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

// StreamContainerIO starts a goroutine to take data from the reader and
// redirect it to the log function (typically we pass in glog.Error for stderr
// and glog.Info for stdout. The caller should wrap glog functions in a closure
// to ensure accurate line numbers are reported:
// https://github.com/openshift/source-to-image/issues/558 .
// StreamContainerIO returns a channel which is closed after the reader is
// closed.
func StreamContainerIO(r io.Reader, errOutput *string, logFn func(string)) <-chan struct{} {
	c := make(chan struct{}, 1)
	go func() {
		reader := bufio.NewReader(r)
		for {
			text, err := reader.ReadString('\n')
			if text != "" {
				logFn(text)
			}
			if errOutput != nil && len(*errOutput) < maxErrorOutput {
				*errOutput += text + "\n"
			}
			if err != nil {
				if log.Is(2) && err != io.EOF {
					log.V(0).Infof("error: Error reading docker stdout/stderr: %#v", err)
				}
				break
			}
		}
		close(c)
	}()
	return c
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

// PullImage pulls the Docker image specified by name taking the pull policy
// into the account.
func PullImage(name string, d Docker, policy api.PullPolicy) (*PullResult, error) {
	if len(policy) == 0 {
		return nil, errors.New("the policy for pull image must be set")
	}

	var (
		image *api.Image
		err   error
	)
	switch policy {
	case api.PullIfNotPresent:
		image, err = d.CheckAndPullImage(name)
	case api.PullAlways:
		log.Infof("Pulling image %q ...", name)
		image, err = d.PullImage(name)
	case api.PullNever:
		log.Infof("Checking if image %q is available locally ...", name)
		image, err = d.CheckImage(name)
	}
	return &PullResult{Image: image, OnBuild: d.IsImageOnBuild(name)}, err
}

// CheckAllowedUser retrieves the execution users for a Docker image and
// checks that user against an allowed range of uids.
// - If the range of users is not empty, then the user on the Docker image
// needs to be a numeric user
// - The user's uid must be contained by the range(s) specified by the uids
// Rangelist
// - If build image uses an assemble user (via a command override or an
// image label), that user must be within the allowed range of uids.
// - If the image contains ONBUILD instructions and those instructions also
// contain any USER directives, then all users specified by those USER directives
// must meet the uid range criteria as well.
func CheckAllowedUser(d Docker, imageName string, uids user.RangeList, isOnbuild bool, assembleUserConfig string) error {
	if uids == nil || uids.Empty() {
		return nil
	}

	// OnBuild users always need to be checked for layered builds
	// Only return error if a user is not allowed, otherwise continue
	if isOnbuild {
		onBuildUsers, err := extractOnBuildUsers(d, imageName)
		if err != nil {
			return err
		}
		for _, usr := range onBuildUsers {
			if !user.IsUserAllowed(usr, &uids) {
				return s2ierr.NewUserNotAllowedError(imageName, true)
			}
		}
	}

	// P1: Assemble user configuration
	if len(assembleUserConfig) > 0 {
		if !user.IsUserAllowed(assembleUserConfig, &uids) {
			// Pass in the override, since assembleUser can come from the image label
			return s2ierr.NewAssembleUserNotAllowedError(imageName, true)
		}
		return nil
	}

	// P2: Assemble user label in image
	assembleUser, err := extractAssembleUser(d, imageName)
	if err != nil {
		return err
	}
	if len(assembleUser) > 0 {
		if !user.IsUserAllowed(assembleUser, &uids) {
			// Pass in the override, since assembleUser can come from the image label
			return s2ierr.NewAssembleUserNotAllowedError(imageName, false)
		}
		return nil
	}

	// Default - image user
	imageUser, err := extractImageUser(d, imageName)
	if err != nil {
		return err
	}
	if !user.IsUserAllowed(imageUser, &uids) {
		return s2ierr.NewUserNotAllowedError(imageName, false)
	}

	return nil
}

func extractUser(userSpec string) string {
	if strings.Contains(userSpec, ":") {
		parts := strings.SplitN(userSpec, ":", 2)
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(userSpec)
}

func extractImageUser(d Docker, imageName string) (string, error) {
	imageUserSpec, err := d.GetImageUser(imageName)
	if err != nil {
		return "", err
	}
	imageUser := extractUser(imageUserSpec)
	return imageUser, nil
}

var dockerLineDelim = regexp.MustCompile(`[\t\v\f\r ]+`)

// extractOnBuildUsers checks a list of Docker ONBUILD instructions for user
// directives. It returns a list of users specified in the ONBUILD directives.
func extractOnBuildUsers(d Docker, imageName string) ([]string, error) {
	cmds, err := d.GetOnBuild(imageName)
	var users []string
	if err != nil {
		return users, err
	}
	for _, line := range cmds {
		parts := dockerLineDelim.Split(line, 2)
		if strings.ToLower(parts[0]) != "user" {
			continue
		}
		users = append(users, extractUser(parts[1]))
	}
	return users, nil
}

// CheckReachable returns if the Docker daemon is reachable from s2i
func (d *stiDocker) CheckReachable() error {
	_, err := d.Version()
	return err
}

func pullAndCheck(image string, docker Docker, pullPolicy api.PullPolicy, config *api.Config) (*PullResult, error) {
	r, err := PullImage(image, docker, pullPolicy)
	if err != nil {
		return nil, err
	}

	err = CheckAllowedUser(docker, image, config.AllowedUIDs, r.OnBuild, config.AssembleUser)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// GetBuilderImage processes the config and performs operations necessary to
// make the Docker image specified as BuilderImage available locally. It
// returns information about the base image, containing metadata necessary for
// choosing the right STI build strategy.
func GetBuilderImage(docker Docker, config *api.Config) (*PullResult, error) {
	return pullAndCheck(config.BuilderImage, docker, config.BuilderPullPolicy, config)
}

// GetRebuildImage obtains the metadata information for the image specified in
// a s2i rebuild operation. Assumptions are made that the build is available
// locally since it should have been previously built.
func GetRebuildImage(docker Docker, config *api.Config) (*PullResult, error) {
	return pullAndCheck(config.Tag, docker, config.BuilderPullPolicy, config)
}

// GetRuntimeImage processes the config and performs operations necessary to
// make the Docker image specified as RuntimeImage available locally.
func GetRuntimeImage(docker Docker, config *api.Config) error {
	_, err := pullAndCheck(config.RuntimeImage, docker, config.RuntimeImagePullPolicy, config)
	return err
}

// GetDefaultDockerConfig checks relevant Docker environment variables to
// provide defaults for our command line flags
func GetDefaultDockerConfig() *api.DockerConfig {
	cfg := &api.DockerConfig{}

	if cfg.Endpoint = os.Getenv("DOCKER_HOST"); cfg.Endpoint == "" {
		cfg.Endpoint = client.DefaultDockerHost
	}

	certPath := os.Getenv("DOCKER_CERT_PATH")
	if certPath == "" {
		certPath = cliconfig.Dir()
	}

	cfg.CertFile = filepath.Join(certPath, "cert.pem")
	cfg.KeyFile = filepath.Join(certPath, "key.pem")
	cfg.CAFile = filepath.Join(certPath, "ca.pem")

	if tlsVerify := os.Getenv("DOCKER_TLS_VERIFY"); tlsVerify != "" {
		cfg.TLSVerify = true
	}

	return cfg
}

// GetAssembleUser finds an assemble user on the given image.
// This functions receives the config to check if the AssembleUser was defined in command line
// If the cmd is blank, it tries to fetch the value from the Builder Image defined Label (assemble-user)
// Otherwise it follows the common flow, using the USER defined in Dockerfile
func GetAssembleUser(docker Docker, config *api.Config) (string, error) {
	if len(config.AssembleUser) > 0 {
		return config.AssembleUser, nil
	}
	return extractAssembleUser(docker, config.BuilderImage)
}

func extractAssembleUser(docker Docker, imageName string) (string, error) {
	imageData, err := docker.GetLabels(imageName)
	if err != nil {
		return "", err
	}
	return imageData[constants.AssembleUserLabel], nil
}
