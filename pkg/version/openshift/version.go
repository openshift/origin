package openshift

import (
	"fmt"

	etcdversion "github.com/coreos/etcd/version"
	"github.com/openshift/origin/pkg/version"
	kubeversion "k8s.io/kubernetes/pkg/version"
)

// Info contains versioning information.
// TODO: Add []string of api versions supported? It's still unclear
// how we'll want to distribute that information.
type Info struct {
	version.Info

	EtcdVersion string
	KubeVersion string
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	return Info{
		Info:        version.Get(),
		KubeVersion: kubeversion.Get().String(),
		EtcdVersion: etcdversion.Version,
	}
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	kubeVersion := info.KubeVersion
	if len(kubeVersion) == 0 {
		kubeVersion = "unknown"
	}

	etcdVersion := info.EtcdVersion
	if len(etcdVersion) == 0 {
		etcdVersion = "unknown"
	}

	return fmt.Sprintf("%s\nkubernetes %s\netcd %s", info.Info, kubeVersion, etcdVersion)
}
