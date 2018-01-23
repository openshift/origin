package node

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/volume"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/volume/emptydir"
)

// TODO this is a best effort check at the moment that should either move to kubelet or be removed entirely
// EnsureKubeletAccess performs a number of test operations that the Kubelet requires to properly function.
// All errors here are fatal.
func EnsureKubeletAccess() {
	if cmdutil.Env("OPENSHIFT_CONTAINERIZED", "") == "true" {
		if _, err := os.Stat("/rootfs"); os.IsPermission(err) || os.IsNotExist(err) {
			glog.Fatal("error: Running in containerized mode, but cannot find the /rootfs directory - be sure to mount the host filesystem at /rootfs (read-only) in the container.")
		}
		if !sameFileStat(true, "/rootfs/sys", "/sys") {
			glog.Fatal("error: Running in containerized mode, but the /sys directory in the container does not appear to match the host /sys directory - be sure to mount /sys into the container.")
		}
		if !sameFileStat(true, "/rootfs/var/run", "/var/run") {
			glog.Fatal("error: Running in containerized mode, but the /var/run directory in the container does not appear to match the host /var/run directory - be sure to mount /var/run (read-write) into the container.")
		}
	}
	// TODO: check whether we can mount disks (for volumes)
	// TODO: check things cAdvisor needs to properly function
	// TODO: test a cGroup move?
}

// sameFileStat checks whether the provided paths are the same file, to verify that a user has correctly
// mounted those binaries
func sameFileStat(requireMode bool, src, dst string) bool {
	srcStat, err := os.Stat(src)
	if err != nil {
		glog.V(4).Infof("Unable to stat %q: %v", src, err)
		return false
	}
	dstStat, err := os.Stat(dst)
	if err != nil {
		glog.V(4).Infof("Unable to stat %q: %v", dst, err)
		return false
	}
	if requireMode && srcStat.Mode() != dstStat.Mode() {
		glog.V(4).Infof("Mode mismatch between %q (%s) and %q (%s)", src, srcStat.Mode(), dst, dstStat.Mode())
		return false
	}
	if !os.SameFile(srcStat, dstStat) {
		glog.V(4).Infof("inode and device mismatch between %q (%s) and %q (%s)", src, srcStat, dst, dstStat)
		return false
	}
	return true
}

// TODO we need to stop doing this or get it upstream
// EnsureVolumeDir attempts to convert the provided volume directory argument to
// an absolute path and create the directory if it does not exist. Will exit if
// an error is encountered.
func EnsureVolumeDir(volumeDirName string) {
	if err := initializeVolumeDir(volumeDirName); err != nil {
		glog.Fatal(err)
	}
}

func initializeVolumeDir(rootDirectory string) error {
	if !filepath.IsAbs(rootDirectory) {
		return fmt.Errorf("%q is not an absolute path", rootDirectory)
	}

	if _, err := os.Stat(rootDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(rootDirectory, 0750); err != nil {
			return fmt.Errorf("Couldn't create kubelet volume root directory '%s': %s", rootDirectory, err)
		}
	}
	return nil
}

// TODO this needs to move into the forked kubelet with a `--openshift-config` flag
// PatchUpstreamVolumePluginsForLocalQuota checks if the node config specifies a local storage
// perFSGroup quota, and if so will test that the volumeDirectory is on a
// filesystem suitable for quota enforcement. If checks pass the k8s emptyDir
// volume plugin will be replaced with a wrapper version which adds quota
// functionality.
func PatchUpstreamVolumePluginsForLocalQuota(nodeConfig configapi.NodeConfig) func() []volume.VolumePlugin {
	// This looks a little weird written this way but it allows straight lifting from here to kube at a future time
	// and will allow us to wrap the exec.

	existingProbeVolumePlugins := app.ProbeVolumePlugins
	return func() []volume.VolumePlugin {
		if nodeConfig.VolumeConfig.LocalQuota.PerFSGroup == nil {
			return existingProbeVolumePlugins()
		}

		glog.V(4).Info("Replacing empty-dir volume plugin with quota wrapper")
		wrappedEmptyDirPlugin := false

		quotaApplicator, err := emptydir.NewQuotaApplicator(nodeConfig.VolumeDirectory)
		if err != nil {
			glog.Fatalf("Could not set up local quota, %s", err)
		}

		// Create a volume spec with emptyDir we can use to search for the
		// emptyDir plugin with CanSupport:
		emptyDirSpec := &volume.Spec{
			Volume: &kapiv1.Volume{
				VolumeSource: kapiv1.VolumeSource{
					EmptyDir: &kapiv1.EmptyDirVolumeSource{},
				},
			},
		}

		ret := existingProbeVolumePlugins()
		for idx, plugin := range ret {
			// Can't really do type checking or use a constant here as they are not exported:
			if plugin.CanSupport(emptyDirSpec) {
				wrapper := emptydir.EmptyDirQuotaPlugin{
					VolumePlugin:    plugin,
					Quota:           *nodeConfig.VolumeConfig.LocalQuota.PerFSGroup,
					QuotaApplicator: quotaApplicator,
				}
				ret[idx] = &wrapper
				wrappedEmptyDirPlugin = true
			}
		}
		// Because we can't look for the k8s emptyDir plugin by any means that would
		// survive a refactor, error out if we couldn't find it:
		if !wrappedEmptyDirPlugin {
			glog.Fatal(errors.New("No plugin handling EmptyDir was found, unable to apply local quotas"))
		}

		return ret
	}
}

func GetClusterDNS(dnsClient kclientsetexternal.Interface, currClusterDNS []string) []string {
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
func includesServicePort(ports []kapiv1.ServicePort, port int, portName string) bool {
	for _, p := range ports {
		if p.Port == int32(port) && p.Name == portName {
			return true
		}
	}
	return false
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
