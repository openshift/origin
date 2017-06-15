package brokerapi

type Service struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	Bindable        bool                    `json:"bindable"`
	Tags            []string                `json:"tags,omitempty"`
	PlanUpdatable   bool                    `json:"plan_updateable"`
	Plans           []ServicePlan           `json:"plans"`
	Requires        []RequiredPermission    `json:"requires,omitempty"`
	Metadata        *ServiceMetadata        `json:"metadata,omitempty"`
	DashboardClient *ServiceDashboardClient `json:"dashboard_client,omitempty"`
}

type ServiceDashboardClient struct {
	ID          string `json:"id"`
	Secret      string `json:"secret"`
	RedirectURI string `json:"redirect_uri"`
}

type ServicePlan struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Free        *bool                `json:"free,omitempty"`
	Bindable    *bool                `json:"bindable,omitempty"`
	Metadata    *ServicePlanMetadata `json:"metadata,omitempty"`
}

type ServicePlanMetadata struct {
	DisplayName string            `json:"displayName,omitempty"`
	Bullets     []string          `json:"bullets,omitempty"`
	Costs       []ServicePlanCost `json:"costs,omitempty"`
}

type ServicePlanCost struct {
	Amount map[string]float64 `json:"amount"`
	Unit   string             `json:"unit"`
}

type ServiceMetadata struct {
	DisplayName         string `json:"displayName,omitempty"`
	ImageUrl            string `json:"imageUrl,omitempty"`
	LongDescription     string `json:"longDescription,omitempty"`
	ProviderDisplayName string `json:"providerDisplayName,omitempty"`
	DocumentationUrl    string `json:"documentationUrl,omitempty"`
	SupportUrl          string `json:"supportUrl,omitempty"`
}

func FreeValue(v bool) *bool {
	return &v
}

func BindableValue(v bool) *bool {
	return &v
}

type RequiredPermission string

const (
	PermissionRouteForwarding = RequiredPermission("route_forwarding")
	PermissionSyslogDrain     = RequiredPermission("syslog_drain")
	PermissionVolumeMount     = RequiredPermission("volume_mount")
)
