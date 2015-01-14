package provider

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet"
	"github.com/fsouza/go-dockerclient"

	osclient "github.com/openshift/origin/pkg/client"
	secretapi "github.com/openshift/origin/pkg/secret/api"
)

const (
	SecretDomain    = "secret.openshift.com"
	EnvSecretType   = "ENV"
	FileSecretType  = "FILE"
	RequiredMarker  = "!"
	DefaultFileMode = os.FileMode(0644)
)

func NewSecretProvider(rootDir string, osClient osclient.Interface, kClient kclient.Interface) kubelet.SecurityContextProvider {
	return &secretProvider{
		rootDir:  rootDir,
		osClient: osClient,
		kClient:  kClient,
	}
}

type secretProvider struct {
	rootDir  string
	osClient osclient.Interface
	kClient  kclient.Interface
}

type envSecret struct {
	name     string
	required bool
}

type fileSecret struct {
	path     string
	owner    int
	group    int
	mode     os.FileMode
	required bool
}

func (p *secretProvider) getPath(pod *kapi.BoundPod, container *kapi.Container, secretName string) string {
	return filepath.Join(p.rootDir, pod.Namespace, pod.Name, "secrets", container.Name, "volumes", secretName)
}

func (p *secretProvider) SecureContainer(
	boundPod *kapi.BoundPod,
	container *kapi.Container,
	createOptions *docker.CreateContainerOptions) error {

	pod, err := p.kClient.Pods(boundPod.Namespace).Get(boundPod.Name)
	if err != nil {
		return err
	}
	envSecrets := p.getContainerEnvSecrets(pod, container.Name)
	if len(envSecrets) > 0 {
		envs, err := p.provisionEnvSecrets(pod.Namespace, envSecrets)
		if err != nil {
			return err
		}
		createOptions.Config.Env = append(createOptions.Config.Env, envs...)
	}
	return nil
}

func (p *secretProvider) SecureHostConfig(
	boundPod *kapi.BoundPod,
	container *kapi.Container,
	hostConfig *docker.HostConfig) error {

	pod, err := p.kClient.Pods(boundPod.Namespace).Get(boundPod.Name)
	if err != nil {
		return err
	}
	fileSecrets := p.getContainerFileSecrets(pod, container.Name)
	if len(fileSecrets) > 0 {
		dir, err := p.setupSecretsDir(pod.Namespace, pod.Name, container.Name)
		binds, err := p.provisionFileSecrets(pod.Namespace, dir, fileSecrets)
		if err != nil {
			return err
		}
		hostConfig.Binds = append(hostConfig.Binds, binds...)
	}
	return nil
}

// getContainerEnvSecrets returns a map of secret descriptors to be passed as environment variables to the
// container to be created
// The pod is expected to contain annotations in the format of:
// secret.openshift.com/[secretName] = ENV:<containerName>:<EnvVarName>[:!]
// If ! (optional) is present at the end of the secret value, it means that it is required and that the container
// should not run without it being present.
// example: secret.openshift.com/dbpassword = ENV:frontend:DB_PASSWORD
func (p *secretProvider) getContainerEnvSecrets(pod *kapi.Pod, containerName string) map[string]envSecret {
	result := make(map[string]envSecret)
	for key, value := range pod.Annotations {
		if isSecret(key) && isEnvSecret(value) && matchesContainer(value, containerName) {
			if envSecret, ok := parseEnvSecret(value); ok {
				result[secretName(key)] = *envSecret
			}
		}
	}
	return result
}

// getContainerFileSecrets returns a map of secret descriptors to be passed as files to the
// container to be created
// The pod is expected to contain annotations in the format of:
// secret.openshift.com/[secretName] = FILE:<containerName>:[<file_path>[:filemode][[:ownerid][:groupid]][:!], [...]]
// filemode is optional and is expected to contain an octal value such as 0644 or 644.
// ownerid and groupid are optional, but if ownerid is specified, group id is expected to be there as well
// Both ownerid and groupid are expected to contain a numeric uid
// If ! is present at the end of the secret value, it means that it is required and that the container
// should not run without it being present.
// After the container name, multiple files can be specified (comma-separated) and their number must match the number
// of data items in the referenced secret
// example: secret.openshift.com/sshkey = FILE:builder:/root/.ssh/id_rsa:400:0:0,/root/.ssh/id_rsa.pub:644:0:0
func (p *secretProvider) getContainerFileSecrets(pod *kapi.Pod, containerName string) map[string][]fileSecret {
	result := make(map[string][]fileSecret)
	for key, value := range pod.Annotations {
		if isSecret(key) && isFileSecret(value) && matchesContainer(value, containerName) {
			if fileSecret, ok := parseFileSecret(value); ok {
				result[secretName(key)] = fileSecret
			}
		}
	}
	return result
}

func (p *secretProvider) provisionEnvSecrets(namespace string, secrets map[string]envSecret) ([]string, error) {
	result := []string{}
	for secret, descriptor := range secrets {
		secretData, err := p.fetchSecretAsText(namespace, secret)
		if err != nil {
			if descriptor.required {
				return nil, err
			}
			continue
		}
		result = append(result, fmt.Sprintf("%s=%s", descriptor.name, secretData[0]))
	}
	return result, nil
}

func (p *secretProvider) provisionFileSecrets(namespace string, dir string, secrets map[string][]fileSecret) ([]string, error) {
	result := []string{}
	for secret, descriptors := range secrets {
		secretData, err := p.fetchSecretBytes(namespace, secret)
		if err != nil {
			for _, desc := range descriptors {
				if desc.required {
					return nil, err
				}
			}
			continue
		}
		fileNames, err := writeFileSecrets(dir, descriptors, secretData)
		if err != nil {
			return nil, err
		}
		for i := range fileNames {
			result = append(result, fmt.Sprintf("%s:%s", fileNames[i], descriptors[i].path))
		}
	}
	return result, nil
}

func (p *secretProvider) fetchSecretBytes(namespace string, name string) ([][]byte, error) {
	secret, err := p.osClient.Secrets(namespace).Get(name)
	result := [][]byte{}
	if err != nil {
		return nil, err
	}
	if secret.Type == secretapi.Base64SecretType {
		for _, s := range secret.Data {
			bytes, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, err
			}
			result = append(result, bytes)
		}
	} else {
		for _, s := range secret.Data {
			result = append(result, []byte(s))
		}
	}
	return result, nil
}

func (p *secretProvider) fetchSecretAsText(namespace string, name string) ([]string, error) {
	secret, err := p.osClient.Secrets(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

func (p *secretProvider) setupSecretsDir(namespace, name, container string) (string, error) {
	dir := filepath.Join(p.rootDir, namespace, name, "secrets", container, "volumes")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	return dir, nil
}

func isSecret(annotationKey string) bool {
	if strings.Contains(annotationKey, "/") {
		parts := strings.Split(annotationKey, "/")
		if len(parts) == 2 && parts[0] == SecretDomain {
			return true
		}
	}
	return false
}

func secretName(annotationKey string) string {
	parts := strings.Split(annotationKey, "/")
	return parts[1]
}

func isEnvSecret(value string) bool {
	return isSecretType(value, EnvSecretType)
}

func isFileSecret(value string) bool {
	return isSecretType(value, FileSecretType)
}

func isSecretType(value string, secretType string) bool {
	parts := strings.Split(value, ":")
	if parts[0] == secretType {
		return true
	}
	return false
}

func matchesContainer(value string, containerName string) bool {
	parts := strings.Split(value, ":")
	if parts[1] == containerName {
		return true
	}
	return false
}

func parseEnvSecret(value string) (*envSecret, bool) {
	parts := strings.Split(value, ":")
	if len(parts) > 2 {
		result := envSecret{}
		result.name = parts[2]
		if len(parts) > 3 && parts[3] == RequiredMarker {
			result.required = true
		}
		return &result, true
	}
	return nil, false
}

func parseFileSecret(value string) ([]fileSecret, bool) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) == 3 {
		filedescs := strings.Split(parts[2], ",")
		result := []fileSecret{}
		for _, filedesc := range filedescs {
			if fileSecret, ok := parseSingleFileSecret(filedesc); ok {
				result = append(result, *fileSecret)
			}
		}
		return result, true
	}
	return nil, false
}

func parseSingleFileSecret(value string) (*fileSecret, bool) {
	parts := strings.Split(value, ":")
	result := &fileSecret{}
	if len(parts) == 0 {
		return nil, false
	}
	result.path = parts[0]
	if len(parts) > 1 {
		if parts[1] == RequiredMarker {
			result.required = true
			return result, true
		}
		modeInt, err := strconv.ParseInt(parts[1], 8, 32)
		if err != nil {
			return nil, false
		}
		result.mode = os.FileMode(modeInt)
	}
	if len(parts) > 2 {
		if parts[2] == RequiredMarker {
			result.required = true
			return result, true
		}
		if len(parts) < 4 {
			return nil, false
		}
		ownerId, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, false
		}
		groupId, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, false
		}
		result.owner = ownerId
		result.group = groupId
	}
	if len(parts) > 4 {
		if parts[4] == RequiredMarker {
			result.required = true
		}
	}
	return result, true
}

func writeFileSecrets(dir string, descriptors []fileSecret, secretData [][]byte) ([]string, error) {
	if len(secretData) < len(descriptors) {
		return nil, fmt.Errorf("Not enough secrets to satisfy number of file descriptors")
	}
	result := []string{}
	for i, desc := range descriptors {
		name := filepath.Join(dir, uuid.NewRandom().String())
		mode := desc.mode
		if mode == 0 {
			mode = DefaultFileMode
		}
		if err := ioutil.WriteFile(name, secretData[i], mode); err != nil {
			return nil, err
		}
		if err := os.Chown(name, desc.owner, desc.group); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, nil
}
