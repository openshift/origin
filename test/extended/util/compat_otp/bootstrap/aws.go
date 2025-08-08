package bootstrap

import (
	"os"

	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	clusterinfra "github.com/openshift/origin/test/extended/util/compat_otp/clusterinfra"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// EnvVarSSHCloudPrivAWSUser stores the environment variable used to configure the AWS ssh user
	EnvVarSSHCloudPrivAWSUser = "SSH_CLOUD_PRIV_AWS_USER"
)

// AWSBSInfoProvider implements interface BSInfoProvider
type AWSBSInfoProvider struct{}

// GetIPs returns the IPs of the boostrap machine if this machine exists in AWS
func (a AWSBSInfoProvider) GetIPs(oc *exutil.CLI) (*Ips, error) {
	// Extract the infrastructure name from the cluster infrastructure resource
	infraName, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.infrastructureName}").Output()
	if err != nil {
		e2e.Logf("Could not get bootstrap's IP in AWS. Unable to get infrastructure's name. Error: %s", err)
		return nil, err
	}

	bootstrapName := infraName + "-bootstrap"

	clusterinfra.GetAwsCredentialFromCluster(oc.AsAdmin())
	aws := compat_otp.InitAwsSession()
	bootstrapInstanceID, err := aws.GetAwsInstanceID(bootstrapName)
	if err != nil {
		// If the instance cannot be found, but no other error happens we return a &InstanceNotFound error, so that
		// it can be used to skip the test case if no bootstrap is present
		if notFoundErr, notFound := err.(*compat_otp.AWSInstanceNotFound); notFound {
			return nil, &InstanceNotFound{notFoundErr.InstanceName}
		}
		e2e.Logf("Could not get bootstrap's IP in AWS. Unable to get bootstrap instance ID from infrastructure name '%s'. Error: %s",
			infraName, err)
		return nil, err

	}

	state, err := aws.GetAwsInstanceState(bootstrapInstanceID)
	if err != nil {
		e2e.Logf("Could not get bootstrap's IP in AWS. Unable to get state for bootstrap instance ID '%s'. Error: %s",
			bootstrapInstanceID, err)
		return nil, err

	}

	if state == "terminated" {
		e2e.Logf("Boostrap instance's state: %s", state)
		// If the found instance is terminated, return a &InstanceNotFound error
		// it can be used to skip the test case if no bootstrap is present
		return nil, &InstanceNotFound{bootstrapInstanceID}
	}

	bootstrapIP, err := aws.GetAwsIntIPs(bootstrapInstanceID)
	if err != nil {
		e2e.Logf("Could not get bootstrap's IP in AWS. Unable to get bootstrap IPs from instance ID '%s'. Error: %s",
			bootstrapInstanceID, err)
		return nil, err
	}

	return &Ips{PublicIPAddress: bootstrapIP["publicIP"], PrivateIPAddress: bootstrapIP["privateIP"]}, nil
}

// GetSSHUser returns the user needed to connect to the bootstrap machine via ssh
func (a AWSBSInfoProvider) GetSSHUser() string {
	user, exists := os.LookupEnv(EnvVarSSHCloudPrivAWSUser)

	if !exists {
		user = DefaultSSHUser
	}

	return user
}
