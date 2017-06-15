package brokerapi

type EmptyResponse struct{}

type ErrorResponse struct {
	Error       string `json:"error,omitempty"`
	Description string `json:"description"`
}

type CatalogResponse struct {
	Services []Service `json:"services"`
}

type ProvisioningResponse struct {
	DashboardURL  string `json:"dashboard_url,omitempty"`
	OperationData string `json:"operation,omitempty"`
}

type UpdateResponse struct {
	OperationData string `json:"operation,omitempty"`
}

type DeprovisionResponse struct {
	OperationData string `json:"operation,omitempty"`
}

type LastOperationResponse struct {
	State       LastOperationState `json:"state"`
	Description string             `json:"description,omitempty"`
}

type ExperimentalVolumeMountBindingResponse struct {
	Credentials     interface{}               `json:"credentials"`
	SyslogDrainURL  string                    `json:"syslog_drain_url,omitempty"`
	RouteServiceURL string                    `json:"route_service_url,omitempty"`
	VolumeMounts    []ExperimentalVolumeMount `json:"volume_mounts,omitempty"`
}

type ExperimentalVolumeMount struct {
	ContainerPath string                         `json:"container_path"`
	Mode          string                         `json:"mode"`
	Private       ExperimentalVolumeMountPrivate `json:"private"`
}

type ExperimentalVolumeMountPrivate struct {
	Driver  string `json:"driver"`
	GroupID string `json:"group_id"`
	Config  string `json:"config"`
}
