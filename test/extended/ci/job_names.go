package ci

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	mcv1client "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		jobIsRHCOS10 := strings.Contains(jobName, "rhcos10")

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			if jobIsRHCOS10 {
				// TODO(muller): Assume we do not have RHCOS10 microshift jobs now. If someone adds a RHCOS10 job, this failure
				// should force them to figure out how to detect RHCOS10 in microshift and update this test.
				e2e.Failf("TODO: job name %q indicates RHCOS10 which cannot be checked for MicroShift clusters now", jobName)
				return
			}

			e2eskipper.Skip("Cannot check RHCOS for MicroShift clusters")
		}

		isHyperShift, err := exutil.IsHypershift(context.TODO(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			if jobIsRHCOS10 {
				// TODO(muller): Assume we do not have RHCOS10 hypershift jobs now. If someone adds a RHCOS10 job, this failure
				// should force them to figure out how to detect RHCOS10 in hypershift and update this test.
				e2e.Failf("TODO: job name %q indicates RHCOS10 which cannot be checked for HyperShift clusters now", jobName)
				return
			}

			e2eskipper.Skip("Cannot check RHCOS for HyperShift clusters")
		}

		clusterIsRHCOS10 := isRHCOS10(oc.MachineConfigurationClient())

		if clusterIsRHCOS10 && !jobIsRHCOS10 {
			e2e.Failf("cluster runs RHCOS10 so job name %q must contain 'rhcos10'", jobName)
		}
		if !clusterIsRHCOS10 && jobIsRHCOS10 {
			e2e.Failf("cluster does not run RHCOS10 so job name %q must not contain 'rhcos10')", jobName)
		}
	})
})

// isRHCOS10 checks whether the cluster is running RHEL 10 by examining the worker
// MCP's OSImageStream setting, falling back to the cluster-wide default stream
// from the OSImageStream singleton if the MCP does not specify one.
func isRHCOS10(machineConfigClient mcv1client.Interface) bool {
	mcp, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), "worker", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error getting worker MCP")

	if mcp.Spec.OSImageStream.Name != "" {
		return mcp.Spec.OSImageStream.Name == "rhel-10"
	}

	osImageStream, err := machineConfigClient.MachineconfigurationV1alpha1().OSImageStreams().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if kapierrs.IsNotFound(err) {
		// OSImageStream CRD not present (feature gate not enabled), assume not RHEL 10
		return false
	}
	o.Expect(err).NotTo(o.HaveOccurred(), "Error getting OSImageStream singleton")

	return osImageStream.Status.DefaultStream == "rhel-10"
}
