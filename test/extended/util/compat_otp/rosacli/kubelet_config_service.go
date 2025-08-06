package rosacli

import (
	"bytes"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
	"gopkg.in/yaml.v3"
)

type KubeletConfigService interface {
	ResourcesCleaner

	DescribeKubeletConfig(clusterID string) (bytes.Buffer, error)
	ReflectKubeletConfigDescription(result bytes.Buffer) *KubeletConfigDescription
	EditKubeletConfig(clusterID string, flags ...string) (bytes.Buffer, error)
	DeleteKubeletConfig(clusterID string, flags ...string) (bytes.Buffer, error)
	CreateKubeletConfig(clusterID string, flags ...string) (bytes.Buffer, error)
}

type kubeletConfigService struct {
	ResourcesService

	created map[string]bool
}

func NewKubeletConfigService(client *Client) KubeletConfigService {
	return &kubeletConfigService{
		ResourcesService: ResourcesService{
			client: client,
		},
		created: make(map[string]bool),
	}
}

// Struct for the 'rosa describe kubeletconfig' output
type KubeletConfigDescription struct {
	PodPidsLimit int `yaml:"Pod Pids Limit,omitempty"`
}

// Describe Kubeletconfig
func (k *kubeletConfigService) DescribeKubeletConfig(clusterID string) (bytes.Buffer, error) {
	describe := k.client.Runner.
		Cmd("describe", "kubeletconfig").
		CmdFlags("-c", clusterID)

	return describe.Run()
}

// Pasrse the result of 'rosa describe kubeletconfig' to the KubeletConfigDescription struct
func (k *kubeletConfigService) ReflectKubeletConfigDescription(result bytes.Buffer) *KubeletConfigDescription {
	res := new(KubeletConfigDescription)
	theMap, _ := k.client.Parser.TextData.Input(result).Parse().YamlToMap()
	data, _ := yaml.Marshal(&theMap)
	yaml.Unmarshal(data, res)
	return res
}

// Edit the kubeletconfig
func (k *kubeletConfigService) EditKubeletConfig(clusterID string, flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	editCluster := k.client.Runner.
		Cmd("edit", "kubeletconfig").
		CmdFlags(combflags...)
	return editCluster.Run()
}

// Delete the kubeletconfig
func (k *kubeletConfigService) DeleteKubeletConfig(clusterID string, flags ...string) (output bytes.Buffer, err error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	editCluster := k.client.Runner.
		Cmd("delete", "kubeletconfig").
		CmdFlags(combflags...)
	output, err = editCluster.Run()
	if err == nil {
		k.created[clusterID] = false
	}
	return
}

// Create the kubeletconfig
func (k *kubeletConfigService) CreateKubeletConfig(clusterID string, flags ...string) (output bytes.Buffer, err error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	createCluster := k.client.Runner.
		Cmd("create", "kubeletconfig").
		CmdFlags(combflags...)
	output, err = createCluster.Run()
	if err == nil {
		k.created[clusterID] = true
	}
	return
}

func (k *kubeletConfigService) CleanResources(clusterID string) (errors []error) {
	if k.created[clusterID] {
		logger.Infof("Remove remaining kubelet config")
		_, err := k.DeleteKubeletConfig(clusterID)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return
}
