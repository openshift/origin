package bootstrap

import (
	"fmt"
	"os"

	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// EnvVarSSHCloudPrivAzureUser stores the environment variable used to configure the Azure ssh user
	EnvVarSSHCloudPrivAzureUser = "SSH_CLOUD_PRIV_AZURE_USER"
)

// AzureBSInfoProvider implements interface BSInfoProvider
type AzureBSInfoProvider struct{}

// GetIPs returns the IPs of the boostrap machine if this machine exists in Azure
func (a AzureBSInfoProvider) GetIPs(oc *exutil.CLI) (*Ips, error) {
	// Extract the infrastructure name from the cluster infrastructure resource
	infraName, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
	if err != nil {
		e2e.Logf("Could not get bootstrap's IP in Azure. Unable to get infrastructure's name. Error: %s", err)
		return nil, err
	}

	bootstrapName := infraName + "-bootstrap"

	resourceGroupName, err := compat_otp.GetAzureCredentialFromCluster(oc.AsAdmin())
	if err != nil {
		e2e.Logf("Error reading cluster's azure credentials")
		return nil, err
	}

	azSession, err := compat_otp.NewAzureSessionFromEnv()
	if err != nil {
		e2e.Logf("Error creating a new azure session")
		return nil, err
	}

	bootstrapInstanceID, err := compat_otp.GetAzureVMInstance(azSession, bootstrapName, resourceGroupName)
	if err != nil {
		e2e.Logf("Could not get bootstrap's IP in Azure. Unable to get bootstrap instance ID from infrastructure name '%s'. Error: %s",
			infraName, err)
		return nil, err

	}
	e2e.Logf("Instance vm %s", bootstrapInstanceID)
	// If the instance cannot be found, but no other error happens we return a &InstanceNotFound error, so that
	// it can be used to skip the test case if no bootstrap is present
	if bootstrapInstanceID == "" {
		return nil, &InstanceNotFound{bootstrapName}
	}

	state, err := compat_otp.GetAzureVMInstanceState(azSession, bootstrapInstanceID, resourceGroupName)
	if err != nil {
		e2e.Logf("Could not get bootstrap's IP in Azure. Unable to get state for bootstrap instance ID '%s' in resource group '%s'. Error: %s",
			bootstrapInstanceID, resourceGroupName, err)
		return nil, err

	}

	if state != "running" {
		e2e.Logf("Boostrap instance's state: %s", state)
		// If the found instance is not running, return a &InstanceNotFound error
		// it can be used to skip the test case if no bootstrap is present
		return nil, &InstanceNotFound{bootstrapInstanceID}
	}

	// In ipi deployments the name of the public IP is xxxx-bootstrap-pip-v4 and in upi deployments it is xxx-bootstrap-ssh-pip
	// so we need to use a regex search in order to get the IP no matter if upi or ipi
	bootstrapPublicIP, err := compat_otp.GetAzureVMPublicIPByNameRegex(azSession, resourceGroupName, bootstrapInstanceID)
	if err != nil {
		e2e.Logf("Could not get bootstrap's public IP in Azure. Unable to get bootstrap public IP from instance ID '%s' in resource group '%s'. Error: %s",
			bootstrapInstanceID, resourceGroupName, err)
		return nil, err
	}

	if bootstrapPublicIP == "" {
		return nil, fmt.Errorf("No public IP is assigned for the boostrap machine %s in resource group %s", bootstrapInstanceID, resourceGroupName)
	}

	bootstrapPrivateIP, err := compat_otp.GetAzureVMPrivateIP(azSession, resourceGroupName, bootstrapInstanceID)
	if err != nil {
		e2e.Logf("Could not get bootstrap's private IP in Azure. Unable to get bootstrap private IP from instance ID '%s' in resource group '%s'. Error: %s",
			bootstrapInstanceID, resourceGroupName, err)
		return nil, err
	}

	return &Ips{PublicIPAddress: bootstrapPublicIP, PrivateIPAddress: bootstrapPrivateIP}, nil
}

// GetSSHUser returns the user needed to connect to the bootstrap machine via ssh
func (a AzureBSInfoProvider) GetSSHUser() string {
	user, exists := os.LookupEnv(EnvVarSSHCloudPrivAzureUser)

	if !exists {
		user = DefaultSSHUser
	}

	return user
}
