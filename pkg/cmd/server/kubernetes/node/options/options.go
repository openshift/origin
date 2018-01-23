package node

import (
	"fmt"
	"net"
	"strings"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig"
	kubeletcni "k8s.io/kubernetes/pkg/kubelet/network/cni"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/network"
)

// computeKubeletFlags returns the flags to use when starting the kubelet
// TODO this needs to return a []string and be passed to cobra, but as an intermediate step, we'll compute the map and run it through the existing paths
func computeKubeletFlags(startingArgs map[string][]string, options configapi.NodeConfig) (map[string][]string, error) {
	args := map[string][]string{}
	for key, slice := range startingArgs {
		for _, val := range slice {
			args[key] = append(args[key], val)
		}
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	path := ""
	var fileCheckInterval int64
	if options.PodManifestConfig != nil {
		path = options.PodManifestConfig.Path
		fileCheckInterval = options.PodManifestConfig.FileCheckIntervalSeconds
	}
	kubeAddressStr, kubePortStr, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("cannot parse node address: %v", err)
	}

	setIfUnset(args, "address", kubeAddressStr)
	setIfUnset(args, "port", kubePortStr)
	setIfUnset(args, "require-kubeconfig", "true")
	setIfUnset(args, "kubeconfig", options.MasterKubeConfig)
	setIfUnset(args, "pod-manifest-path", path)
	setIfUnset(args, "root-dir", options.VolumeDirectory)
	setIfUnset(args, "node-ip", options.NodeIP)
	setIfUnset(args, "hostname-override", options.NodeName)
	setIfUnset(args, "allow-privileged", "true")
	setIfUnset(args, "register-node", "true")
	setIfUnset(args, "read-only-port", "0")      // no read only access
	setIfUnset(args, "cadvisor-port", "0")       // no unsecured cadvisor access
	setIfUnset(args, "healthz-port", "0")        // no unsecured healthz access
	setIfUnset(args, "healthz-bind-address", "") // no unsecured healthz access
	setIfUnset(args, "cluster-dns", options.DNSIP)
	setIfUnset(args, "cluster-domain", options.DNSDomain)
	setIfUnset(args, "host-network-sources", kubelettypes.ApiserverSource, kubelettypes.FileSource)
	setIfUnset(args, "host-pid-sources", kubelettypes.ApiserverSource, kubelettypes.FileSource)
	setIfUnset(args, "host-ipc-sources", kubelettypes.ApiserverSource, kubelettypes.FileSource)
	setIfUnset(args, "http-check-frequency", "0s") // no remote HTTP pod creation access
	setIfUnset(args, "file-check-frequency", fmt.Sprintf("%ds", fileCheckInterval))
	setIfUnset(args, "pod-infra-container-image", imageTemplate.ExpandOrDie("pod"))
	setIfUnset(args, "max-pods", "250")
	setIfUnset(args, "pods-per-core", "10")
	setIfUnset(args, "cgroup-driver", "systemd")
	setIfUnset(args, "container-runtime-endpoint", options.DockerConfig.DockerShimSocket)
	setIfUnset(args, "image-service-endpoint", options.DockerConfig.DockerShimSocket)
	setIfUnset(args, "experimental-dockershim-root-directory", options.DockerConfig.DockershimRootDirectory)
	setIfUnset(args, "containerized", fmt.Sprintf("%v", cmdutil.Env("OPENSHIFT_CONTAINERIZED", "") == "true"))
	setIfUnset(args, "authentication-token-webhook", "true")
	setIfUnset(args, "authentication-token-webhook-cache-ttl", options.AuthConfig.AuthenticationCacheTTL)
	setIfUnset(args, "anonymous-auth", "true")
	setIfUnset(args, "client-ca-file", options.ServingInfo.ClientCA)
	setIfUnset(args, "authorization-mode", "Webhook")
	setIfUnset(args, "authorization-webhook-cache-authorized-ttl", options.AuthConfig.AuthorizationCacheTTL)
	setIfUnset(args, "authorization-webhook-cache-unauthorized-ttl", options.AuthConfig.AuthorizationCacheTTL)

	if network.IsOpenShiftNetworkPlugin(options.NetworkConfig.NetworkPluginName) {
		// SDN plugin pod setup/teardown is implemented as a CNI plugin
		setIfUnset(args, "network-plugin", kubeletcni.CNIPluginName)
		setIfUnset(args, "cni-conf-dir", kubeletcni.DefaultNetDir)
		setIfUnset(args, "cni-bin-dir", kubeletcni.DefaultCNIDir)
		setIfUnset(args, "hairpin-mode", kubeletconfig.HairpinNone)
	} else {
		setIfUnset(args, "network-plugin", options.NetworkConfig.NetworkPluginName)
	}

	// prevents kube from generating certs
	setIfUnset(args, "tls-cert-file", options.ServingInfo.ServerCert.CertFile)
	setIfUnset(args, "tls-private-key-file", options.ServingInfo.ServerCert.KeyFile)
	// roundtrip to get a default value
	setIfUnset(args, "tls-cipher-suites", crypto.CipherSuitesToNamesOrDie(crypto.CipherSuitesOrDie(options.ServingInfo.CipherSuites))...)
	setIfUnset(args, "tls-min-version", crypto.TLSVersionToNameOrDie(crypto.TLSVersionOrDie(options.ServingInfo.MinTLSVersion)))

	return args, nil
}

func setIfUnset(cmdLineArgs map[string][]string, key string, value ...string) {
	if _, ok := cmdLineArgs[key]; !ok {
		cmdLineArgs[key] = value
	}
}

// Build creates the core Kubernetes component configs for a given NodeConfig, or returns
// an error
func Build(options configapi.NodeConfig) (*kubeletoptions.KubeletServer, error) {
	kubeletFlags, err := computeKubeletFlags(options.KubeletArguments, options)
	if err != nil {
		return nil, fmt.Errorf("cannot create kubelet args: %v", err)
	}

	// Defaults are tested in TestKubeletDefaults
	server, err := kubeletoptions.NewKubeletServer()
	if err != nil {
		return nil, fmt.Errorf("cannot create kubelet server: %v", err)
	}
	if err := cmdflags.Resolve(kubeletFlags, server.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	// terminate early if feature gate is incorrect on the node
	if len(server.FeatureGates) > 0 {
		if err := utilfeature.DefaultFeatureGate.SetFromMap(server.FeatureGates); err != nil {
			return nil, err
		}
	}
	if utilfeature.DefaultFeatureGate.Enabled(features.RotateKubeletServerCertificate) {
		// Server cert rotation is ineffective if a cert is hardcoded.
		if len(server.CertDirectory) > 0 {
			server.TLSCertFile = ""
			server.TLSPrivateKeyFile = ""
		}
	}

	return server, nil
}

func ToFlags(config *kubeletoptions.KubeletServer) []string {
	server, _ := kubeletoptions.NewKubeletServer()
	args := cmdflags.AsArgs(config.AddFlags, server.AddFlags)

	// there is a special case.  If you set `--cgroups-per-qos=false` and `--enforce-node-allocatable` is
	// an empty string, `--enforce-node-allocatable=""` needs to be explicitly set
	if !config.CgroupsPerQOS && len(config.EnforceNodeAllocatable) == 0 {
		args = append(args, `--enforce-node-allocatable=`)
	}

	return args
}

// Some flags are *required* to be set when running from openshift start node.  This ensures they are set.
// If they are not set, we fail.  This is compensating for some lost integration tests.
func CheckFlags(args []string) error {
	if needle := "--authentication-token-webhook=true"; !hasArg(needle, args) {
		return fmt.Errorf("missing %v: %v", needle, args)
	}
	if needle := "--authorization-mode=Webhook"; !hasArg(needle, args) {
		return fmt.Errorf("missing %v: %v", needle, args)
	}
	if needle := "--tls-min-version="; !hasArgPrefix(needle, args) {
		return fmt.Errorf("missing %v: %v", needle, args)
	}
	if needle := "--tls-cipher-suites="; !hasArgPrefix(needle, args) {
		return fmt.Errorf("missing %v: %v", needle, args)
	}

	return nil
}

func hasArg(needle string, haystack []string) bool {
	return sets.NewString(haystack...).Has(needle)
}

func hasArgPrefix(needle string, haystack []string) bool {
	for _, haystackToken := range haystack {
		if strings.HasPrefix(haystackToken, needle) {
			return true
		}
	}

	return false
}
