package schema

// ClusterListIntent ...
type ClusterListIntent struct {
	APIVersion string           `json:"api_version"`
	Entities   []*ClusterIntent `json:"entities,omitempty"`
	Metadata   *ListMetadata    `json:"metadata"`
}

// ClusterIntent ...
type ClusterIntent struct {
	APIVersion string `json:"api_version,omitempty"`

	Metadata *Metadata `json:"metadata"`

	Spec *Cluster `json:"spec,omitempty"`

	Status *ClusterDefStatus `json:"status,omitempty"`
}

// Cluster ...
type Cluster struct {
	Name      string           `json:"name,omitempty"`
	Resources *ClusterResource `json:"resources,omitempty"`
}

// ClusterDefStatus ...
type ClusterDefStatus struct {
	State       string             `json:"state,omitempty"`
	MessageList []*MessageResource `json:"message_list,omitempty"`
	Name        string             `json:"name,omitempty"`
	Resources   *ClusterObj        `json:"resources,omitempty"`
}

// ClusterObj ...
type ClusterObj struct {
	Nodes             *ClusterNodes    `json:"nodes,omitempty"`
	Config            *ClusterConfig   `json:"config,omitempty"`
	Network           *ClusterNetwork  `json:"network,omitempty"`
	Analysis          *ClusterAnalysis `json:"analysis,omitempty"`
	RuntimeStatusList []string         `json:"runtime_status_list,omitempty"`
}

// ClusterNodes ...
type ClusterNodes struct {
	HypervisorServerList []*HypervisorServer `json:"hypervisor_server_list,omitempty"`
}

// HypervisorServer ...
type HypervisorServer struct {
	IP      string `json:"ip,omitempty"`
	Version string `json:"version,omitempty"`
	Type    string `json:"type,omitempty"`
}

// ClusterResource ...
type ClusterResource struct {
	Config            *ConfigClusterSpec `json:"config,omitempty"`
	Network           *ClusterNetwork    `json:"network,omitempty"`
	RunTimeStatusList []string           `json:"runtime_status_list,omitempty"`
}

// ConfigClusterSpec ...
type ConfigClusterSpec struct {
	GpuDriverVersion              string                      `json:"gpu_driver_version,omitempty"`
	ClientAuth                    *ClientAuth                 `json:"client_auth,omitempty"`
	AuthorizedPublicKeyList       []*PublicKey                `json:"authorized_public_key_list,omitempty"`
	SoftwareMap                   map[string]ClusterSoftware  `json:"software_map,omitempty"`
	EncryptionStatus              string                      `json:"encryption_status,omitempty"`
	RedundancyFactor              int64                       `json:"redundancy_factor,omitempty"`
	CertificationSigningInfo      *CertificationSigningInfo   `json:"certification_signing_info,omitempty"`
	SupportedInformationVerbosity string                      `json:"supported_information_verbosity,omitempty"`
	ExternalConfigurations        *ExternalConfigurationsSpec `json:"external_configurations,omitempty"`
	EnabledFeatureList            []string                    `json:"enabled_feature_list,omitempty"`
	Timezone                      string                      `json:"timezone,omitempty"`
	OperationMode                 string                      `json:"operation_mode,omitempty"`
}

type ClusterSoftware struct {
	// software type
	// Required: true
	SoftwareType string `json:"software_type"`

	// Current software status.
	Status string `json:"status,omitempty"`

	// version
	// Required: true
	Version string `json:"version"`
}

// PublicKey ...
type PublicKey struct {
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

// ClientAuth ...
type ClientAuth struct {
	Status  string `json:"status,omitempty"`
	CaChain string `json:"ca_chain,omitempty"`
	Name    string `json:"name,omitempty"`
}

// ExternalConfigurationsSpec ...
type ExternalConfigurationsSpec struct {
	CitrixConnectorConfig *CitrixConnectorConfigDetailsSpec `json:"citrix_connector_config,omitempty"`
}

// CitrixConnectorConfigDetailsSpec ...
type CitrixConnectorConfigDetailsSpec struct {
	CitrixVMReferenceList []*Reference                `json:"citrix_connector_config,omitempty"`
	ClientSecret          string                      `json:"client_secret,omitempty"`
	CustomerID            string                      `json:"customer_id,omitempty"`
	ClientID              string                      `json:"client_id,omitempty"`
	ResourceLocation      *CitrixResourceLocationSpec `json:"resource_location,omitempty"`
}

// CitrixResourceLocationSpec ...
type CitrixResourceLocationSpec struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// ClusterNetwork ...
type ClusterNetwork struct {
	MasqueradingPort       int64                   `json:"masquerading_port,omitempty"`
	MasqueradingIP         string                  `json:"masquerading_ip,omitempty"`
	ExternalIP             string                  `json:"external_ip,omitempty"`
	HTTPProxyList          []*ClusterNetworkEntity `json:"http_proxy_list,omitempty"`
	SMTPServer             *SMTPServer             `json:"smtp_server,omitempty"`
	NTPServerIPList        []string                `json:"ntp_server_ip_list,omitempty"`
	ExternalSubnet         string                  `json:"external_subnet,omitempty"`
	NFSSubnetWhitelist     []string                `json:"nfs_subnet_whitelist,omitempty"`
	ExternalDataServicesIP string                  `json:"external_data_services_ip,omitempty"`
	DomainServer           *ClusterDomainServer    `json:"domain_server,omitempty"`
	NameServerIPList       []string                `json:"name_server_ip_list,omitempty"`
	HTTPProxyWhitelist     []*HTTPProxyWhitelist   `json:"http_proxy_whitelist,omitempty"`
	InternalSubnet         string                  `json:"internal_subnet,omitempty"`
}

// HTTPProxyWhitelist ...
type HTTPProxyWhitelist struct {
	Target     *string `json:"target,omitempty"`
	TargetType *string `json:"target_type,omitempty"`
}

// ClusterDomainServer ...
type ClusterDomainServer struct {
	Nameserver        *string      `json:"nameserver,omitempty"`
	Name              *string      `json:"name,omitempty"`
	DomainCredentials *Credentials `json:"external_data_services_ip,omitempty"`
}

// SMTPServer ...
type SMTPServer struct {
	Type         *string               `json:"type,omitempty"`
	EmailAddress *string               `json:"email_address,omitempty"`
	Server       *ClusterNetworkEntity `json:"server,omitempty"`
}

// ClusterNetworkEntity ...
type ClusterNetworkEntity struct {
	Credentials   *Credentials `json:"credentials,omitempty"`
	ProxyTypeList []*string    `json:"proxy_type_list,omitempty"`
	Address       *Address     `json:"address,omitempty"`
}

// Credentials ...
type Credentials struct {
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
}

// CertificationSigningInfo ...
type CertificationSigningInfo struct {
	City             *string `json:"city,omitempty"`
	CommonNameSuffix *string `json:"common_name_suffix,omitempty"`
	State            *string `json:"state,omitempty"`
	CountryCode      *string `json:"country_code,omitempty"`
	CommonName       *string `json:"common_name,omitempty"`
	Organization     *string `json:"organization,omitempty"`
	EmailAddress     *string `json:"email_address,omitempty"`
}

// ClusterConfig ...
type ClusterConfig struct {
	GpuDriverVersion              *string                    `json:"gpu_driver_version,omitempty"`
	ClientAuth                    *ClientAuth                `json:"client_auth,omitempty"`
	AuthorizedPublicKeyList       []*PublicKey               `json:"authorized_public_key_list,omitempty"`
	SoftwareMap                   *SoftwareMap               `json:"software_map,omitempty"`
	EncryptionStatus              *string                    `json:"encryption_status,omitempty"`
	SslKey                        *SslKey                    `json:"ssl_key,omitempty"`
	ServiceList                   []*string                  `json:"service_list,omitempty"`
	SupportedInformationVerbosity *string                    `json:"supported_information_verbosity,omitempty"`
	CertificationSigningInfo      *CertificationSigningInfo  `json:"certification_signing_info,omitempty"`
	RedundancyFactor              *int64                     `json:"redundancy_factor,omitempty"`
	ExternalConfigurations        *ExternalConfigurations    `json:"external_configurations,omitempty"`
	OperationMode                 *string                    `json:"operation_mode,omitempty"`
	CaCertificateList             []*CaCert                  `json:"ca_certificate_list,omitempty"`
	EnabledFeatureList            []*string                  `json:"enabled_feature_list,omitempty"`
	IsAvailable                   *bool                      `json:"is_available,omitempty"`
	Build                         *BuildInfo                 `json:"build,omitempty"`
	Timezone                      *string                    `json:"timezone,omitempty"`
	ClusterArch                   *string                    `json:"cluster_arch,omitempty"`
	ManagementServerList          []*ClusterManagementServer `json:"management_server_list,omitempty"`
}

// ClusterAnalysis ...
type ClusterAnalysis struct {
	VMEfficiencyMap *VMEfficiencyMap `json:"vm_efficiency_map,omitempty"`
}

// VMEfficiencyMap ...
type VMEfficiencyMap struct {
	BullyVMNum           *string `json:"bully_vm_num,omitempty"`
	ConstrainedVMNum     *string `json:"constrained_vm_num,omitempty"`
	DeadVMNum            *string `json:"dead_vm_num,omitempty"`
	InefficientVMNum     *string `json:"inefficient_vm_num,omitempty"`
	OverprovisionedVMNum *string `json:"overprovisioned_vm_num,omitempty"`
}

// SoftwareMapValues ...
type SoftwareMapValues struct {
	SoftwareType *string `json:"software_type,omitempty"`
	Status       *string `json:"status,omitempty"`
	Version      *string `json:"version,omitempty"`
}

// SoftwareMap ...
type SoftwareMap struct {
	NCC *SoftwareMapValues `json:"ncc,omitempty"`
	NOS *SoftwareMapValues `json:"nos,omitempty"`
}

// ClusterManagementServer ...
type ClusterManagementServer struct {
	IP         *string   `json:"ip,omitempty"`
	DrsEnabled *bool     `json:"drs_enabled,omitempty"`
	StatusList []*string `json:"status_list,omitempty"`
	Type       *string   `json:"type,omitempty"`
}

// BuildInfo ...
type BuildInfo struct {
	CommitID      *string `json:"commit_id,omitempty"`
	FullVersion   *string `json:"full_version,omitempty"`
	CommitDate    *string `json:"commit_date,omitempty"`
	Version       *string `json:"version,omitempty"`
	ShortCommitID *string `json:"short_commit_id,omitempty"`
	BuildType     *string `json:"build_type,omitempty"`
}

// CaCert ...
type CaCert struct {
	CaName      *string `json:"ca_name,omitempty"`
	Certificate *string `json:"certificate,omitempty"`
}

// ExternalConfigurations ...
type ExternalConfigurations struct {
	CitrixConnectorConfig *CitrixConnectorConfigDetails `json:"citrix_connector_config,omitempty"`
}

// CitrixConnectorConfigDetails ...
type CitrixConnectorConfigDetails struct {
	CitrixVMReferenceList *[]Reference            `json:"citrix_vm_reference_list,omitempty"`
	ClientSecret          *string                 `json:"client_secret,omitempty"`
	CustomerID            *string                 `json:"customer_id,omitempty"`
	ClientID              *string                 `json:"client_id,omitempty"`
	ResourceLocation      *CitrixResourceLocation `json:"resource_location,omitempty"`
}

// CitrixResourceLocation ...
type CitrixResourceLocation struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// SslKey ...
type SslKey struct {
	KeyType        *string                   `json:"key_type,omitempty"`
	KeyName        *string                   `json:"key_name,omitempty"`
	SigningInfo    *CertificationSigningInfo `json:"signing_info,omitempty"`
	ExpireDatetime *string                   `json:"expire_datetime,omitempty"`
}
