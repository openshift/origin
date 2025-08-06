package schema

// Response object for intentful operation of hosts
type HostListIntent struct {
	Entities   []*HostIntent `json:"entities,omitempty"`
	APIVersion string        `json:"api_version"`
	Metadata   *ListMetadata `json:"metadata"`
}

type HostIntent struct {
	Status     *HostDefStatus `json:"status,omitempty"`
	Spec       *Host          `json:"spec,omitempty"`
	APIVersion string         `json:"api_version,omitempty"`
	Metadata   *Metadata      `json:"metadata"`
}

type HostDefStatus struct {
	// The state of the entity
	State            string            `json:"state,omitempty"`
	MessageList      []MessageResource `json:"message_list,omitempty"`
	Name             string            `json:"name,omitempty"`
	Resources        *HostResources    `json:"resources"`
	ClusterReference *Reference        `json:"cluster_reference,omitempty"`
}

// Host resources.
type HostResources struct {
	WindowsDomain     *WindowsDomain   `json:"windows_domain,omitempty"`
	ControllerVM      *ControllerVM    `json:"controller_vm,omitempty"`
	Hypervisor        *Hypervisor      `json:"hypervisor,omitempty"`
	FailoverCluster   *FailoverCluster `json:"failover_cluster,omitempty"`
	HostType          string           `json:"host_type,omitempty"`
	SerialNumber      string           `json:"serial_number,omitempty"`
	CPUModel          string           `json:"cpu_model,omitempty"`
	NumCPUCores       int              `json:"num_cpu_cores,omitempty"`
	NumCPUSockets     int              `json:"num_cpu_sockets,omitempty"`
	MemoryCapacityMib uint64           `json:"memory_capacity_mib,omitempty"`
	IPMI              *HostIPMI        `json:"ipmi,omitempty"`
	Block             *HostBlock       `json:"block,omitempty"`
}
type HostBlock struct {
	BlockSerialNumber string `json:"block_serial_number,omitempty"`
	BlockModel        string `json:"block_model,omitempty"`
}

type HostIPMI struct {
	IP string `json:"ip,omitempty"`
}

type Hypervisor struct {
	NumVMs             int    `json:"num_vms,omitempty"`
	IP                 string `json:"ip,omitempty"`
	HypervisorFullName string `json:"hypervisor_full_name,omitempty"`
}

// Hyper-V node domain.
type WindowsDomain struct {
	// The name of the node to be renamed to during domain-join. If not given,a new name will be automatically assigned.
	Name string `json:"name,omitempty"`
	// Full name of domain.
	DomainName       string       `json:"domain_name,omitempty"`
	DomainCredential *Credentials `json:"domain_credential"`
	// Path to organization unit in the domain.
	OrganizationUnitPath string `json:"organization_unit_path,omitempty"`
	// The name prefix in the domain in case of CPS deployment.
	NamePrefix string `json:"name_prefix,omitempty"`
	// The ip of name server that can resolve the domain name. Required during joining domain.
	NameServerIP string `json:"name_server_ip,omitempty"`
}

// Host controller vm information.
type ControllerVM struct {
	// Controller VM IP address.
	IP string `json:"ip"`
	// Controller VM NAT IP address.
	NatIP      string      `json:"nat_ip,omitempty"`
	OplogUsage *OplogUsage `json:"oplog_usage,omitempty"`
	// Controller VM NAT port.
	NatPort int32 `json:"nat_port,omitempty"`
}

// oplog disk usage.
type OplogUsage struct {
	// Oplog disk size used in percentage.
	OplogDiskPct float32 `json:"oplog_disk_pct,omitempty"`
	// Size of oplog disk in bytes.
	OplogDiskSize int64 `json:"oplog_disk_size,omitempty"`
}

// Hyper-V failover cluster.
type FailoverCluster struct {
	// IP address of the failover cluster.
	IP               string       `json:"ip,omitempty"`
	Name             string       `json:"name,omitempty"`
	DomainCredential *Credentials `json:"domain_credential"`
}

// Host Definition.
type Host struct {
	// Host Name.
	Name      string         `json:"name,omitempty"`
	Resources *HostResources `json:"resources"`
}
