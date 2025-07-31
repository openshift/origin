package rosacli

import (
	"bytes"
	"errors"
)

type ResourcesCleaner interface {
	CleanResources(clusterID string) []error
}

type CLDNamedResourceService interface {
	ResourcesCleaner

	List(clusterID string) (bytes.Buffer, error)
	Describe(clusterID string, name string) (bytes.Buffer, error)
	Create(clusterID string, name string, flags ...string) (bytes.Buffer, error)
	Edit(clusterID string, name string, flags ...string) (bytes.Buffer, error)
	Delete(clusterID string, name string) (bytes.Buffer, error)
}

type ResourcesService struct {
	client *Client
}

type Client struct {
	// Clients
	Runner *runner
	Parser *Parser

	// services
	// Keep in alphabetical order
	Cluster            ClusterService
	IDP                IDPService
	Ingress            IngressService
	KubeletConfig      KubeletConfigService
	MachinePool        MachinePoolService
	MachinePoolUpgrade MachinePoolUpgradeService
	NetworkVerifier    NetworkVerifierService
	OCMResource        OCMResourceService
	TuningConfig       TuningConfigService
	User               UserService
	Version            VersionService
}

func NewClient() *Client {
	runner := NewRunner()
	parser := NewParser()

	client := &Client{
		Runner: runner,
		Parser: parser,
	}

	// Keep in alphabetical order
	client.Cluster = NewClusterService(client)
	client.IDP = NewIDPService(client)
	client.Ingress = NewIngressService(client)
	client.KubeletConfig = NewKubeletConfigService(client)
	client.MachinePool = NewMachinePoolService(client)
	client.MachinePoolUpgrade = NewMachinePoolUpgradeService(client)
	client.NetworkVerifier = NewNetworkVerifierService(client)
	client.OCMResource = NewOCMResourceService(client)
	client.TuningConfig = NewTuningConfigService(client)
	client.User = NewUserService(client)
	client.Version = NewVersionService(client)

	return client
}

func NewSensitiveClient() *Client {
	client := NewClient()
	client.Runner.Sensitive(true)
	return client
}

func (c *Client) CleanResources(clusterID string) error {
	var errorList []error

	// Keep in logical order
	errorList = append(errorList, c.Version.CleanResources(clusterID)...)
	errorList = append(errorList, c.TuningConfig.CleanResources(clusterID)...)
	errorList = append(errorList, c.MachinePoolUpgrade.CleanResources(clusterID)...)
	errorList = append(errorList, c.MachinePool.CleanResources(clusterID)...)
	errorList = append(errorList, c.Ingress.CleanResources(clusterID)...)
	errorList = append(errorList, c.NetworkVerifier.CleanResources(clusterID)...)
	errorList = append(errorList, c.KubeletConfig.CleanResources(clusterID)...)
	errorList = append(errorList, c.User.CleanResources(clusterID)...)
	errorList = append(errorList, c.IDP.CleanResources(clusterID)...)
	errorList = append(errorList, c.OCMResource.CleanResources(clusterID)...)
	errorList = append(errorList, c.Cluster.CleanResources(clusterID)...)

	return errors.Join(errorList...)

}
