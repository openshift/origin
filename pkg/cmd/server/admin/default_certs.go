package admin

import (
	"fmt"
	"path"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

const (
	DefaultCADir     = "ca"
	DefaultMasterDir = "master"
)

type ClientCertInfo struct {
	CertLocation configapi.CertInfo
	SubDir       string
	User         string
	Groups       util.StringSet
}

func DefaultSignerName() string {
	return fmt.Sprintf("%s@%d", "openshift-signer", time.Now().Unix())
}

func DefaultRootCAFile(certDir string) string {
	return DefaultCertFilename(certDir, DefaultCADir)
}

func DefaultKubeletClientCAFile(certDir string) string {
	return DefaultRootCAFile(certDir)
}

func DefaultKubeletClientCerts(certDir string) []ClientCertInfo {
	return []ClientCertInfo{
		DefaultMasterKubeletClientCertInfo(certDir),
	}
}

func DefaultMasterKubeletClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: path.Join(certDir, DefaultMasterDir, "kubelet-client.crt"),
			KeyFile:  path.Join(certDir, DefaultMasterDir, "kubelet-client.key"),
		},
		User: "system:master",
	}
}

func DefaultEtcdClientCAFile(certDir string) string {
	return DefaultRootCAFile(certDir)
}

func DefaultEtcdClientCerts(certDir string) []ClientCertInfo {
	return []ClientCertInfo{
		DefaultMasterEtcdClientCertInfo(certDir),
	}
}

func DefaultMasterEtcdClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: path.Join(certDir, DefaultMasterDir, "etcd-client.crt"),
			KeyFile:  path.Join(certDir, DefaultMasterDir, "etcd-client.key"),
		},
		User: "system:master",
	}
}

func DefaultAPIClientCAFile(certDir string) string {
	return DefaultRootCAFile(certDir)
}

func DefaultAPIClientCerts(certDir string) []ClientCertInfo {
	return []ClientCertInfo{
		DefaultDeployerClientCertInfo(certDir),
		DefaultOpenshiftLoopbackClientCertInfo(certDir),
		DefaultKubeClientClientCertInfo(certDir),
		DefaultClusterAdminClientCertInfo(certDir),
		DefaultRouterClientCertInfo(certDir),
		DefaultRegistryClientCertInfo(certDir),
	}
}

func DefaultRouterClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, bootstrappolicy.RouterUnqualifiedUsername),
			KeyFile:  DefaultKeyFilename(certDir, bootstrappolicy.RouterUnqualifiedUsername),
		},
		SubDir: bootstrappolicy.RouterUnqualifiedUsername,
		User:   bootstrappolicy.RouterUsername,
		Groups: util.NewStringSet(bootstrappolicy.RouterGroup),
	}
}

func DefaultRegistryClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, bootstrappolicy.RegistryUnqualifiedUsername),
			KeyFile:  DefaultKeyFilename(certDir, bootstrappolicy.RegistryUnqualifiedUsername),
		},
		SubDir: bootstrappolicy.RegistryUnqualifiedUsername,
		User:   bootstrappolicy.RegistryUsername,
		Groups: util.NewStringSet(bootstrappolicy.RegistryGroup),
	}
}

func DefaultDeployerClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, "openshift-deployer"),
			KeyFile:  DefaultKeyFilename(certDir, "openshift-deployer"),
		},
		SubDir: "openshift-deployer",
		User:   "system:openshift-deployer",
		Groups: util.NewStringSet("system:deployers"),
	}
}

func DefaultOpenshiftLoopbackClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, "openshift-client"),
			KeyFile:  DefaultKeyFilename(certDir, "openshift-client"),
		},
		SubDir: "openshift-client",
		User:   "system:openshift-client",
	}
}

func DefaultKubeClientClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, "kube-client"),
			KeyFile:  DefaultKeyFilename(certDir, "kube-client"),
		},
		SubDir: "kube-client",
		User:   "system:kube-client",
	}
}

func DefaultClusterAdminClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, "admin"),
			KeyFile:  DefaultKeyFilename(certDir, "admin"),
		},
		SubDir: "admin",
		User:   "system:admin",
		Groups: util.NewStringSet("system:cluster-admins"),
	}
}

func DefaultServerCerts(certDir string) []configapi.CertInfo {
	return []configapi.CertInfo{
		DefaultMasterServingCertInfo(certDir),
		DefaultAssetServingCertInfo(certDir),
		DefaultEtcdServingCertInfo(certDir),
	}
}

func DefaultMasterServingCertInfo(certDir string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(certDir, DefaultMasterDir, "server.crt"),
		KeyFile:  path.Join(certDir, DefaultMasterDir, "server.key"),
	}
}

func DefaultAssetServingCertInfo(certDir string) configapi.CertInfo {
	// Use master certs for assets also
	return DefaultMasterServingCertInfo(certDir)
}

func DefaultEtcdServingCertInfo(certDir string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(certDir, "etcd", "server.crt"),
		KeyFile:  path.Join(certDir, "etcd", "server.key"),
	}
}

func DefaultNodeDir(nodeName string) string {
	return "node-" + nodeName
}

func DefaultNodeServingCertInfo(certDir, nodeName string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(certDir, DefaultNodeDir(nodeName), "server.crt"),
		KeyFile:  path.Join(certDir, DefaultNodeDir(nodeName), "server.key"),
	}
}
func DefaultNodeClientCertInfo(certDir, nodeName string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(certDir, DefaultNodeDir(nodeName), "master-client.crt"),
		KeyFile:  path.Join(certDir, DefaultNodeDir(nodeName), "master-client.key"),
	}
}
func DefaultNodeKubeConfigFile(certDir, nodeName string) string {
	return path.Join(certDir, DefaultNodeDir(nodeName), ".kubeconfig")
}

func DefaultCertDir(certDir, username string) string {
	return path.Join(certDir, username)
}

func DefaultCertFilename(certDir, username string) string {
	return path.Join(DefaultCertDir(certDir, username), "cert.crt")
}

func DefaultKeyFilename(certDir, username string) string {
	return path.Join(DefaultCertDir(certDir, username), "key.key")
}
func DefaultSerialFilename(certDir, username string) string {
	return path.Join(DefaultCertDir(certDir, username), "serial.txt")
}
func DefaultKubeConfigFilename(certDir, username string) string {
	return path.Join(DefaultCertDir(certDir, username), ".kubeconfig")
}
