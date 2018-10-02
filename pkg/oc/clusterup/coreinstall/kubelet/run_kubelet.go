package kubelet

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/openshift/origin/pkg/oc/clusterup/docker/openshift"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/util"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

type KubeletRunConfig struct {
	// ContainerBinds is a list of local/path:image/path pairs
	ContainerBinds []string
	// NodeImage is the docker image for openshift start node
	NodeImage   string
	PodImage    string
	Environment []string

	HTTPProxy  string
	HTTPSProxy string
	NoProxy    []string

	DockerRoot               string
	HostVolumesDir           string
	HostPersistentVolumesDir string
	UseNsenterMount          bool

	Args []string
}

func NewKubeletRunConfig() *KubeletRunConfig {
	return &KubeletRunConfig{
		ContainerBinds: []string{
			"/var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins:rw",
			"/var/log:/var/log:rw",
			"/var/run:/var/run:rw",
			"/sys:/sys:rw",
			"/sys/fs/cgroup:/sys/fs/cgroup:rw",
			"/dev:/dev",
		},
	}

}

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (opt KubeletRunConfig) StartKubelet(dockerClient util.Interface, podManifestDir, assetsDir, logdir string) (string, error) {
	kubeletFlags := []string{
		"--address=0.0.0.0",
		"--allow-privileged=true",
		"--anonymous-auth=true",
		"--authentication-token-webhook=true",
		"--authentication-token-webhook-cache-ttl=5m",
		"--authorization-mode=Webhook",
		"--authorization-webhook-cache-authorized-ttl=5m",
		"--authorization-webhook-cache-unauthorized-ttl=5m",
		"--cadvisor-port=0",
		"--cgroup-driver=systemd",
		"--client-ca-file=/var/lib/origin/bootkube/tls/kube-ca.crt",
		"--cluster-domain=cluster.local",
		"--container-runtime-endpoint=unix:///var/run/dockershim.sock",
		"--containerized=true",
		"--experimental-dockershim-root-directory=/var/lib/dockershim",
		"--fail-swap-on=false",
		"--healthz-bind-address=",
		"--healthz-port=0",
		"--host-ipc-sources=api",
		"--host-ipc-sources=file",
		"--host-network-sources=api",
		"--host-network-sources=file",
		"--host-pid-sources=api",
		"--host-pid-sources=file",
		"--hostname-override=localhost",
		"--http-check-frequency=0s",
		"--image-service-endpoint=unix:///var/run/dockershim.sock",
		"--iptables-masquerade-bit=0",
		"--kubeconfig=/var/lib/origin/bootkube/auth/kubeconfig",
		"--max-pods=250",
		"--network-plugin=",
		"--node-ip=",
		// "--pod-infra-container-image=openshift/origin-pod:latest",
		fmt.Sprintf("--pod-infra-container-image=%s", opt.PodImage),
		"--port=10250",
		"--read-only-port=0",
		"--register-node=true",
		"--node-labels=node-role.kubernetes.io/master=",
		"--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		"--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		"--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
		"--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
		"--tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
		"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		"--tls-cipher-suites=TLS_RSA_WITH_AES_128_GCM_SHA256",
		"--tls-cipher-suites=TLS_RSA_WITH_AES_256_GCM_SHA384",
		"--tls-cipher-suites=TLS_RSA_WITH_AES_128_CBC_SHA",
		"--tls-cipher-suites=TLS_RSA_WITH_AES_256_CBC_SHA",
		"--tls-min-version=VersionTLS12",
		"--tls-cert-file=/var/lib/origin/bootkube/tls/apiserver.crt",
		"--tls-private-key-file=/var/lib/origin/bootkube/tls/apiserver.key",
		"--pod-manifest-path=/var/lib/origin/pod-manifests",
		"--file-check-frequency=1s",
		"--cluster-dns=172.30.0.2",
		"--v=5",
	}
	hostVolumesDir := opt.HostVolumesDir

	kubeletFlags = append(kubeletFlags, "--root-dir="+opt.HostVolumesDir)

	opt.ContainerBinds = append(opt.ContainerBinds, podManifestDir+":/var/lib/origin/pod-manifests:z")
	opt.ContainerBinds = append(opt.ContainerBinds, assetsDir+":/var/lib/origin/bootkube:z")
	opt.ContainerBinds = append(opt.ContainerBinds, fmt.Sprintf("%[1]s:%[1]s", opt.HostPersistentVolumesDir))

	opt.Environment = append(opt.Environment, fmt.Sprintf("OPENSHIFT_PV_DIR=%s", opt.HostPersistentVolumesDir))
	if !opt.UseNsenterMount {
		hostVolumeDirSource := hostVolumesDir
		if runtime.GOOS != "linux" {
			hostVolumeDirSource = opt.HostVolumesDir
		}
		opt.ContainerBinds = append(opt.ContainerBinds, fmt.Sprintf("%s:%s:shared", hostVolumeDirSource, hostVolumesDir))
		opt.Environment = append(opt.Environment, "OPENSHIFT_CONTAINERIZED=false")
	} else {
		opt.ContainerBinds = append(opt.ContainerBinds, "/:/rootfs:ro")
		opt.ContainerBinds = append(opt.ContainerBinds, fmt.Sprintf("%[1]s:%[1]s:rslave", hostVolumesDir))
	}

	opt.ContainerBinds = append(opt.ContainerBinds, fmt.Sprintf("%[1]s:%[1]s", opt.DockerRoot))

	// Kubelet needs to be able to write to
	// /sys/devices/virtual/net/vethXXX/brport/hairpin_mode, so make this rw, not ro.
	opt.ContainerBinds = append(opt.ContainerBinds, "/sys/devices/virtual/net:/sys/devices/virtual/net:rw")

	imageRunHelper := run.NewRunHelper(util.NewHelper(dockerClient)).New()
	var env []string
	if len(opt.HTTPProxy) > 0 {
		env = append(env, fmt.Sprintf("HTTP_PROXY=%s", opt.HTTPProxy))
	}
	if len(opt.HTTPSProxy) > 0 {
		env = append(env, fmt.Sprintf("HTTPS_PROXY=%s", opt.HTTPSProxy))
	}
	if len(opt.NoProxy) > 0 {
		env = append(env, fmt.Sprintf("NO_PROXY=%s", strings.Join(opt.NoProxy, ",")))
	}
	env = append(env, opt.Environment...)

	runKubeletCmd := []string{
		"kubelet",
	}

	opt.Args = append(opt.Args, kubeletFlags...)
	runKubeletCmd = append(runKubeletCmd, opt.Args...)

	containerID, err := imageRunHelper.Image(opt.NodeImage).
		Name(openshift.OriginContainerName).
		Privileged().
		DiscardContainer().
		HostNetwork().
		HostPid().
		Bind(opt.ContainerBinds...).
		Env(env...).
		Entrypoint("hyperkube").
		Command(runKubeletCmd...).Start()
	if err != nil {
		return "", errors.NewError("unable to start kubelet: %v", err).WithCause(err)
	}

	return containerID, nil
}
