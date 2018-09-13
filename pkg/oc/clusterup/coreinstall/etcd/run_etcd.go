package etcd

import (
	"fmt"
	"path/filepath"

	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/openshift"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

type EtcdConfig struct {
	EtcdImage string

	// AssetsDir is location where bootkube generated assets (needed for TLS).
	AssetsDir string

	// EtcdDataDir is location where etcd will store data.
	EtcdDataDir string

	// ContainerBinds is location to additional container bind mounts for bootkube containers.
	ContainerBinds []string
}

// Start runs the bootkube render command. The assets produced by this commands are stored in AssetsDir.
func (opt *EtcdConfig) Start(dockerClient dockerhelper.Interface) (string, error) {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	etcdCommand := []string{
		"--trusted-ca-file", "/var/run/etcd/tls/etcd-client-ca.crt",
		"--cert-file", "/var/run/etcd/tls/etcd-client.crt",
		"--key-file", "/var/run/etcd/tls/etcd-client.key",
		"--client-cert-auth",
		"--listen-client-urls", "https://0.0.0.0:2379",
		"--advertise-client-urls", "https://0.0.0.0:2379",
		"--data-dir", "/var/run/etcd/data",
	}

	binds := opt.ContainerBinds
	binds = append(binds, fmt.Sprintf("%s:/var/run/etcd/tls:z", filepath.Join(opt.AssetsDir, "tls")))
	binds = append(binds, fmt.Sprintf("%s:/var/run/etcd/data:z", opt.EtcdDataDir))

	containerID, err := imageRunHelper.Image(opt.EtcdImage).
		Name(openshift.EtcdContainerName).
		HostNetwork().
		HostPid().
		Privileged().
		DiscardContainer().
		Bind(binds...).
		Entrypoint("/usr/local/bin/etcd").
		Command(etcdCommand...).Start()

	if err != nil {
		return "", errors.NewError("etcd exited: %v", err).WithCause(err)
	}

	return containerID, nil
}
