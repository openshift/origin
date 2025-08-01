package azure

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider"
	"sigs.k8s.io/yaml"
)

// LoadConfigFile uses the cluster to fetch the cloud provider config from `openshift-config/cloud-provider-config` config map's config key.
// It then uses the `AZURE_AUTH_LOCATION` to load the credentials for Azure API and update the cloud provider config with the client secret. In-cluster cloud provider config
// uses Azure Managed Identity attached to virtual machines to provide Azure API access, while the e2e tests are usually run from outside the cluster and therefore need explicit auth creds.
func LoadConfigFile() ([]byte, error) {
	// LoadClientset but don't set the UserAgent to include the current test name because
	// we don't run any test yet and this call panics
	client, err := e2e.LoadClientset(true)
	if err != nil {
		return nil, err
	}
	config, err := cloudProviderConfigFromCluster(client.CoreV1())
	if err != nil {
		return nil, err
	}

	settings, err := getAuthFile()
	if err != nil {
		return nil, err
	}
	config.AADClientID = settings.ClientID
	config.AADClientSecret = settings.ClientSecret
	config.UseManagedIdentityExtension = false
	config.UseInstanceMetadata = false

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func cloudProviderConfigFromCluster(client clientcorev1.ConfigMapsGetter) (*provider.Config, error) {
	cm, err := client.ConfigMaps("openshift-config").Get(context.Background(), "cloud-provider-config", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data, ok := cm.Data["config"]
	if !ok {
		return nil, errors.New("No cloud provider config was set in openshift-config/cloud-provider-config")
	}
	config := &provider.Config{}
	if err := yaml.Unmarshal([]byte(data), config); err != nil {
		return nil, err
	}
	return config, nil
}

// loads the auth file using the file provided by AZURE_AUTH_LOCATION
// this mimics the function https://godoc.org/github.com/Azure/go-autorest/autorest/azure/auth#GetSettingsFromFile which is not currently available with vendor Azure SDK.
func getAuthFile() (*file, error) {
	fileLocation := os.Getenv("AZURE_AUTH_LOCATION")
	if fileLocation == "" {
		return nil, errors.New("environment variable AZURE_AUTH_LOCATION is not set")
	}

	contents, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		return nil, err
	}

	authFile := file{}
	err = json.Unmarshal(contents, &authFile)
	if err != nil {
		return nil, err
	}

	return &authFile, nil
}

// File represents the authentication file
type file struct {
	ClientID                string `json:"clientId,omitempty"`
	ClientSecret            string `json:"clientSecret,omitempty"`
	SubscriptionID          string `json:"subscriptionId,omitempty"`
	TenantID                string `json:"tenantId,omitempty"`
	ActiveDirectoryEndpoint string `json:"activeDirectoryEndpointUrl,omitempty"`
	ResourceManagerEndpoint string `json:"resourceManagerEndpointUrl,omitempty"`
	GraphResourceID         string `json:"activeDirectoryGraphResourceId,omitempty"`
	SQLManagementEndpoint   string `json:"sqlManagementEndpointUrl,omitempty"`
	GalleryEndpoint         string `json:"galleryEndpointUrl,omitempty"`
	ManagementEndpoint      string `json:"managementEndpointUrl,omitempty"`
}
