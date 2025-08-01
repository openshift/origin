package rosacli

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

type Version struct {
	ChannelGroup string `json:"channel_group,omitempty"`
	RawID        string `json:"raw_id,omitempty"`
}

type Encryption struct {
	KmsKeyArn            string `json:"kms_key_arn,omitempty"`
	EtcdEncryptionKmsArn string `json:"etcd_encryption_kms_arn,omitempty"`
}

type Properties struct {
	ProvisionShardID string `json:"provision_shard_id,omitempty"`
}

type Sts struct {
	RoleArn             string `json:"role_arn,omitempty"`
	SupportRoleArn      string `json:"support_role_arn,omitempty"`
	WorkerRoleArn       string `json:"worker_role_arn,omitempty"`
	ControlPlaneRoleArn string `json:"control_plane_role_arn,omitempty"`
	OidcConfigID        string `json:"oidc_config_id,omitempty"`
	OperatorRolesPrefix string `json:"operator_roles_prefix,omitempty"`
}

type AWS struct {
	Sts Sts `json:"sts,omitempty"`
}

type Proxy struct {
	Enabled         bool   `json:"enabled,omitempty"`
	Http            string `json:"http,omitempty"`
	Https           string `json:"https,omitempty"`
	TrustBundleFile string `json:"trust_bundle_file,omitempty"`
}

type Subnets struct {
	PrivateSubnetIds string `json:"private_subnet_ids,omitempty"`
	PublicSubnetIds  string `json:"public_subnet_ids,omitempty"`
}

type Nodes struct {
	Replicas    string `json:"replicas,omitempty"`
	MinReplicas string `json:"min_replicas,omitempty"`
	MaxReplicas string `json:"max_replicas,omitempty"`
}

type Autoscaling struct {
	Enabled bool `json:"enabled,omitempty"`
}

type ClusterConfig struct {
	DisableScpChecks          bool        `json:"disable_scp_checks,omitempty"`
	DisableWorkloadMonitoring bool        `json:"disable_workload_monitoring,omitempty"`
	EnableCustomerManagedKey  bool        `json:"enable_customer_managed_key,omitempty"`
	EtcdEncryption            bool        `json:"etcd_encryption,omitempty"`
	Fips                      bool        `json:"fips,omitempty"`
	Hypershift                bool        `json:"hypershift,omitempty"`
	MultiAZ                   bool        `json:"multi_az,omitempty"`
	Private                   bool        `json:"private,omitempty"`
	PrivateLink               bool        `json:"private_link,omitempty"`
	Sts                       bool        `json:"sts,omitempty"`
	AuditLogArn               string      `json:"audit_log_arn,omitempty"`
	AvailabilityZones         string      `json:"availability_zones,omitempty"`
	DefaultMpLabels           string      `json:"default_mp_labels,omitempty"`
	Ec2MetadataHttpTokens     string      `json:"ec2_metadata_http_tokens,omitempty"`
	Name                      string      `json:"name,omitempty"`
	Region                    string      `json:"region,omitempty"`
	Tags                      string      `json:"tags,omitempty"`
	WorkerDiskSize            string      `json:"worker_disk_size,omitempty"`
	Autoscaling               Autoscaling `json:"autoscaling,omitempty"`
	Aws                       AWS         `json:"aws,omitempty"`
	Encryption                Encryption  `json:"encryption,omitempty"`
	Nodes                     Nodes       `json:"nodes,omitempty"`
	Properties                Properties  `json:"properties,omitempty"`
	Proxy                     Proxy       `json:"proxy,omitempty"`
	Subnets                   Subnets     `json:"subnets,omitempty"`
	Version                   Version     `json:"version,omitempty"`
}

func ParseClusterProfile() (*ClusterConfig, error) {
	filePath := getClusterConfigFile()
	// Load the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading JSON file: %v", err)
	}

	// Parse the JSON data into the ClusterConfig struct
	var config ClusterConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON data: %v", err)
	}

	return &config, nil
}

// Get the cluster config file
func getClusterConfigFile() string {
	sharedDir := os.Getenv("SHARED_DIR")
	return path.Join(sharedDir, "cluster-config")
}

func GetClusterID() (clusterID string) {
	clusterID = getClusterIDENVExisted()
	if clusterID != "" {
		return
	}

	if _, err := os.Stat(getClusterIDFile()); err != nil {
		logger.Errorf("Cluster id file not existing")
		return ""
	}
	fileCont, _ := os.ReadFile(getClusterIDFile())
	clusterID = string(fileCont)
	return
}

// Get the cluster config file, for jean chen
func getClusterIDFile() string {
	sharedDir := os.Getenv("SHARED_DIR")
	return path.Join(sharedDir, "cluster-id")
}

// Get the clusterID env.
func getClusterIDENVExisted() string {
	return os.Getenv("CLUSTER_ID")
}
