package bootstrap

import (
	"fmt"
	"os"

	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// EnvVarSSHCloudPrivKey storest the environment variable used to configure the location of the ssh private key to connect to bootstrap machine
	EnvVarSSHCloudPrivKey = "SSH_CLOUD_PRIV_KEY"
	// DefaultSSHUser default user in case of not being configured via envvar
	DefaultSSHUser = "core"
)

// InstanceNotFound reports an error because the bootstrap instance is not found. It can be used to skip the test case
type InstanceNotFound struct{ InstanceName string }

// Error implements the error interface
func (inferr *InstanceNotFound) Error() string {
	return fmt.Sprintf("Instance %s has 'terminated' status", inferr.InstanceName)
}

// BSInfoProvider any struct implementing this interface can be used to create a Boostrap object.
// Currently it is only implemented by AWSBSInfoProvider
type BSInfoProvider interface {
	GetIPs(*exutil.CLI) (*Ips, error)
	GetSSHUser() string
}

// Bootstrap contains the functionality regarding the bootstrap machine
type Bootstrap struct {
	SSH compat_otp.SshClient
	IPs Ips
}

// Ips struct to store the public and the private IPs of the bootstrap machine
type Ips struct {
	PrivateIPAddress string
	PublicIPAddress  string
}

// GetBootstrap returns a bootstrap struct pointing to the bootstrap machine if exists
func GetBootstrap(oc *exutil.CLI) (*Bootstrap, error) {
	bsInfoProvider, err := GetBSInfoProvider(oc)
	if err != nil {
		return nil, err
	}

	bootstrapIPs, err := bsInfoProvider.GetIPs(oc.AsAdmin())
	if err != nil {
		return nil, err
	}

	user := bsInfoProvider.GetSSHUser()

	return buildBootstrap(user, *bootstrapIPs, 22), nil
}

// GetBSInfoProvider returns a struct implementing BSInfoProvider for the current platform
func GetBSInfoProvider(oc *exutil.CLI) (BSInfoProvider, error) {
	platform := compat_otp.CheckPlatform(oc)
	switch platform {
	case "aws":
		return AWSBSInfoProvider{}, nil
	case "azure":
		return AzureBSInfoProvider{}, nil
	default:
		return nil, fmt.Errorf("Platform not already supported. Cannot get bootstrap information for platform: %s", platform)
	}

}

// GetBootstrapPrivateKey returns the location of the private key needed to login to the bootstrap machine
func GetBootstrapPrivateKey() string {
	return os.Getenv(EnvVarSSHCloudPrivKey)
}

func buildBootstrap(user string, bootstrapIPs Ips, port int) *Bootstrap {
	privateKey := GetBootstrapPrivateKey()
	publicIP := bootstrapIPs.PublicIPAddress
	e2e.Logf("Creating bootstrap with ip '%s', user: '%s', private key: '%s', port '%d'",
		publicIP, user, privateKey, port)
	return &Bootstrap{SSH: compat_otp.SshClient{User: user, Host: publicIP, Port: port, PrivateKey: privateKey},
		IPs: bootstrapIPs}
}
