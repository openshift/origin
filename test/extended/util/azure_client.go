package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"

	azcoreto "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2020-10-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/containerregistry/mgmt/2019-05-01/containerregistry"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// AzureSession is an object representing session for subscription
type AzureSession struct {
	SubscriptionID string
	Authorizer     autorest.Authorizer
}

// NewAzureSessionFromEnv new azure session from env credentials
func NewAzureSessionFromEnv() (*AzureSession, error) {
	authorizer, azureSessErr := auth.NewAuthorizerFromEnvironment()
	if azureSessErr != nil {
		e2e.Logf("New Azure Session from ENV error: %v", azureSessErr)
		return nil, azureSessErr
	}
	sess := AzureSession{
		SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"),
		Authorizer:     authorizer,
	}
	return &sess, nil
}

// GetAzureCredentialFromCluster gets Azure credentials from cluster and loads them as environment variables
func GetAzureCredentialFromCluster(oc *CLI) (string, error) {
	credential, getSecErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/azure-credentials", "-n", "kube-system", "-o=jsonpath={.data}").Output()
	if getSecErr != nil {
		return "", nil
	}

	type azureCredentials struct {
		AzureClientID       string `json:"azure_client_id,omitempty"`
		AzureClientSecret   string `json:"azure_client_secret,omitempty"`
		AzureSubscriptionID string `json:"azure_subscription_id,omitempty"`
		AzureTenantID       string `json:"azure_tenant_id,omitempty"`
		AzureResourceGroup  string `json:"azure_resourcegroup,omitempty"`
		AzureResourcePrefix string `json:"azure_resource_prefix,omitempty"`
	}
	azureCreds := azureCredentials{}
	if err := json.Unmarshal([]byte(credential), &azureCreds); err != nil {
		return "", err
	}

	azureClientID, err := base64.StdEncoding.DecodeString(azureCreds.AzureClientID)
	if err != nil {
		return "", err
	}

	azureClientSecret, err := base64.StdEncoding.DecodeString(azureCreds.AzureClientSecret)
	if err != nil {
		return "", err
	}

	azureSubscriptionID, err := base64.StdEncoding.DecodeString(azureCreds.AzureSubscriptionID)
	if err != nil {
		return "", err
	}

	azureTenantID, err := base64.StdEncoding.DecodeString(azureCreds.AzureTenantID)
	if err != nil {
		return "", err
	}

	azureResourceGroup, err := base64.StdEncoding.DecodeString(azureCreds.AzureResourceGroup)
	if err != nil {
		return "", err
	}

	azureResourcePrefix, err := base64.StdEncoding.DecodeString(azureCreds.AzureResourcePrefix)
	if err != nil {
		return "", err
	}
	os.Setenv("AZURE_CLIENT_ID", string(azureClientID))
	os.Setenv("AZURE_CLIENT_SECRET", string(azureClientSecret))
	os.Setenv("AZURE_SUBSCRIPTION_ID", string(azureSubscriptionID))
	os.Setenv("AZURE_TENANT_ID", string(azureTenantID))
	os.Setenv("AZURE_RESOURCE_PREFIX", string(azureResourcePrefix))
	e2e.Logf("Azure credentials successfully loaded.")
	return string(azureResourceGroup), nil
}

// getUserAssignedIdentitiesClient get user assigned identities client
func getUserAssignedIdentitiesClient(sess *AzureSession) msi.UserAssignedIdentitiesClient {
	msiClient := msi.NewUserAssignedIdentitiesClient(sess.SubscriptionID)
	msiClient.Authorizer = sess.Authorizer
	return msiClient
}

// getRoleAssignmentsClient get role assignments client
func getRoleAssignmentsClient(sess *AzureSession) authorization.RoleAssignmentsClient {
	roleAssignmentsClient := authorization.NewRoleAssignmentsClient(sess.SubscriptionID)
	roleAssignmentsClient.Authorizer = sess.Authorizer
	return roleAssignmentsClient
}

// getRegistryClient get registry client
func getRegistryClient(sess *AzureSession) containerregistry.RegistriesClient {
	registryClient := containerregistry.NewRegistriesClient(sess.SubscriptionID)
	registryClient.Authorizer = sess.Authorizer
	return registryClient
}

// CreateAzureContainerRegistry create azure container registry
func CreateAzureContainerRegistry(sess *AzureSession, registryName string, resourceGroupName string, location string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registryClient := getRegistryClient(sess)

	registry := containerregistry.Registry{
		Location: &location,
		RegistryProperties: &containerregistry.RegistryProperties{
			AdminUserEnabled: azcoreto.Ptr(true),
		},
		Sku: &containerregistry.Sku{
			Name: containerregistry.Basic,
		},
	}

	future, err := registryClient.Create(ctx, resourceGroupName, registryName, registry)
	if err != nil {
		e2e.Logf("Failed to create registry: %v", err)
		return err
	}
	if err := future.WaitForCompletionRef(ctx, registryClient.Client); err != nil {
		e2e.Logf("Failed to wait for completion of registry creation: %v", err)
		return err
	}
	e2e.Logf("Azure Container Registry is created")
	return nil
}

// GetAzureContainerRepositoryCredential get azure container repository credential
func GetAzureContainerRepositoryCredential(sess *AzureSession, registryName string, resourceGroupName string) (string, string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registryClient := getRegistryClient(sess)
	loginResult, err := registryClient.ListCredentials(ctx, resourceGroupName, registryName)
	if err != nil {
		e2e.Logf("Failed to get the registry credentials: %v", err)
		return "", "", err
	}

	passwordValue := ""
	for _, password := range *loginResult.Passwords {
		passwordValue = *password.Value
		break
	}
	return *loginResult.Username, passwordValue, nil
}

// DeleteAzureContainerRegistry deletes azure container registry
func DeleteAzureContainerRegistry(sess *AzureSession, registryName string, resourceGroupName string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registryClient := getRegistryClient(sess)
	future, err := registryClient.Delete(ctx, resourceGroupName, registryName)
	if err != nil {
		e2e.Logf("Failed to delete registry: %v", err)
		return err
	}
	if err := future.WaitForCompletionRef(ctx, registryClient.Client); err != nil {
		e2e.Logf("Failed to wait for completion of registry deletion: %v", err)
		return err
	}
	e2e.Logf("Azure Container Registry is deleted")
	return nil
}
