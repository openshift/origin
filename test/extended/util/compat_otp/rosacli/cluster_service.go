package rosacli

import (
	"bytes"
	"strings"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
	"gopkg.in/yaml.v3"
)

type ClusterService interface {
	ResourcesCleaner

	DescribeCluster(clusterID string) (bytes.Buffer, error)
	ReflectClusterDescription(result bytes.Buffer) (*ClusterDescription, error)
	DescribeClusterAndReflect(clusterID string) (*ClusterDescription, error)
	List() (bytes.Buffer, error)
	CreateDryRun(clusterName string, flags ...string) (bytes.Buffer, error)
	EditCluster(clusterID string, flags ...string) (bytes.Buffer, error)
	DeleteUpgrade(flags ...string) (bytes.Buffer, error)

	IsHostedCPCluster(clusterID string) (bool, error)
	IsSTSCluster(clusterID string) (bool, error)
	IsPrivateCluster(clusterID string) (bool, error)
	IsUsingReusableOIDCConfig(clusterID string) (bool, error)
	GetClusterVersion(clusterID string) (Version, error)
	IsBYOVPCCluster(clusterID string) (bool, error)
}

type clusterService struct {
	ResourcesService
}

func NewClusterService(client *Client) ClusterService {
	return &clusterService{
		ResourcesService: ResourcesService{
			client: client,
		},
	}
}

// Struct for the 'rosa describe cluster' output
type ClusterDescription struct {
	Name                  string                   `yaml:"Name,omitempty"`
	ID                    string                   `yaml:"ID,omitempty"`
	ExternalID            string                   `yaml:"External ID,omitempty"`
	OpenshiftVersion      string                   `yaml:"OpenShift Version,omitempty"`
	ChannelGroup          string                   `yaml:"Channel Group,omitempty"`
	DNS                   string                   `yaml:"DNS,omitempty"`
	AWSAccount            string                   `yaml:"AWS Account,omitempty"`
	AWSBillingAccount     string                   `yaml:"AWS Billing Account,omitempty"`
	APIURL                string                   `yaml:"API URL,omitempty"`
	ConsoleURL            string                   `yaml:"Console URL,omitempty"`
	Region                string                   `yaml:"Region,omitempty"`
	MultiAZ               string                   `yaml:"Multi-AZ,omitempty"`
	State                 string                   `yaml:"State,omitempty"`
	Private               string                   `yaml:"Private,omitempty"`
	Created               string                   `yaml:"Created,omitempty"`
	DetailsPage           string                   `yaml:"Details Page,omitempty"`
	ControlPlane          string                   `yaml:"Control Plane,omitempty"`
	ScheduledUpgrade      string                   `yaml:"Scheduled Upgrade,omitempty"`
	InfraID               string                   `yaml:"Infra ID,omitempty"`
	AdditionalTrustBundle string                   `yaml:"Additional trust bundle,omitempty"`
	Ec2MetadataHttpTokens string                   `yaml:"Ec2 Metadata Http Tokens,omitempty"`
	Availability          []map[string]string      `yaml:"Availability,omitempty"`
	Nodes                 []map[string]interface{} `yaml:"Nodes,omitempty"`
	Network               []map[string]string      `yaml:"Network,omitempty"`
	Proxy                 []map[string]string      `yaml:"Proxy,omitempty"`
	STSRoleArn            string                   `yaml:"Role (STS) ARN,omitempty"`
	// STSExternalID            string                   `yaml:"STS External ID,omitempty"`
	SupportRoleARN           string              `yaml:"Support Role ARN,omitempty"`
	OperatorIAMRoles         []string            `yaml:"Operator IAM Roles,omitempty"`
	InstanceIAMRoles         []map[string]string `yaml:"Instance IAM Roles,omitempty"`
	ManagedPolicies          string              `yaml:"Managed Policies,omitempty"`
	UserWorkloadMonitoring   string              `yaml:"User Workload Monitoring,omitempty"`
	FIPSMod                  string              `yaml:"FIPS mode,omitempty"`
	OIDCEndpointURL          string              `yaml:"OIDC Endpoint URL,omitempty"`
	PrivateHostedZone        []map[string]string `yaml:"Private Hosted Zone,omitempty"`
	AuditLogForwarding       string              `yaml:"Audit Log Forwarding,omitempty"`
	ProvisioningErrorMessage string              `yaml:"Provisioning Error Message,omitempty"`
	ProvisioningErrorCode    string              `yaml:"Provisioning Error Code,omitempty"`
	LimitedSupport           []map[string]string `yaml:"Limited Support,omitempty"`
	AuditLogRoleARN          string              `yaml:"Audit Log Role ARN,omitempty"`
	FailedInflightChecks     string              `yaml:"Failed Inflight Checks,omitempty"`
}

func (c *clusterService) DescribeCluster(clusterID string) (bytes.Buffer, error) {
	describe := c.client.Runner.
		Cmd("describe", "cluster").
		CmdFlags("-c", clusterID)

	return describe.Run()
}

func (c *clusterService) DescribeClusterAndReflect(clusterID string) (res *ClusterDescription, err error) {
	output, err := c.DescribeCluster(clusterID)
	if err != nil {
		return nil, err
	}
	return c.ReflectClusterDescription(output)
}

// Pasrse the result of 'rosa describe cluster' to the RosaClusterDescription struct
func (c *clusterService) ReflectClusterDescription(result bytes.Buffer) (res *ClusterDescription, err error) {
	var data []byte
	res = new(ClusterDescription)
	theMap, err := c.client.
		Parser.
		TextData.
		Input(result).
		Parse().
		TransformOutput(func(str string) (newStr string) {
			// Apply transformation to avoid issue with the list of Inflight checks below
			// It will consider
			newStr = strings.Replace(str, "Failed Inflight Checks:", "Failed Inflight Checks: |", 1)
			newStr = strings.ReplaceAll(newStr, "\t", "  ")
			return
		}).
		YamlToMap()
	if err != nil {
		return
	}
	data, err = yaml.Marshal(&theMap)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(data, res)
	return res, err
}

func (c *clusterService) List() (bytes.Buffer, error) {
	list := c.client.Runner.Cmd("list", "cluster")
	return list.Run()
}

func (c *clusterService) CreateDryRun(clusterName string, flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"-c", clusterName, "--dry-run"}, flags...)
	createDryRun := c.client.Runner.
		Cmd("create", "cluster").
		CmdFlags(combflags...)
	return createDryRun.Run()
}

func (c *clusterService) EditCluster(clusterID string, flags ...string) (bytes.Buffer, error) {
	combflags := append([]string{"-c", clusterID}, flags...)
	editCluster := c.client.Runner.
		Cmd("edit", "cluster").
		CmdFlags(combflags...)
	return editCluster.Run()
}

func (c *clusterService) DeleteUpgrade(flags ...string) (bytes.Buffer, error) {
	DeleteUpgrade := c.client.Runner.
		Cmd("delete", "upgrade").
		CmdFlags(flags...)
	return DeleteUpgrade.Run()
}

func (c *clusterService) CleanResources(clusterID string) (errors []error) {
	logger.Debugf("Nothing releated to cluster was done there")
	return
}

// Check if the cluster is hosted-cp cluster
func (c *clusterService) IsHostedCPCluster(clusterID string) (bool, error) {
	jsonData, err := c.getJSONClusterDescription(clusterID)
	if err != nil {
		return false, err
	}
	return jsonData.DigBool("hypershift", "enabled"), nil
}

// Check if the cluster is sts cluster. hosted-cp cluster is also treated as sts cluster
func (c *clusterService) IsSTSCluster(clusterID string) (bool, error) {
	jsonData, err := c.getJSONClusterDescription(clusterID)
	if err != nil {
		return false, err
	}
	return jsonData.DigBool("aws", "sts", "enabled"), nil
}

// Check if the cluster is private cluster
func (c *clusterService) IsPrivateCluster(clusterID string) (bool, error) {
	jsonData, err := c.getJSONClusterDescription(clusterID)
	if err != nil {
		return false, err
	}
	return jsonData.DigString("api", "listening") == "internal", nil
}

// Check if the cluster is using reusable oidc-config
func (c *clusterService) IsUsingReusableOIDCConfig(clusterID string) (bool, error) {
	jsonData, err := c.getJSONClusterDescription(clusterID)
	if err != nil {
		return false, err
	}
	return jsonData.DigBool("aws", "sts", "oidc_config", "reusable"), nil
}

// Get cluster version
func (c *clusterService) GetClusterVersion(clusterID string) (clusterVersion Version, err error) {
	var clusterConfig *ClusterConfig
	clusterConfig, err = ParseClusterProfile()
	if err != nil {
		return
	}

	if clusterConfig.Version.RawID != "" {
		clusterVersion = clusterConfig.Version
	} else {
		// Else retrieve from cluster description
		var jsonData *jsonData
		jsonData, err = c.getJSONClusterDescription(clusterID)
		if err != nil {
			return
		}
		clusterVersion = Version{
			RawID:        jsonData.DigString("version", "raw_id"),
			ChannelGroup: jsonData.DigString("version", "channel_group"),
		}
	}
	return
}

func (c *clusterService) getJSONClusterDescription(clusterID string) (*jsonData, error) {
	c.client.Runner.JsonFormat()
	output, err := c.DescribeCluster(clusterID)
	if err != nil {
		logger.Errorf("it met error when describeCluster in IsUsingReusableOIDCConfig is %v", err)
		return nil, err
	}
	c.client.Runner.UnsetFormat()
	return c.client.Parser.JsonData.Input(output).Parse(), nil
}

// Check if the cluster is byo vpc cluster
func (c *clusterService) IsBYOVPCCluster(clusterID string) (bool, error) {
	jsonData, err := c.getJSONClusterDescription(clusterID)
	if err != nil {
		return false, err
	}
	if len(jsonData.DigString("aws", "subnet_ids")) > 0 {
		return true, nil
	}
	return false, nil
}
