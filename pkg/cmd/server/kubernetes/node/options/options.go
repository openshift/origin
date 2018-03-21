package node

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/features"
	kubeletcni "k8s.io/kubernetes/pkg/kubelet/network/cni"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/network"
)

// ComputeKubeletFlags returns the flags to use when starting the kubelet.
func ComputeKubeletFlags(startingArgs map[string][]string, options configapi.NodeConfig) ([]string, error) {
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

	// Override kubelet iptables-masquerade-bit value to match overridden kube-proxy
	// iptables-masquerade-bit value, UNLESS the user has overridden kube-proxy to match the
	// previously-not-overridden kubelet value, in which case we don't want to re-break them.
	if len(options.ProxyArguments["iptables-masquerade-bit"]) != 1 || options.ProxyArguments["iptables-masquerade-bit"][0] != "14" {
		setIfUnset(args, "iptables-masquerade-bit", "0")
	}

	if network.IsOpenShiftNetworkPlugin(options.NetworkConfig.NetworkPluginName) {
		// SDN plugin pod setup/teardown is implemented as a CNI plugin
		setIfUnset(args, "network-plugin", kubeletcni.CNIPluginName)
	} else {
		setIfUnset(args, "network-plugin", options.NetworkConfig.NetworkPluginName)
	}

	// prevents kube from generating certs
	setIfUnset(args, "tls-cert-file", options.ServingInfo.ServerCert.CertFile)
	setIfUnset(args, "tls-private-key-file", options.ServingInfo.ServerCert.KeyFile)
	// roundtrip to get a default value
	setIfUnset(args, "tls-cipher-suites", crypto.CipherSuitesToNamesOrDie(crypto.CipherSuitesOrDie(options.ServingInfo.CipherSuites))...)
	setIfUnset(args, "tls-min-version", crypto.TLSVersionToNameOrDie(crypto.TLSVersionOrDie(options.ServingInfo.MinTLSVersion)))

	// Server cert rotation is ineffective if a cert is hardcoded.
	if len(args["feature-gates"]) > 0 {
		// TODO this affects global state, but it matches what happens later.  Need a less side-effecty way to do it
		if err := utilfeature.DefaultFeatureGate.Set(args["feature-gates"][0]); err != nil {
			return nil, err
		}
		if utilfeature.DefaultFeatureGate.Enabled(features.RotateKubeletServerCertificate) {
			// Server cert rotation is ineffective if a cert is hardcoded.
			setIfUnset(args, "tls-cert-file", "")
			setIfUnset(args, "tls-private-key-file", "")
		}
	}

	// default cluster-dns to the master's DNS if possible, but only if we can reach the master
	// TODO: this exists to support legacy cases where the node defaulted to the master's DNS.
	//   we can remove this when we drop support for master DNS when CoreDNS is in use everywhere.
	if len(args["cluster-dns"]) == 0 {
		if externalKubeClient, _, err := configapi.GetExternalKubeClient(options.MasterKubeConfig, options.MasterClientConnectionOverrides); err == nil {
			args["cluster-dns"] = getClusterDNS(externalKubeClient, args["cluster-dns"])
		}
	}

	// there is a special case.  If you set `--cgroups-per-qos=false` and `--enforce-node-allocatable` is
	// an empty string, `--enforce-node-allocatable=""` needs to be explicitly set
	// cgroups-per-qos defaults to true
	if cgroupArg, enforceAllocatable := args["cgroups-per-qos"], args["enforce-node-allocatable"]; len(cgroupArg) == 1 && cgroupArg[0] == "false" && len(enforceAllocatable) == 0 {
		args["enforce-node-allocatable"] = []string{""}
	}

	var keys []string
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var arguments []string
	for _, key := range keys {
		for _, token := range args[key] {
			arguments = append(arguments, fmt.Sprintf("--%s=%v", key, token))
		}
	}
	return arguments, nil
}

func setIfUnset(cmdLineArgs map[string][]string, key string, value ...string) {
	if _, ok := cmdLineArgs[key]; !ok {
		cmdLineArgs[key] = value
	}
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

func getClusterDNS(dnsClient kclientsetexternal.Interface, currClusterDNS []string) []string {
	var clusterDNS net.IP
	if len(currClusterDNS) == 0 {
		if service, err := dnsClient.Core().Services(metav1.NamespaceDefault).Get("kubernetes", metav1.GetOptions{}); err == nil {
			if includesServicePort(service.Spec.Ports, 53, "dns") {
				// Use master service if service includes "dns" port 53.
				clusterDNS = net.ParseIP(service.Spec.ClusterIP)
			}
		}
	}
	if clusterDNS == nil {
		if endpoint, err := dnsClient.Core().Endpoints(metav1.NamespaceDefault).Get("kubernetes", metav1.GetOptions{}); err == nil {
			if endpointIP, ok := firstEndpointIPWithNamedPort(endpoint, 53, "dns"); ok {
				// Use first endpoint if endpoint includes "dns" port 53.
				clusterDNS = net.ParseIP(endpointIP)
			} else if endpointIP, ok := firstEndpointIP(endpoint, 53); ok {
				// Test and use first endpoint if endpoint includes any port 53.
				if err := cmdutil.WaitForSuccessfulDial(false, "tcp", fmt.Sprintf("%s:%d", endpointIP, 53), 50*time.Millisecond, 0, 2); err == nil {
					clusterDNS = net.ParseIP(endpointIP)
				}
			}
		}
	}
	if clusterDNS != nil && !clusterDNS.IsUnspecified() {
		return []string{clusterDNS.String()}
	}

	return currClusterDNS
}

// TODO: more generic location
func includesEndpointPort(ports []kapiv1.EndpointPort, port int) bool {
	for _, p := range ports {
		if p.Port == int32(port) {
			return true
		}
	}
	return false
}

// TODO: more generic location
func includesServicePort(ports []kapiv1.ServicePort, port int, portName string) bool {
	for _, p := range ports {
		if p.Port == int32(port) && p.Name == portName {
			return true
		}
	}
	return false
}

// TODO: more generic location
func firstEndpointIP(endpoints *kapiv1.Endpoints, port int) (string, bool) {
	for _, s := range endpoints.Subsets {
		if !includesEndpointPort(s.Ports, port) {
			continue
		}
		for _, a := range s.Addresses {
			return a.IP, true
		}
	}
	return "", false
}

// TODO: more generic location
func firstEndpointIPWithNamedPort(endpoints *kapiv1.Endpoints, port int, portName string) (string, bool) {
	for _, s := range endpoints.Subsets {
		if !includesNamedEndpointPort(s.Ports, port, portName) {
			continue
		}
		for _, a := range s.Addresses {
			return a.IP, true
		}
	}
	return "", false
}

// TODO: more generic location
func includesNamedEndpointPort(ports []kapiv1.EndpointPort, port int, portName string) bool {
	for _, p := range ports {
		if p.Port == int32(port) && p.Name == portName {
			return true
		}
	}
	return false
}
