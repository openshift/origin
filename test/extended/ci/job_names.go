package ci

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcv1client "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	kclientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-ci] [Early] prow job name", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("job-names")
	jobName := os.Getenv("JOB_NAME")

	// If it's an e2e job, we're only interested in the parts that come after e2e, e.g.
	// a presubmit on pull-ci-openshift-ovn-kubernetes-master-e2e-hypershift, we don't want
	// to look at the repo name which contains the -ovn- substring!
	originalJobName := jobName
	parts := strings.Split(jobName, "e2e-")
	if len(parts) >= 2 {
		jobName = parts[len(parts)-1]
	}
	isPeriodic := strings.HasPrefix(jobName, "periodic-")

	g.It("should match feature set", func() {
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		}

		isTechPreview := exutil.IsTechPreviewNoUpgrade(context.TODO(), oc.AdminConfigClient())
		if strings.Contains(jobName, "-techpreview") && !isTechPreview {
			e2e.Failf("job name %q has mismatched feature set in name (expected techpreview in cluster feature set)", jobName)
		}
		if !strings.Contains(jobName, "-techpreview") && isTechPreview {
			e2e.Failf("job name %q has mismatched feature set in name (expected techpreview in job name)", jobName)
		}
	})

	g.It("should match security mode", func() {
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		}

		isFIPS, err := exutil.IsFIPS(oc.AdminKubeClient().CoreV1())
		e2e.ExpectNoError(err, "could not retrieve security mode")
		if strings.Contains(jobName, "-fips") && !isFIPS {
			e2e.Failf("job name %q has mismatched security mode in name (expected fips to be enabled)", jobName)
		}
		if !strings.Contains(jobName, "-fips") && isFIPS {
			e2e.Failf("job name %q has mismatched security mode in name (expected fips in job name)", jobName)
		}
	})

	g.It("should match platform type", func() {
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		} else if strings.Contains(jobName, "agnostic") {
			e2eskipper.Skipf("JOB_NAME contains agnostic, not expecting platform in name")
		}

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		hasPlatform := true
		platform := infra.Status.PlatformStatus.Type
		switch platform {
		case v1.AlibabaCloudPlatformType:
			if !strings.Contains(jobName, "alibaba") {
				hasPlatform = false
			}
		case v1.EquinixMetalPlatformType, v1.BareMetalPlatformType:
			if !strings.Contains(jobName, "metal") {
				hasPlatform = false
			}
		default:
			if !strings.Contains(jobName, strings.ToLower(string(platform))) {
				hasPlatform = false
			}
		}

		if !hasPlatform {
			result.Flakef("job name %q does not contain platform type in name (%s)", originalJobName, platform)
		}

	})

	g.It("should match network type", func() {
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		}

		network, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		switch network.Status.NetworkType {
		case "OpenShiftSDN":
			if strings.Contains(jobName, "ovn") {
				e2e.Failf("job name %q has mismatched network type in name (expected `sdn`, found `ovn`)", jobName)
			} else if !strings.Contains(jobName, "sdn") {
				failMessage := fmt.Sprintf("job name %q does not have network type in name (expected `sdn`)", jobName)
				if isPeriodic {
					e2e.Fail(failMessage)
				} else {
					result.Flakef("%s", failMessage)
				}
			}
		case "OVNKubernetes":
			if strings.Contains(jobName, "sdn") {
				e2e.Failf("job name %q has mismatched network type in name (expected `ovn`, found `sdn`)", jobName)
			} else if !strings.Contains(jobName, "ovn") {
				failMessage := fmt.Sprintf("job name %q does not have network type in name (expected `ovn`)", jobName)
				if isPeriodic {
					e2e.Fail(failMessage)
				} else {
					result.Flakef("%s", failMessage)
				}
			}
		default:
			// Use this to find any other cases, so we can update the test
			result.Flakef("job uses network type that's not ovn or sdn")
		}
	})

	g.It("should match cluster version [apigroup:config.openshift.io]", func() {
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		}
		if !isPeriodic {
			e2eskipper.Skipf("This is only expected to work on periodics, skipping")
		}

		jobNameVersions := make([]string, 0)
		// Look for OpenShift-like 4.x versions.
		// NOTE: This will break on OpenShift major bumps, but we have jobs that reference
		// non-openshift version numbers, limiting to 4.x reduces false positives.
		versionMatcher := regexp.MustCompile(`-(4.[0-9]+)`)
		matches := versionMatcher.FindAllStringSubmatch(jobName, -1)
		for _, match := range matches {
			jobNameVersions = append(jobNameVersions, match[1])
		}

		// The logic here is that if the job mentions something that looks like an OpenShift release, we want to make
		// sure that any current and past cluster versions x.y are reflected in the job name.  For example, a job upgrading
		// from 4.11 to 4.12 should reference both in the name.
		if len(jobNameVersions) == 0 {
			e2eskipper.Skipf("No versions found in job name, skipping.")
		}

		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	clusterVersionLoop:
		for _, clusterVersion := range cv.Status.History {
			for _, jobNameVersion := range jobNameVersions {
				if strings.HasPrefix(clusterVersion.Version, jobNameVersion) {
					continue clusterVersionLoop // found
				}
			}
			// if we reach here, we didn't find the cluster version in the job name
			result.Flakef("job name %q is missing cluster version %s", jobName, clusterVersion.Version)
		}
	})

	g.It("should match os version", func() {
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		}

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShift, err := exutil.IsHypershift(context.TODO(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		if isMicroShift {
			validateMicroshiftNodeOS(jobName)
		} else if isHyperShift {
			validateHypershiftNodeOS(jobName)
		} else {
			validateStandaloneNodeOS(oc, jobName)
		}
	})
})

func validatePreOSImageStreamsNodeOS(coreClient kclientset.Interface) {
	// In clusters with no OSImageStreams the nodes should be always RHEL 9
	nodes, err := coreClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error listing nodes")

	for _, node := range nodes.Items {
		osImage := node.Status.NodeInfo.OSImage
		o.Expect(osImage).To(o.ContainSubstring("CoreOS 9."), "Pre OS Image Stream cluster should use RHEL 9 nodes")
	}
}

func fetchMCPStreams(machineConfigClient mcv1client.Interface, osImageStreams *mcfgv1.OSImageStream) map[string]string {
	mcps, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error fetching MCPs")

	var workerMCP *mcfgv1.MachineConfigPool
	for _, mcp := range mcps.Items {
		if mcp.Name == "worker" {
			workerMCP = &mcp
			break
		}
	}
	o.Expect(workerMCP).NotTo(o.BeNil(), "Cluster worker MCP does not exist")

	mcpsStreams := make(map[string]string)
	for _, mcp := range mcps.Items {
		stream := mcp.Spec.OSImageStream.Name
		if stream == "" {
			// It can be a custom MCP using the stream of the parent
			if mcp.Name == "master" || mcp.Name == "worker" || mcp.Name == "arbiter" {
				// Not a custom pool: It's using the cluster-wide stream
				stream = osImageStreams.Status.DefaultStream
			} else if workerMCP.Spec.OSImageStream.Name != "" {
				stream = workerMCP.Spec.OSImageStream.Name
			} else {
				stream = osImageStreams.Status.DefaultStream
			}
		}
		mcpsStreams[mcp.Name] = stream
	}

	return mcpsStreams
}

func validateMicroshiftNodeOS(jobName string) {
	if strings.Contains(jobName, "rhcos10") || strings.Contains(jobName, "rhcos9-10") {
		// TODO(muller): Assume we do not have RHCOS10/mixed microshift jobs now. If someone adds such a job, this failure
		// should force them to figure out how to detect RHCOS10 in microshift and update this test.
		e2e.Failf("TODO: job name %q indicates RHCOS10 or mixed OS which cannot be checked for MicroShift clusters now", jobName)
		return
	}

	e2eskipper.Skip("Cannot check RHCOS for MicroShift clusters")

}

func validateHypershiftNodeOS(jobName string) {
	if strings.Contains(jobName, "rhcos10") || strings.Contains(jobName, "rhcos9-10") {
		// TODO(muller): Assume we do not have RHCOS10/mixed hypershift jobs now. If someone adds such a job, this failure
		// should force them to figure out how to detect RHCOS10 in hypershift and update this test.
		e2e.Failf("TODO: job name %q indicates RHCOS10 or mixed OS which cannot be checked for HyperShift clusters now", jobName)
		return
	}

	e2eskipper.Skip("Cannot check RHCOS for HyperShift clusters")
}

func validateStandaloneNodeOS(oc *exutil.CLI, jobName string) {
	clusterVersion, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error getting ClusterVersion version singleton")

	// If no explicit stream assume rhel-10
	targetStream := "rhel-10"
	if strings.Contains(jobName, "rhcos10") {
		targetStream = "rhel-10"
	} else if strings.Contains(jobName, "rhcos9") {
		targetStream = "rhel-9"
	} else if strings.Contains(jobName, "upgrade") {
		installVersion := getInstallVersion(clusterVersion)
		// In standalone a cluster installed using OCP 4.x preserves RHEL 9 even after upgrading to 5
		if installVersion.Major() < 5 {
			targetStream = "rhel-9"
		}
	}

	mcfgClient := oc.MachineConfigurationClient()

	// Fetch the OSImageStream CR to get the cluster default
	osImageStream, err := mcfgClient.MachineconfigurationV1().OSImageStreams().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		clusterSemver, err := utilversion.ParseGeneric(clusterVersion.Status.Desired.Version)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error parsing ClusterVersion desired version %v", err)
		if clusterSemver.LessThan(utilversion.MustParseSemantic("4.23.0")) {
			// Pre-OS Image Streams GA. OS was always RHEL 9
			validatePreOSImageStreamsNodeOS(oc.AdminKubeClient())
			// Return now, the rest of the test is based on the presence of OSImageStream
			return
		}
	}

	o.Expect(err).NotTo(o.HaveOccurred(), "Error getting OSImageStream singleton in an OS Image Stream enabled cluster")

	mcpsStreams := fetchMCPStreams(mcfgClient, osImageStream)
	if !strings.Contains(jobName, "rhcos9-10") {
		// Regular non-mixed cluster
		for mcp, stream := range mcpsStreams {
			o.Expect(stream).To(o.Equal(targetStream), "MCP %s uses %s as stream but was expecting %s", mcp, stream, targetStream)
		}
	} else {
		// Mixed cluster: Simple check that makes sure that each stream is used at least once
		var hasRHCOS9 bool
		var hasRHCOS10 bool
		for _, stream := range mcpsStreams {
			if stream == "rhel-9" {
				hasRHCOS9 = true
			} else if stream == "rhel-10" {
				hasRHCOS10 = true
			}
		}
		o.Expect(hasRHCOS9).To(o.BeTrue(), "The cluster is a RHEL 9 and RHEL 10 mixed cluster but it doesn't use RHEL 9")
		o.Expect(hasRHCOS10).To(o.BeTrue(), "The cluster is a RHEL 9 and RHEL 10 mixed cluster but it doesn't use RHEL 10")
	}
}

func getInstallVersion(clusterVersion *v1.ClusterVersion) *utilversion.Version {
	completed := make([]v1.UpdateHistory, 0, len(clusterVersion.Status.History))
	for _, entry := range clusterVersion.Status.History {
		if entry.CompletionTime != nil && entry.State == v1.CompletedUpdate {
			completed = append(completed, entry)
		}
	}
	if len(completed) == 0 {
		e2e.Failf("Unable to determine the cluster install version as no version is flagged as complete")
	}

	slices.SortFunc(completed, func(a, b v1.UpdateHistory) int {
		return a.CompletionTime.Time.Compare(b.CompletionTime.Time)
	})

	v, err := utilversion.ParseGeneric(completed[0].Version)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error parsing ClusterVersion version %v", err)
	return v
}
