package fake

import (
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation/whitelist"
)

type RegistryWhitelister struct{}

func (rw *RegistryWhitelister) AdmitHostname(host string, transport whitelist.WhitelistTransport) error {
	return nil
}
func (rw *RegistryWhitelister) AdmitPullSpec(pullSpec string, transport whitelist.WhitelistTransport) error {
	return nil
}
func (rw *RegistryWhitelister) AdmitDockerImageReference(ref *imageapi.DockerImageReference, transport whitelist.WhitelistTransport) error {
	return nil
}
func (rw *RegistryWhitelister) WhitelistRegistry(hostPortGlob string, transport whitelist.WhitelistTransport) error {
	return nil
}
func (rw *RegistryWhitelister) WhitelistPullSpecs(pullSpec ...string) {}
func (rw *RegistryWhitelister) Copy() whitelist.RegistryWhitelister {
	return &RegistryWhitelister{}
}
