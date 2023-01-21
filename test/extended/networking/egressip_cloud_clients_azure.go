package networking

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	azureVMPublisher       = "RedHat"
	azureVMOffer           = "RHEL"
	azureVMSku             = "8-LVM"
	azureVMVersion         = "latest"
	azureVMSize            = "Standard_A2_v2"
	azureVMUser            = "cloud-user"
	azureVMAdminPassword   = "Ara$#hdei4"
	azureVMDisablePassword = false
	azureVMPublicKeyPath   = "/home/cloud-user/.ssh/authorized_keys"
)

var (
	defaultAzureOperationTimeout = 10 * time.Second
)

type azureCloudClient struct {
	oc                             *exutil.CLI
	vmClient                       compute.VirtualMachinesClient
	virtualNetworkClient           network.VirtualNetworksClient
	networkClient                  network.InterfacesClient
	ipClient                       network.PublicIPAddressesClient
	securityGroupsClient           network.SecurityGroupsClient
	interfacesClient               network.InterfacesClient
	virtualMachineExtensionsClient compute.VirtualMachineExtensionsClient
	env                            azure.Environment
	resourceGroup                  string
	ctx                            context.Context
	location                       *string
	subnet                         *network.Subnet
}

func newAzureCloudClient(oc *exutil.CLI) (*azureCloudClient, error) {
	client := &azureCloudClient{oc: oc, ctx: context.TODO()}
	err := client.initCloudSecret()
	if err != nil {
		return nil, err
	}
	// Extract location.
	instance, err := client.getInstance()
	if err != nil {
		return nil, err
	}
	client.location = instance.Location

	// Extract subnet.
	principalInterface, err := client.getPrincipalInterface(instance)
	if err != nil {
		return nil, err
	}
	for _, ipConfiguration := range *principalInterface.IPConfigurations {
		client.subnet = ipConfiguration.Subnet
		break
	}

	return client, nil
}

func (a *azureCloudClient) initCloudSecret() error {
	data, err := readCloudSecret(a.oc, secretNamespace, secretName)
	if err != nil {
		return err
	}

	fields := []string{"azure_client_id", "azure_client_secret", "azure_tenant_id", "azure_subscription_id",
		"azure_resourcegroup"}
	for _, f := range fields {
		if _, ok := data[f]; !ok {
			return fmt.Errorf("could not read %q from cloud secret", f)
		}
	}
	clientID := string(data["azure_client_id"])
	clientSecret := string(data["azure_client_secret"])
	tenantID := string(data["azure_tenant_id"])
	subscriptionID := string(data["azure_subscription_id"])
	a.resourceGroup = string(data["azure_resourcegroup"])

	// Pick the Azure "Environment", which is just a named set of API endpoints.
	name := "AzurePublicCloud"
	a.env, err = azure.EnvironmentFromName(name)
	if err != nil {
		return fmt.Errorf("failed to initialize Azure environment: %w", err)
	}

	authorizer, err := a.getAuthorizer(a.env, clientID, clientSecret, tenantID)
	if err != nil {
		return err
	}

	a.vmClient = compute.NewVirtualMachinesClientWithBaseURI(a.env.ResourceManagerEndpoint, subscriptionID)
	a.vmClient.Authorizer = authorizer
	err = a.vmClient.AddToUserAgent(userAgent)
	if err != nil {
		return err
	}

	a.networkClient = network.NewInterfacesClientWithBaseURI(a.env.ResourceManagerEndpoint, subscriptionID)
	a.networkClient.Authorizer = authorizer
	err = a.networkClient.AddToUserAgent(userAgent)
	if err != nil {
		return err
	}

	a.virtualNetworkClient = network.NewVirtualNetworksClientWithBaseURI(a.env.ResourceManagerEndpoint, subscriptionID)
	a.virtualNetworkClient.Authorizer = authorizer
	err = a.virtualNetworkClient.AddToUserAgent(userAgent)
	if err != nil {
		return err
	}

	a.ipClient = network.NewPublicIPAddressesClient(subscriptionID)
	a.ipClient.Authorizer = authorizer
	err = a.ipClient.AddToUserAgent(userAgent)
	if err != nil {
		return err
	}

	a.securityGroupsClient = network.NewSecurityGroupsClient(subscriptionID)
	a.securityGroupsClient.Authorizer = authorizer
	err = a.securityGroupsClient.AddToUserAgent(userAgent)
	if err != nil {
		return err
	}

	a.virtualMachineExtensionsClient = compute.NewVirtualMachineExtensionsClient(subscriptionID)
	a.virtualMachineExtensionsClient.Authorizer = authorizer
	err = a.virtualMachineExtensionsClient.AddToUserAgent(userAgent)
	if err != nil {
		return err
	}

	a.interfacesClient = network.NewInterfacesClient(subscriptionID)
	a.interfacesClient.Authorizer = authorizer
	err = a.interfacesClient.AddToUserAgent(userAgent)
	return err
}

func azureSecurityGroupName(vmName string) string {
	return fmt.Sprintf("%s-security-group", vmName)
}

func azurePublicIPName(vmName string) string {
	return fmt.Sprintf("%s-public-ip", vmName)
}

func azureInterfaceName(vmName string) string {
	return fmt.Sprintf("%s-interface", vmName)
}

func azureExtensionName(vmName string) string {
	return fmt.Sprintf("%s-extension", vmName)
}

// createVM creates a VirtualMachine in Azure and sets the VM's privateIP and publicIP field after creation.
// In Azure, we can create a custom security group for the port. However, that takes a bit longer and is more error
// prone. For port 6443, we do not need any of this because the default security group already allows 6443 to the
// subnet.
func (a *azureCloudClient) createVM(vm *vm, requestPublicIP bool) error {
	// Create a securityGroup if needed.
	var err error
	var securityGroup *network.SecurityGroup
	if len(vm.ports) > 0 {
		p, ok := vm.ports[applicationPort]
		if len(vm.ports) > 2 || (ok && !(p.protocol == tcp && p.port == 6443)) {
			securityGroupName := azureSecurityGroupName(vm.name)
			framework.Logf("Creating security group %s for VM %s", securityGroupName, vm.name)
			securityGroup, err = a.createSecurityGroup(securityGroupName, vm.ports)
			if err != nil {
				return err
			}
		}
	}
	// Create a public IP address for IP address assignment if requested.
	var createdPublicIP *network.PublicIPAddress
	if requestPublicIP {
		publicIPName := azurePublicIPName(vm.name)
		framework.Logf("Creating public IP %s for VM %s", publicIPName, vm.name)
		createdPublicIP, err = a.createPublicIP(publicIPName)
		if err != nil {
			return err
		}
	}
	// Create a network interface from the provided principal interface.
	interfaceName := azureInterfaceName(vm.name)
	framework.Logf("Creating interface %s for VM %s", interfaceName, vm.name)
	createdInterface, err := a.createNetworkInterface(createdPublicIP, securityGroup, interfaceName)
	if err != nil {
		return err
	}
	// Create a Virtual Machine.
	framework.Logf("Creating VirtualMachine %s", vm.name)
	err = a.createVirtualMachine(vm.name, vm.sshPublicKey, createdInterface)
	if err != nil {
		return err
	}
	// Create VirtualMachineExtension of type CustomScript to install podman and to run agnhost on VM.
	virtualMachineExtensionName := azureExtensionName(vm.name)
	framework.Logf("Creating VirtualMachine CustomScript extension %s for VM %s", virtualMachineExtensionName, vm.name)
	script := printp(vm.startupScript, vm.startupScriptParameters)
	err = a.createVirtualMachineExtension(vm.name, virtualMachineExtensionName, script)
	if err != nil {
		return err
	}
	// Read private IP information.
	var retrievedPrivateIP *string
	retrievedNetworkInterface, err := a.interfacesClient.Get(a.ctx, a.resourceGroup, interfaceName, "")
	if err != nil {
		return err
	}
	for _, ipConfiguration := range *retrievedNetworkInterface.IPConfigurations {
		retrievedPrivateIP = ipConfiguration.PrivateIPAddress
		break
	}
	if retrievedPrivateIP == nil {
		return fmt.Errorf("invalid private IP address")
	}
	vm.privateIP = net.ParseIP(*retrievedPrivateIP)
	// Read public IP information and wait for the public IP to be assigned if a public IP was requested.
	if requestPublicIP {
		framework.Logf("Retrieving updated public IP information for VM %s", vm.name)
		if createdPublicIP.Name == nil {
			return fmt.Errorf("create public IP name is nil")
		}
		retrievedPublicIPAddress, err := a.ipClient.Get(a.ctx, a.resourceGroup, *createdPublicIP.Name, "")
		if err != nil {
			return err
		}
		if retrievedPublicIPAddress.IPAddress == nil {
			return fmt.Errorf("invalid public IP address")
		}
		// Set VM publicIP and privateIP.
		vm.publicIP = net.ParseIP(*retrievedPublicIPAddress.IPAddress)
	}
	return nil
}

// deleteVM deletes the provided VM instance from Azure and tears down security groups, public IPs and ports.
func (a *azureCloudClient) deleteVM(vm *vm) error {
	// Delete a Virtual Machine.
	framework.Logf("Deleting VirtualMachine %s", vm.name)
	err := a.deleteVirtualMachine(vm.name)
	if azureParseDeleteError(err) != nil {
		return err
	}
	// Delete a network interface from the provided principal interface.
	interfaceName := azureInterfaceName(vm.name)
	framework.Logf("Deleting interface %s for VM %s", interfaceName, vm.name)
	err = a.deleteNetworkInterface(interfaceName)
	if azureParseDeleteError(err) != nil {
		return err
	}
	// Delete a public IP address for IP address assignment.
	publicIPName := azurePublicIPName(vm.name)
	framework.Logf("Deleting public IP %s for VM %s", publicIPName, vm.name)
	err = a.deletePublicIP(publicIPName)
	if azureParseDeleteError(err) != nil {
		return err
	}
	// Delete a custom security group.
	securityGroupName := azureSecurityGroupName(vm.name)
	framework.Logf("Deleting security group %s for VM %s", securityGroupName, vm.name)
	err = a.deleteSecurityGroup(securityGroupName)
	if azureParseDeleteError(err) != nil {
		return err
	}
	return nil
}

// Close implements the cloudClient interface method of the same name and implementes the Closer interface. In Azure,
// this is a noop.
func (a *azureCloudClient) Close() error {
	return nil
}

// azureParseDeleteError takes an Azure error message. If the error message is of type DetailedError and has a
// StatusCode of 404, we return nil. The goal is to ignore NotFound errors. Otherwise, we return the same error that
// we got as input.
func azureParseDeleteError(err error) error {
	if detErr, ok := err.(autorest.DetailedError); ok {
		if detErr.StatusCode == 404 {
			framework.Logf("Element not found, err: %q", err)
			return nil
		}
	}
	return err
}

// getInstance gets a single worker node instance. We want to create a new virtual machine at the same location and
// subnet as the first worker node that we can find.
func (a *azureCloudClient) getInstance() (*compute.VirtualMachine, error) {
	providerID, err := getWorkerProviderID(a.oc.AsAdmin())
	if err != nil {
		return nil, err
	}
	providerData := strings.Split(providerID, "/")
	if len(providerData) != 11 {
		return nil, fmt.Errorf("unexpected provider ID %s", providerID)
	}
	ctx, cancel := context.WithTimeout(a.ctx, defaultAzureOperationTimeout)
	defer cancel()
	instance, err := a.vmClient.Get(ctx, a.resourceGroup, providerData[len(providerData)-1], "")
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// createVirtualMachineExtension creates a VirtualMachineExtension of type CustomScript. This lets us run a bash
// script directly on the virtual machine to set up an agnhost container for /clientip tests (or other things).
func (a *azureCloudClient) createVirtualMachineExtension(name, virtualMachineExtensionName, script string) error {
	scriptEncoded := base64.StdEncoding.EncodeToString([]byte(script))
	type protectedSettings struct {
		Script string `json:"script"`
	}
	extensionParameters := compute.VirtualMachineExtension{
		Response: autorest.Response{},
		VirtualMachineExtensionProperties: &compute.VirtualMachineExtensionProperties{
			Publisher:          to.StringPtr("Microsoft.Azure.Extensions"),
			Type:               to.StringPtr("CustomScript"),
			TypeHandlerVersion: to.StringPtr("2.1"),
			// Settings:           protectedSettings{Script: scriptEncoded},
			ProtectedSettings: protectedSettings{Script: scriptEncoded},
			ProvisioningState: new(string),
			InstanceView: &compute.VirtualMachineExtensionInstanceView{
				Name:               new(string),
				Type:               new(string),
				TypeHandlerVersion: new(string),
				Substatuses:        &[]compute.InstanceViewStatus{},
				Statuses:           &[]compute.InstanceViewStatus{},
			},
		},
		Location: a.location,
	}
	res, err := a.virtualMachineExtensionsClient.CreateOrUpdate(a.ctx, a.resourceGroup, name, virtualMachineExtensionName,
		extensionParameters)
	if err != nil {
		return err
	}
	return res.WaitForCompletionRef(a.ctx, a.virtualMachineExtensionsClient.Client)
}

// createVirtualMachine creates the actual virtual machine.
func (a *azureCloudClient) createVirtualMachine(name, sshPublicKey string, createdInterface *network.Interface) error {
	parameters := compute.VirtualMachine{
		Location: a.location,
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			StorageProfile: &compute.StorageProfile{
				ImageReference: &compute.ImageReference{
					Publisher: to.StringPtr(azureVMPublisher),
					Offer:     to.StringPtr(azureVMOffer),
					Sku:       to.StringPtr(azureVMSku),
					Version:   to.StringPtr(azureVMVersion),
				},
			},
			HardwareProfile: &compute.HardwareProfile{
				// https://learn.microsoft.com/en-us/azure/virtual-machines/av2-series
				VMSize: azureVMSize,
			},
			OsProfile: &compute.OSProfile{
				ComputerName:  to.StringPtr(name),
				AdminUsername: to.StringPtr(azureVMUser),
				AdminPassword: to.StringPtr(azureVMAdminPassword),
				LinuxConfiguration: &compute.LinuxConfiguration{
					DisablePasswordAuthentication: to.BoolPtr(azureVMDisablePassword),
					SSH: &compute.SSHConfiguration{
						PublicKeys: &[]compute.SSHPublicKey{
							{
								Path:    to.StringPtr(azureVMPublicKeyPath),
								KeyData: to.StringPtr(sshPublicKey),
							},
						},
					},
				},
			},
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: createdInterface.ID,
						NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
							Primary: to.BoolPtr(true),
						},
					},
				},
			},
		},
	}
	createOrUpdateFuture, err := a.vmClient.CreateOrUpdate(a.ctx, a.resourceGroup, name, parameters)
	if err != nil {
		return err
	}
	return createOrUpdateFuture.WaitForCompletionRef(a.ctx, a.ipClient.Client)
}

func (a *azureCloudClient) deleteVirtualMachine(name string) error {
	res, err := a.vmClient.Delete(a.ctx, a.resourceGroup, name)
	if err != nil {
		return err
	}
	return res.WaitForCompletionRef(a.ctx, a.vmClient.Client)
}

func (a *azureCloudClient) createNetworkInterface(createdPublicIP *network.PublicIPAddress,
	securityGroup *network.SecurityGroup, interfaceName string) (*network.Interface, error) {
	var publicIPAddress *network.PublicIPAddress
	if createdPublicIP != nil {
		publicIPAddress = &network.PublicIPAddress{
			ID: createdPublicIP.ID,
		}
	}
	var newIPConfigurations []network.InterfaceIPConfiguration
	// Set up IPConfigurations.
	newIPConfigurations = append(newIPConfigurations, network.InterfaceIPConfiguration{
		Response: autorest.Response{},
		InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
			PrivateIPAllocationMethod: network.Dynamic,
			Subnet:                    a.subnet,
			PublicIPAddress:           publicIPAddress,
		},
		Name: &interfaceName,
	})
	// Set up interface parameters and link to securityGroup if one was provided.
	interfaceParameters := network.Interface{
		Response: autorest.Response{},
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			IPConfigurations:     &newIPConfigurations,
			NetworkSecurityGroup: securityGroup,
		},
		Location: a.location,
	}
	createOrUpdateInterface, err := a.interfacesClient.CreateOrUpdate(a.ctx, a.resourceGroup, interfaceName,
		interfaceParameters)
	if err != nil {
		return nil, err
	}
	err = createOrUpdateInterface.WaitForCompletionRef(a.ctx, a.interfacesClient.Client)
	if err != nil {
		return nil, err
	}
	createdInterface, err := createOrUpdateInterface.Result(a.interfacesClient)
	if err != nil {
		return nil, err
	}
	return &createdInterface, nil
}

func (a *azureCloudClient) deleteNetworkInterface(interfaceName string) error {
	res, err := a.interfacesClient.Delete(a.ctx, a.resourceGroup, interfaceName)
	if err != nil {
		return err
	}
	return res.WaitForCompletionRef(a.ctx, a.interfacesClient.Client)
}

func (a *azureCloudClient) createPublicIP(publicIPName string) (*network.PublicIPAddress, error) {
	publicIPResult, err := a.ipClient.CreateOrUpdate(
		a.ctx,
		a.resourceGroup,
		publicIPName,
		network.PublicIPAddress{
			Sku: &network.PublicIPAddressSku{
				Name: network.PublicIPAddressSkuNameBasic,
			},
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: network.Dynamic,
			},
			Location: a.location,
		},
	)
	if err != nil {
		return nil, err
	}
	err = publicIPResult.WaitForCompletionRef(a.ctx, a.ipClient.Client)
	if err != nil {
		return nil, err
	}
	createdPublicIP, err := publicIPResult.Result(a.ipClient)
	if err != nil {
		return nil, err
	}
	return &createdPublicIP, nil
}

func (a *azureCloudClient) deletePublicIP(publicIPName string) error {
	res, err := a.ipClient.Delete(a.ctx, a.resourceGroup, publicIPName)
	if err != nil {
		return err
	}
	return res.WaitForCompletionRef(a.ctx, a.ipClient.Client)
}

func (a *azureCloudClient) createSecurityGroup(securityGroupName string, listenPorts map[string]protocolPort) (*network.SecurityGroup, error) {
	var securityRules []network.SecurityRule
	var priority int32 = 100
	for name, pp := range listenPorts {
		priority++
		securityRules = append(securityRules, network.SecurityRule{
			SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
				Description:              to.StringPtr(name),
				Protocol:                 network.SecurityRuleProtocol(strings.Title(strings.ToLower(pp.protocol))),
				DestinationPortRange:     to.StringPtr(strconv.Itoa(pp.port)),
				DestinationAddressPrefix: to.StringPtr("*"),
				SourcePortRange:          to.StringPtr("*"),
				SourceAddressPrefix:      to.StringPtr("*"),
				Access:                   network.SecurityRuleAccessAllow,
				Priority:                 to.Int32Ptr(priority),
				Direction:                network.SecurityRuleDirectionInbound,
			},
			Name: to.StringPtr(name),
		})
	}
	securityGroupResult, err := a.securityGroupsClient.CreateOrUpdate(
		a.ctx,
		a.resourceGroup,
		securityGroupName,
		network.SecurityGroup{
			Location: a.location,
			SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
				SecurityRules: &securityRules,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	err = securityGroupResult.WaitForCompletionRef(a.ctx, a.securityGroupsClient.Client)
	if err != nil {
		return nil, err
	}
	securityGroup, err := securityGroupResult.Result(a.securityGroupsClient)
	if err != nil {
		return nil, err
	}
	return &securityGroup, nil
}

func (a *azureCloudClient) deleteSecurityGroup(name string) error {
	res, err := a.securityGroupsClient.Delete(a.ctx, a.resourceGroup, name)
	if err != nil {
		return err
	}
	return res.WaitForCompletionRef(a.ctx, a.securityGroupsClient.Client)
}

func (a *azureCloudClient) getPrincipalInterface(instance *compute.VirtualMachine) (*network.Interface, error) {
	var principalInterface network.Interface
	for _, netif := range *instance.NetworkProfile.NetworkInterfaces {
		if netif.NetworkInterfaceReferenceProperties != nil && netif.Primary != nil && *netif.Primary {
			intf, err := a.networkClient.Get(a.ctx, a.resourceGroup, azureGetNameFromResourceID(*netif.ID), "")
			if err != nil {
				return nil, err
			}
			principalInterface = intf
			break
		}
	}
	return &principalInterface, nil
}

// azureGetNameFromResourceID gets the resource name from the resourceID string (returns the last index).
func azureGetNameFromResourceID(id string) string {
	return id[strings.LastIndex(id, "/"):]
}

func (a *azureCloudClient) getAuthorizer(env azure.Environment, clientID, clientSecret,
	tenantID string) (autorest.Authorizer, error) {
	c := &auth.ClientCredentialsConfig{
		TenantID:     tenantID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AADEndpoint:  env.ActiveDirectoryEndpoint,
	}
	c.Resource = env.TokenAudience
	authorizer, err := c.Authorizer()
	if err != nil {
		return nil, err
	}
	return authorizer, nil
}
