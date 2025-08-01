package compat_otp

import (
	"context"
	"fmt"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	azTo "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armfeatures"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	o "github.com/onsi/gomega"
)

// AzureClientSet encapsulates Azure account information and multiple clients.
type AzureClientSet struct {
	// Account information
	SubscriptionID  string
	tokenCredential azcore.TokenCredential

	// Clients
	capacityReservationGroupClient *armcompute.CapacityReservationGroupsClient
	capacityReservationsClient     *armcompute.CapacityReservationsClient
	featuresClient                 *armfeatures.Client
	graphServiceClient             *msgraphsdkgo.GraphServiceClient
	keysClient                     *armkeyvault.KeysClient
	resourceGroupsClient           *armresources.ResourceGroupsClient
	vaultsClient                   *armkeyvault.VaultsClient
	virtualMachinesClient          *armcompute.VirtualMachinesClient
}

func NewAzureClientSet(subscriptionId string, tokenCredential azcore.TokenCredential) *AzureClientSet {
	return &AzureClientSet{
		SubscriptionID:  subscriptionId,
		tokenCredential: tokenCredential,
	}
}

// NewAzureClientSetWithRootCreds constructs an AzureClientSet with info gleaned from the in-cluster root credential.
func NewAzureClientSetWithRootCreds(oc *exutil.CLI) *AzureClientSet {
	azCreds := NewEmptyAzureCredentials()
	o.Expect(azCreds.GetFromClusterAndDecode(oc)).NotTo(o.HaveOccurred())
	o.Expect(azCreds.SetSdkEnvVars()).NotTo(o.HaveOccurred())
	azureCredentials, err := azidentity.NewDefaultAzureCredential(nil)
	o.Expect(err).NotTo(o.HaveOccurred())
	return NewAzureClientSet(azCreds.AzureSubscriptionID, azureCredentials)
}

// NewAzureClientSetWithCredsFromFile constructs an AzureClientSet with info gleaned from a file.
func NewAzureClientSetWithCredsFromFile(filePath string) *AzureClientSet {
	azCreds := NewEmptyAzureCredentialsFromFile()
	o.Expect(azCreds.LoadFromFile(filePath)).NotTo(o.HaveOccurred())
	o.Expect(azCreds.SetSdkEnvVars()).NotTo(o.HaveOccurred())
	azureCredentials, err := azidentity.NewDefaultAzureCredential(nil)
	o.Expect(err).NotTo(o.HaveOccurred())
	return NewAzureClientSet(azCreds.AzureSubscriptionID, azureCredentials)
}

// NewAzureClientSetWithCredsFromCanonicalFile creates an AzureClientSet using credentials from
// the canonical file location defined by AZURE_CREDS_LOCATION.
func NewAzureClientSetWithCredsFromCanonicalFile() *AzureClientSet {
	return NewAzureClientSetWithCredsFromFile(MustGetAzureCredsLocation())
}

// GetResourceGroupClient gets the resource group client from the AzureClientSet, constructs it if necessary.
// Concurrent invocation of this method is safe only when AzureClientSet.resourceGroupsClient is non-nil,
// which is the case when the resourceGroupsClient is eagerly initialized.
func (cs *AzureClientSet) GetResourceGroupClient(options *arm.ClientOptions) *armresources.ResourceGroupsClient {
	if cs.resourceGroupsClient == nil {
		rgClient, err := armresources.NewResourceGroupsClient(cs.SubscriptionID, cs.tokenCredential, options)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.resourceGroupsClient = rgClient
	}
	return cs.resourceGroupsClient
}

// GetCapacityReservationGroupClient get capacity reservation group Client
func (cs *AzureClientSet) GetCapacityReservationGroupClient(options *arm.ClientOptions) *armcompute.CapacityReservationGroupsClient {
	if cs.capacityReservationGroupClient == nil {
		capacityReservationGroupClient, err := armcompute.NewCapacityReservationGroupsClient(cs.SubscriptionID, cs.tokenCredential, options)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.capacityReservationGroupClient = capacityReservationGroupClient
	}
	return cs.capacityReservationGroupClient
}

// GetCapacityReservationsClient get capacity reservations client
func (cs *AzureClientSet) GetCapacityReservationsClient(options *arm.ClientOptions) *armcompute.CapacityReservationsClient {
	if cs.capacityReservationsClient == nil {
		capacityReservationsClient, err := armcompute.NewCapacityReservationsClient(cs.SubscriptionID, cs.tokenCredential, options)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.capacityReservationsClient = capacityReservationsClient
	}
	return cs.capacityReservationsClient
}

// GetVaultsClient gets the vaults client from AzureClientSet, constructs it if necessary.
func (cs *AzureClientSet) GetVaultsClient(options *arm.ClientOptions) *armkeyvault.VaultsClient {
	if cs.vaultsClient == nil {
		vaultsClient, err := armkeyvault.NewVaultsClient(cs.SubscriptionID, cs.tokenCredential, options)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.vaultsClient = vaultsClient
	}
	return cs.vaultsClient
}

// GetKeysClient gets the keys client from AzureClientSet, constructs it if necessary.
func (cs *AzureClientSet) GetKeysClient(options *arm.ClientOptions) *armkeyvault.KeysClient {
	if cs.keysClient == nil {
		keysClient, err := armkeyvault.NewKeysClient(cs.SubscriptionID, cs.tokenCredential, options)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.keysClient = keysClient
	}
	return cs.keysClient
}

// GetVirtualMachinesClient gets the virtual machine client from AzureClientSet, constructs it if necessary.
func (cs *AzureClientSet) GetVirtualMachinesClient(options *arm.ClientOptions) *armcompute.VirtualMachinesClient {
	if cs.virtualMachinesClient == nil {
		virtualMachineClient, err := armcompute.NewVirtualMachinesClient(cs.SubscriptionID, cs.tokenCredential, options)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.virtualMachinesClient = virtualMachineClient
	}
	return cs.virtualMachinesClient
}

// GetGraphServiceClient gets the graph service client from AzureClientSet, constructs it if necessary.
// Pass a nil slice to use the default scope.
func (cs *AzureClientSet) GetGraphServiceClient(scopes []string) *msgraphsdkgo.GraphServiceClient {
	if cs.graphServiceClient == nil {
		graphServiceClient, err := msgraphsdkgo.NewGraphServiceClientWithCredentials(cs.tokenCredential, scopes)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.graphServiceClient = graphServiceClient
	}
	return cs.graphServiceClient
}

// CreateCapacityReservationGroup create a capacity reservation group
func (cs *AzureClientSet) CreateCapacityReservationGroup(ctx context.Context, capacityReservationGroupName string, resourceGroupName string, location string, zone string) (string, error) {
	capacityReservationGroupClient := cs.GetCapacityReservationGroupClient(nil)
	capacityReservationGroup := armcompute.CapacityReservationGroup{
		Location: &location,
		Zones:    []*string{&zone},
	}

	resp, err := capacityReservationGroupClient.CreateOrUpdate(
		ctx,
		resourceGroupName,
		capacityReservationGroupName,
		capacityReservationGroup,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("Failed to create Capacity Reservation Group: %v", err)
	}
	e2e.Logf("Capacity Reservation Group created successfully, capacity reservation group ID: %s", *resp.ID)
	return *resp.ID, err
}

// CreateCapacityReservation create a capacity reservation
func (cs *AzureClientSet) CreateCapacityReservation(ctx context.Context, capacityReservationGroupName string, capacityReservationName string, location string, resourceGroupName string, skuName string, zone string) error {
	capacityReservationsClient := cs.GetCapacityReservationsClient(nil)
	instanceCapacity := int64(1)
	capacityReservation := armcompute.CapacityReservation{
		Location: &location,
		SKU: &armcompute.SKU{
			Capacity: &instanceCapacity,
			Name:     &skuName,
		},
		Zones: []*string{&zone},
	}
	resp, err := capacityReservationsClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		capacityReservationGroupName,
		capacityReservationName,
		capacityReservation,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Failed to create Capacity Reservation: %v", err)
	}
	finalResp, err := resp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to wait for the Capacity Reservation creation to complete: %v", err)
	}
	e2e.Logf("Capacity Reservation created successfully %s", *finalResp.ID)
	return nil
}

// DeleteCapacityReservationGroup delete capacity reservation group
func (cs *AzureClientSet) DeleteCapacityReservationGroup(ctx context.Context, capacityReservationGroupName string, resourceGroupName string) error {
	capacityReservationGroupClient := cs.GetCapacityReservationGroupClient(nil)
	_, err := capacityReservationGroupClient.Delete(
		ctx,
		resourceGroupName,
		capacityReservationGroupName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Failed to delete Capacity Reservation Group: %v", err)
	}
	e2e.Logf("Capacity Reservation Group deleted successfully")
	return nil
}

// DeleteCapacityReservation delete capacity reservation
func (cs *AzureClientSet) DeleteCapacityReservation(ctx context.Context, capacityReservationGroupName string, capacityReservationName string, resourceGroupName string) error {
	capacityReservationsClient := cs.GetCapacityReservationsClient(nil)
	resp, err := capacityReservationsClient.BeginDelete(
		ctx,
		resourceGroupName,
		capacityReservationGroupName,
		capacityReservationName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("Failed to delete Capacity Reservation: %v", err)
	}
	_, err = resp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to wait for the capacity reservation deletation to complete: %v", err)
	}
	e2e.Logf("Capacity Reservation deleted successfully")
	return nil
}

func (cs *AzureClientSet) DeleteResourceGroup(ctx context.Context, rgName string) error {
	poller, err := cs.GetResourceGroupClient(nil).BeginDelete(ctx, rgName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (cs *AzureClientSet) GetServicePrincipalObjectId(ctx context.Context, appId string) (string, error) {
	sp, err := cs.GetGraphServiceClient(nil).ServicePrincipalsWithAppId(&appId).Get(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get service principal: %v", err)
	}
	objectId := sp.GetId()
	if objectId == nil {
		return "", fmt.Errorf("object ID is nil")
	}
	return *objectId, nil
}

func ProcessAzurePages[T any](ctx context.Context, pager *runtime.Pager[T], handlePage func(page T) error) error {
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}
		err = handlePage(page)
		if err != nil {
			return fmt.Errorf("error processing page: %w", err)
		}
	}
	return nil
}

// Create an Azure resource group.
func (cs *AzureClientSet) CreateResourceGroup(ctx context.Context, resourceGroupName, region string) (armresources.ResourceGroupsClientCreateOrUpdateResponse, error) {
	rgClient, _ := armresources.NewResourceGroupsClient(cs.SubscriptionID, cs.tokenCredential, nil)
	param := armresources.ResourceGroup{
		Location: azTo.Ptr(region),
	}
	return rgClient.CreateOrUpdate(ctx, resourceGroupName, param, nil)
}

// Creates Azure storage account.
func (cs *AzureClientSet) CreateStorageAccount(ctx context.Context, resourceGroupName, storageAccountName, region string) (armstorage.AccountsClientListKeysResponse, error) {
	storageClient, _ := armstorage.NewAccountsClient(cs.SubscriptionID, cs.tokenCredential, nil)
	result, _ := storageClient.BeginCreate(ctx, resourceGroupName, storageAccountName, armstorage.AccountCreateParameters{
		Location: azTo.Ptr(region),
		SKU: &armstorage.SKU{
			Name: azTo.Ptr(armstorage.SKUNameStandardLRS),
		},
		Kind: azTo.Ptr(armstorage.KindStorageV2),
	}, nil)

	// Poll until the Storage account is ready
	_, err := result.PollUntilDone(context.Background(), &runtime.PollUntilDoneOptions{
		Frequency: 10 * time.Second,
	})
	AssertWaitPollNoErr(err, "Storage account is not ready...")

	resultKey, err := storageClient.ListKeys(ctx, resourceGroupName, storageAccountName, &armstorage.AccountsClientListKeysOptions{Expand: nil})
	return resultKey, err
}

// Delete the created storage account.
func (cs *AzureClientSet) DeleteStorageAccount(ctx context.Context, resourceGroupName, storageAccountName string) {
	clientFactory, err := armstorage.NewClientFactory(cs.SubscriptionID, cs.tokenCredential, nil)
	if err != nil {
		e2e.Failf("failed to create client: %v", err)
	}
	_, err = clientFactory.NewAccountsClient().Delete(ctx, resourceGroupName, storageAccountName, nil)
	if err != nil {
		e2e.Failf("failed to finish the request: %v", err)
	}
}

func (cs *AzureClientSet) GetStorageAccountProperties(storageAccountName string, resourceGroupName string) armstorage.AccountsClientGetPropertiesResponse {
	ctx := context.Background()
	clientFactory, err := armstorage.NewClientFactory(cs.SubscriptionID, cs.tokenCredential, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	res, err := clientFactory.NewAccountsClient().GetProperties(ctx, resourceGroupName, storageAccountName, &armstorage.AccountsClientGetPropertiesOptions{Expand: nil})
	o.Expect(err).NotTo(o.HaveOccurred())

	return res
}

// GetFeaturesClient get feature Client
func (cs *AzureClientSet) GetFeaturesClient(options *arm.ClientOptions) *armfeatures.Client {
	if cs.featuresClient == nil {
		featuresClient, err := armfeatures.NewClient(cs.SubscriptionID, cs.tokenCredential, options)
		o.Expect(err).NotTo(o.HaveOccurred())
		cs.featuresClient = featuresClient
	}
	return cs.featuresClient
}

// RegisterEncryptionAtHost enable EncryptionAtHost in azure
func (cs *AzureClientSet) RegisterEncryptionAtHost(ctx context.Context) error {
	featuresClient := cs.GetFeaturesClient(nil)
	featureName := "EncryptionAtHost"
	resourceProvider := "Microsoft.Compute"

	// Check if already registered EncryptionAtHost
	feature, err := featuresClient.Get(ctx, resourceProvider, featureName, nil)
	if err == nil && *feature.Properties.State == "Registered" {
		return nil
	}

	// Register and wait for 5 mins to finish registration
	_, err = featuresClient.Register(ctx, resourceProvider, featureName, nil)
	if err != nil {
		return fmt.Errorf("EncryptionAtHost registered failed: %v", err)
	}
	const maxRetries = 10
	for i := 0; i < maxRetries; i++ {
		feature, err := featuresClient.Get(ctx, resourceProvider, featureName, nil)
		if err != nil {
			return fmt.Errorf("Failed to get feature: %v", err)
		}

		if *feature.Properties.State == "Registered" {
			return nil
		}
		time.Sleep(30 * time.Second)
	}
	return fmt.Errorf("Register timeout")
}
