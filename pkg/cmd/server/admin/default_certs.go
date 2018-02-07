package admin

import (
	"fmt"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

const (
	CAFilePrefix     = "ca"
	CABundlePrefix   = "ca-bundle"
	MasterFilePrefix = "master"

	FrontProxyCAFilePrefix = "frontproxy-ca"
)

type ClientCertInfo struct {
	CertLocation    configapi.CertInfo
	UnqualifiedUser string
	User            string
	Groups          sets.String
}

func DefaultSignerName() string {
	return fmt.Sprintf("%s@%d", "openshift-signer", time.Now().Unix())
}

func DefaultCABundleFile(certDir string) string {
	return DefaultCertFilename(certDir, CABundlePrefix)
}

func DefaultServiceServingCertSignerName() string {
	return fmt.Sprintf("%s@%d", "openshift-service-serving-signer", time.Now().Unix())
}

func DefaultRootCAFile(certDir string) string {
	return DefaultCertFilename(certDir, CAFilePrefix)
}

func DefaultKubeletClientCAFile(certDir string) string {
	return DefaultRootCAFile(certDir)
}

func DefaultKubeletClientCerts(certDir string) []ClientCertInfo {
	return []ClientCertInfo{
		DefaultMasterKubeletClientCertInfo(certDir),
	}
}

func DefaultFrontProxySignerName() string {
	return fmt.Sprintf("%s@%d", "aggregator-proxy-ca", time.Now().Unix())
}

func DefaultMasterKubeletClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: path.Join(certDir, MasterFilePrefix+".kubelet-client.crt"),
			KeyFile:  path.Join(certDir, MasterFilePrefix+".kubelet-client.key"),
		},
		User:   bootstrappolicy.MasterKubeletAdminClientUsername,
		Groups: sets.NewString(bootstrappolicy.NodeAdminsGroup),
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
			CertFile: path.Join(certDir, MasterFilePrefix+".etcd-client.crt"),
			KeyFile:  path.Join(certDir, MasterFilePrefix+".etcd-client.key"),
		},
		User: "system:master",
	}
}

func DefaultProxyClientCerts(certDir string) []ClientCertInfo {
	return []ClientCertInfo{
		DefaultProxyClientCertInfo(certDir),
	}
}
func DefaultProxyClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: path.Join(certDir, MasterFilePrefix+".proxy-client.crt"),
			KeyFile:  path.Join(certDir, MasterFilePrefix+".proxy-client.key"),
		},
		User: bootstrappolicy.MasterProxyUsername,
	}
}

func DefaultAPIClientCAFile(certDir string) string {
	return DefaultRootCAFile(certDir)
}

func DefaultAPIClientCerts(certDir string) []ClientCertInfo {
	return []ClientCertInfo{
		DefaultOpenshiftLoopbackClientCertInfo(certDir),
		DefaultClusterAdminClientCertInfo(certDir),
	}
}

func DefaultOpenshiftLoopbackClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, bootstrappolicy.MasterUnqualifiedUsername),
			KeyFile:  DefaultKeyFilename(certDir, bootstrappolicy.MasterUnqualifiedUsername),
		},
		UnqualifiedUser: bootstrappolicy.MasterUnqualifiedUsername,
		User:            bootstrappolicy.MasterUsername,
		Groups:          sets.NewString(bootstrappolicy.MastersGroup),
	}
}

func DefaultAggregatorClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, bootstrappolicy.AggregatorUnqualifiedUsername),
			KeyFile:  DefaultKeyFilename(certDir, bootstrappolicy.AggregatorUnqualifiedUsername),
		},
		UnqualifiedUser: bootstrappolicy.AggregatorUnqualifiedUsername,
		User:            bootstrappolicy.AggregatorUsername,
		Groups:          sets.String{},
	}
}

func DefaultClusterAdminClientCertInfo(certDir string) ClientCertInfo {
	return ClientCertInfo{
		CertLocation: configapi.CertInfo{
			CertFile: DefaultCertFilename(certDir, "admin"),
			KeyFile:  DefaultKeyFilename(certDir, "admin"),
		},
		UnqualifiedUser: "admin",
		User:            "system:admin",
		Groups:          sets.NewString(bootstrappolicy.ClusterAdminGroup, bootstrappolicy.MastersGroup),
	}
}

func DefaultServerCerts(certDir string) []configapi.CertInfo {
	return []configapi.CertInfo{
		DefaultMasterServingCertInfo(certDir),
		DefaultEtcdServingCertInfo(certDir),
	}
}

func DefaultMasterServingCertInfo(certDir string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(certDir, MasterFilePrefix+".server.crt"),
		KeyFile:  path.Join(certDir, MasterFilePrefix+".server.key"),
	}
}

func DefaultEtcdServingCertInfo(certDir string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(certDir, "etcd.server.crt"),
		KeyFile:  path.Join(certDir, "etcd.server.key"),
	}
}

func DefaultServiceAccountPrivateKeyFile(certDir string) string {
	return path.Join(certDir, "serviceaccounts.private.key")
}
func DefaultServiceAccountPublicKeyFile(certDir string) string {
	return path.Join(certDir, "serviceaccounts.public.key")
}

func DefaultNodeDir(nodeName string) string {
	return "node-" + nodeName
}

func DefaultNodeServingCertInfo(nodeDir string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(nodeDir, "server.crt"),
		KeyFile:  path.Join(nodeDir, "server.key"),
	}
}
func DefaultNodeClientCertInfo(nodeDir string) configapi.CertInfo {
	return configapi.CertInfo{
		CertFile: path.Join(nodeDir, "master-client.crt"),
		KeyFile:  path.Join(nodeDir, "master-client.key"),
	}
}
func DefaultNodeKubeConfigFile(nodeDir string) string {
	return path.Join(nodeDir, "node.kubeconfig")
}

func DefaultServiceSignerCAInfo(certDir string) configapi.CertInfo {
	caInfo := configapi.CertInfo{}
	caInfo.CertFile = DefaultCAFilename(certDir, "service-signer")
	caInfo.KeyFile = DefaultKeyFilename(certDir, "service-signer")
	return caInfo
}

func DefaultCAFilename(certDir, prefix string) string {
	return path.Join(certDir, prefix+".crt")
}
func DefaultCertFilename(certDir, prefix string) string {
	return path.Join(certDir, prefix+".crt")
}
func DefaultKeyFilename(certDir, prefix string) string {
	return path.Join(certDir, prefix+".key")
}
func DefaultSerialFilename(certDir, prefix string) string {
	return path.Join(certDir, prefix+".serial.txt")
}
func DefaultKubeConfigFilename(certDir, prefix string) string {
	return path.Join(certDir, prefix+".kubeconfig")
}
