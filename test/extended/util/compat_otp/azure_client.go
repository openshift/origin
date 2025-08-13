package compat_otp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/containerregistry/mgmt/containerregistry"
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2020-10-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-06-01/storage"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	exutil "github.com/openshift/origin/test/extended/util"

	azcoreto "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/go-autorest/autorest/to"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

// getNicClient get nic client
func getNicClient(sess *AzureSession) network.InterfacesClient {
	nicClient := network.NewInterfacesClient(sess.SubscriptionID)
	nicClient.Authorizer = sess.Authorizer
	return nicClient
}

// getStorageClient get storage client
func getStorageClient(sess *AzureSession) storage.AccountsClient {
	storageClient := storage.NewAccountsClient(sess.SubscriptionID)
	storageClient.Authorizer = sess.Authorizer
	return storageClient
}

// GetAzureStorageAccount get Azure Storage Account
func GetAzureStorageAccount(sess *AzureSession, resourceGroupName string) (string, error) {
	storageClient := getStorageClient(sess)
	listGroupAccounts, err := storageClient.ListByResourceGroup(context.Background(), resourceGroupName)
	if err != nil {
		return "", err
	}
	for _, acc := range *listGroupAccounts.Value {
		fmt.Printf("\t%s\n", *acc.Name)
		match, err := regexp.MatchString("cluster", *acc.Name)
		if err != nil {
			return "", err
		}

		if match {
			e2e.Logf("The storage account name is %s,", *acc.Name)
			return *acc.Name, nil
		}
	}
	e2e.Logf("There is no storage account name matching regex : cluster")
	return "", nil
}

// getIPClient get publicIP client
func getIPClient(sess *AzureSession) network.PublicIPAddressesClient {
	ipClient := network.NewPublicIPAddressesClient(sess.SubscriptionID)
	ipClient.Authorizer = sess.Authorizer
	return ipClient
}

// GetAzureVMPrivateIP  get Azure vm private IP
func GetAzureVMPrivateIP(sess *AzureSession, rg, vmName string) (string, error) {
	nicClient := getNicClient(sess)
	privateIP := ""

	//find private IP
	for iter, err := nicClient.ListComplete(context.Background(), rg); iter.NotDone(); err = iter.Next() {
		if err != nil {
			return "", err
		}
		if strings.Contains(*iter.Value().Name, vmName) {
			e2e.Logf("Found int-svc VM with name %s", *iter.Value().Name)
			intF := *iter.Value().InterfacePropertiesFormat.IPConfigurations
			privateIP = *intF[0].InterfaceIPConfigurationPropertiesFormat.PrivateIPAddress
			e2e.Logf("The private IP for vm %s is %s,", vmName, privateIP)
			break
		}
	}

	return privateIP, nil

}

// GetAzureVMPublicIPByNameRegex  returns the first public IP whose name matches the given regex
func GetAzureVMPublicIPByNameRegex(sess *AzureSession, rg, publicIPNameRegex string) (string, error) {
	//find public IP
	e2e.Logf("Looking for publicIP with name matching %s", publicIPNameRegex)
	ipClient := getIPClient(sess)

	for iter, err := ipClient.ListAll(context.Background()); iter.NotDone(); err = iter.Next() {
		if err != nil {
			return "", err
		}

		for _, value := range iter.Values() {
			match, err := regexp.MatchString(publicIPNameRegex, *value.Name)
			if err != nil {
				return "", err
			}

			if match {
				e2e.Logf("The public IP with name %s is %s,", *value.Name, *value.IPAddress)
				return *value.IPAddress, nil
			}
		}
	}

	e2e.Logf("There is no public IP with its name matching regex : %s", publicIPNameRegex)
	return "", nil

}

// GetAzureVMPublicIP  get azure vm public IP
func GetAzureVMPublicIP(sess *AzureSession, rg, vmName string) (string, error) {
	publicIPName := vmName + "PublicIP"
	publicIP := ""
	//find public IP
	ipClient := getIPClient(sess)
	publicIPAtt, getIPErr := ipClient.Get(context.Background(), rg, publicIPName, "")
	if getIPErr != nil {
		return "", getIPErr
	}
	publicIP = *publicIPAtt.IPAddress
	e2e.Logf("The public IP for vm %s is %s,", vmName, publicIP)
	return publicIP, nil

}

// StartAzureVM starts the selected VM
func StartAzureVM(sess *AzureSession, vmName string, resourceGroupName string) (osr autorest.Response, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vmClient := compute.NewVirtualMachinesClient(sess.SubscriptionID)
	vmClient.Authorizer = sess.Authorizer
	future, vmErr := vmClient.Start(ctx, resourceGroupName, vmName)
	if vmErr != nil {
		e2e.Logf("cannot start vm: %v", vmErr)
		return osr, vmErr
	}

	err = future.WaitForCompletionRef(ctx, vmClient.Client)
	if err != nil {
		e2e.Logf("cannot get the vm start future response: %v", err)
		return osr, err
	}
	return future.Result(vmClient)
}

// StopAzureVM stops the selected VM
func StopAzureVM(sess *AzureSession, vmName string, resourceGroupName string) (osr autorest.Response, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vmClient := compute.NewVirtualMachinesClient(sess.SubscriptionID)
	vmClient.Authorizer = sess.Authorizer
	var skipShutdown bool = true
	// skipShutdown parameter is optional, we are taking its true value here
	future, vmErr := vmClient.PowerOff(ctx, resourceGroupName, vmName, &skipShutdown)
	if err != nil {
		e2e.Logf("cannot power off vm: %v", vmErr)
		return osr, vmErr
	}

	err = future.WaitForCompletionRef(ctx, vmClient.Client)
	if err != nil {
		e2e.Logf("cannot get the vm power off future response: %v", err)
		return osr, err
	}
	return future.Result(vmClient)
}

// GetAzureVMInstance get vm instance
func GetAzureVMInstance(sess *AzureSession, vmName string, resourceGroupName string) (string, error) {
	vmClient := compute.NewVirtualMachinesClient(sess.SubscriptionID)
	vmClient.Authorizer = sess.Authorizer
	for vm, vmErr := vmClient.ListComplete(context.Background(), resourceGroupName); vm.NotDone(); vmErr = vm.Next() {
		if vmErr != nil {
			e2e.Logf("got error while traverising RG list: %v", vmErr)
			return "", vmErr
		}
		instanceName := vm.Value()
		if *instanceName.Name == vmName {
			e2e.Logf("Azure instance found :: %s", vmName)
			return vmName, nil
		}
	}
	return "", nil
}

// GetAzureVMInstanceState get vm instance state
func GetAzureVMInstanceState(sess *AzureSession, vmName string, resourceGroupName string) (string, error) {
	var vmErr error
	vmClient := compute.NewVirtualMachinesClient(sess.SubscriptionID)
	vmClient.Authorizer = sess.Authorizer
	vmStatus, vmErr := vmClient.Get(context.Background(), resourceGroupName, vmName, compute.InstanceView)
	if vmErr != nil {
		e2e.Logf("Failed to get vm status :: %v", vmErr)
		return "", vmErr
	}
	status1 := *vmStatus.VirtualMachineProperties.InstanceView.Statuses
	status2 := *status1[1].DisplayStatus
	newStatus := strings.Split(status2, " ")
	e2e.Logf("Azure instance status found :: %v", newStatus[1])
	return string(newStatus[1]), nil
}

// GetAzureCredentialFromCluster gets Azure credentials from cluster and loads them as environment variables
func GetAzureCredentialFromCluster(oc *exutil.CLI) (string, error) {
	credential, getSecErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/azure-credentials", "-n", "kube-system", "-o=jsonpath={.data}").Output()
	if getSecErr != nil {
		credential, getSecErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("secret/azure-cloud-credentials", "-n", "openshift-cloud-controller-manager", "-o=jsonpath={.data}").Output()
		if getSecErr != nil {
			return "", nil
		}
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

// GetAzureStorageAccountFromCluster gets azure storage accountName and accountKey from image registry
// TODO: create a storage account and use that accout to manage azure container
func GetAzureStorageAccountFromCluster(oc *exutil.CLI) (string, string, error) {
	var accountName string
	imageRegistry, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-image-registry").Get(context.Background(), "image-registry", metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	for _, container := range imageRegistry.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == "REGISTRY_STORAGE_AZURE_ACCOUNTNAME" {
				accountName = env.Value
				break
			}
		}
	}

	dirname := "/tmp/" + oc.Namespace() + "-creds"
	defer os.RemoveAll(dirname)
	_ = os.MkdirAll(dirname, 0777)
	_, err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("secret/image-registry-private-configuration", "-n", "openshift-image-registry", "--confirm", "--to="+dirname).Output()
	if err != nil {
		return accountName, "", err
	}
	accountKey, _ := os.ReadFile(dirname + "/REGISTRY_STORAGE_AZURE_ACCOUNTKEY")
	return accountName, string(accountKey), nil
}

// NewAzureContainerClient initializes a new azure blob container client
func NewAzureContainerClient(oc *exutil.CLI, accountName, accountKey, azContainerName string) (azblob.ContainerURL, error) {
	storageAccountURISuffix := ".blob.core.windows.net"
	cloudName, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.azure.cloudName}").Output()
	if strings.ToLower(cloudName) == "azureusgovernmentcloud" {
		storageAccountURISuffix = ".blob.core.usgovcloudapi.net"
	}
	//placeholder if strings.ToLower(cloudName) == "azurechinacloud"
	//placeholder if strings.ToLower(cloudName) == "azuregermancloud"
	u, _ := url.Parse(fmt.Sprintf("https://%s%s", accountName, storageAccountURISuffix))
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	serviceURL := azblob.NewServiceURL(*u, p)
	return serviceURL.NewContainerURL(azContainerName), err
}

// CreateAzureStorageBlobContainer creates azure storage container
func CreateAzureStorageBlobContainer(container azblob.ContainerURL) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// check if the container exists or not
	// if exists, then remove the blobs in the container, if not, create the container
	_, err := container.GetProperties(ctx, azblob.LeaseAccessConditions{})
	message := fmt.Sprintf("%v", err)
	if strings.Contains(message, "ContainerNotFound") {
		_, err = container.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
		return err
	}
	return EmptyAzureBlobContainer(container)
}

// DeleteAzureStorageBlobContainer deletes azure storage container
func DeleteAzureStorageBlobContainer(container azblob.ContainerURL) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := EmptyAzureBlobContainer(container)
	if err != nil {
		return err
	}
	_, err = container.Delete(ctx, azblob.ContainerAccessConditions{})
	if err != nil {
		return fmt.Errorf("error deleting container: %v", err)
	}
	e2e.Logf("Azure storage container is deleted")
	return nil
}

// EmptyAzureBlobContainer removes all the files in azure storage container
func EmptyAzureBlobContainer(container azblob.ContainerURL) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for marker := (azblob.Marker{}); marker.NotDone(); { // The parens around Marker{} are required to avoid compiler error.
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := container.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			return fmt.Errorf("error listing blobs in container: %v", err)
		}

		// IMPORTANT: ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment (if the segment is empty, the loop body won't execute)
		for _, blobInfo := range listBlob.Segment.BlobItems {
			blobURL := container.NewBlockBlobURL(blobInfo.Name)
			_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
			if err != nil {
				return fmt.Errorf("error deleting blob %s: %v", blobInfo.Name, err)
			}
		}
	}
	e2e.Logf("deleted all blob items in the container.")
	return nil
}

// getUserAssignedIdentitiesClient get user assigned identities client
func getUserAssignedIdentitiesClient(sess *AzureSession) msi.UserAssignedIdentitiesClient {
	msiClient := msi.NewUserAssignedIdentitiesClient(sess.SubscriptionID)
	msiClient.Authorizer = sess.Authorizer
	return msiClient
}

// GetUserAssignedIdentityPrincipalID get user assigned identity PrincipalID
func GetUserAssignedIdentityPrincipalID(sess *AzureSession, resourceGroup string, identityName string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	msiClient := getUserAssignedIdentitiesClient(sess)
	identity, err := msiClient.Get(ctx, resourceGroup, identityName)
	if err != nil {
		return "", err
	}
	return identity.PrincipalID.String(), nil
}

// getRoleAssignmentsClient get role assignments client
func getRoleAssignmentsClient(sess *AzureSession) authorization.RoleAssignmentsClient {
	roleAssignmentsClient := authorization.NewRoleAssignmentsClient(sess.SubscriptionID)
	roleAssignmentsClient.Authorizer = sess.Authorizer
	return roleAssignmentsClient
}

// GrantRoleToPrincipalIDByResourceGroup grant role to principalID by resource group
func GrantRoleToPrincipalIDByResourceGroup(sess *AzureSession, principalID string, resourceGroup string, roleId string) (roleAssignmentName string, scope string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	roleAssignmentsClient := getRoleAssignmentsClient(sess)

	roleAssignment := authorization.RoleAssignmentCreateParameters{
		Properties: &authorization.RoleAssignmentProperties{
			PrincipalID:      &principalID,
			RoleDefinitionID: to.StringPtr("/subscriptions/" + sess.SubscriptionID + "/providers/Microsoft.Authorization/roleDefinitions/" + roleId),
		},
	}
	roleAssignmentName = sess.SubscriptionID[:8] + principalID[8:24] + roleId[24:]
	scope = "/subscriptions/" + sess.SubscriptionID + "/resourceGroups/" + resourceGroup
	result, err := roleAssignmentsClient.Create(ctx, scope, roleAssignmentName, roleAssignment)
	if err != nil {
		e2e.Logf("Error creating role assignment: %v", err)
	} else {
		e2e.Logf("Role assignment created: %s", *result.Name)
	}
	return roleAssignmentName, scope
}

// DeleteRoleAssignments deletes role assignments
func DeleteRoleAssignments(sess *AzureSession, roleAssignmentName string, scope string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	roleAssignmentsClient := getRoleAssignmentsClient(sess)
	_, err := roleAssignmentsClient.Delete(ctx, scope, roleAssignmentName)
	if err != nil {
		return fmt.Errorf("error deleting role assignment: %v", err)
	}
	e2e.Logf("Role assignment is deleted")
	return nil
}

// StopAzureStackVM stops the virtual machine with the given name in the specified resource group using Azure exutil.CLI
func StopAzureStackVM(resourceGroupName, vmName string) error {
	cmd := fmt.Sprintf(`az vm stop --name %s --resource-group %s --no-wait`, vmName, resourceGroupName)
	err := exec.Command("bash", "-c", cmd).Run()
	if err != nil {
		return fmt.Errorf("error stopping VM: %v", err)
	}
	return nil
}

// StartAzureStackVM starts the virtual machine with the given name in the specified resource group using Azure exutil.CLI
func StartAzureStackVM(resourceGroupName, vmName string) error {
	cmd := fmt.Sprintf(`az vm start --name %s --resource-group %s`, vmName, resourceGroupName)
	output, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return fmt.Errorf("error starting VM: %v, output: %s", err, output)
	}
	return nil
}

// GetAzureStackVMStatus gets the status of the virtual machine with the given name in the specified resource group using Azure exutil.CLI
func GetAzureStackVMStatus(resourceGroupName, vmName string) (string, error) {
	cmd := fmt.Sprintf(`az vm show --name %s --resource-group %s --query 'powerState' --show-details |awk '{print $2}' | cut -d '"' -f1`, vmName, resourceGroupName)
	instanceState, err := exec.Command("bash", "-c", cmd).Output()
	if string(instanceState) == "" || err != nil {
		return "", fmt.Errorf("Not able to get vm instance state :: %s", err)
	}
	return strings.Trim(string(instanceState), "\n"), err
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
