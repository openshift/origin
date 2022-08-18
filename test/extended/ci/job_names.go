package ci

import (
	"context"
	"fmt"
	"os"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

// trimJobName returns the part of a job name suitable for checking if "ovn" or "sdn" is present.
func trimJobName(jobName string) string {
	// If it's an e2e job, we're only interested in the parts that come after e2e, e.g.
	// a presubmit on pull-ci-openshift-ovn-kubernetes-master-e2e-hypershift, we don't want
	// to look at the repo name which contains the -ovn- substring!
	parts := strings.Split(jobName, "e2e-")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return jobName
}

var _ = g.Describe("[sig-ci] [Early] prow job name", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("job-names")

	g.It("should match platform type [apigroup:config.openshift.io]", func() {
		jobName := os.Getenv("JOB_NAME")
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		} else if strings.Contains(jobName, "agnostic") {
			e2eskipper.Skipf("JOB_NAME contains agnostic, not expecting platform in name")
		}

		originalJobName := jobName
		jobName = trimJobName(jobName)

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

	g.It("should match network type [apigroup:config.openshift.io]", func() {
		jobName := os.Getenv("JOB_NAME")
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		}

		jobType := os.Getenv("JOB_TYPE")
		if jobType == "" {
			e2eskipper.Skipf("JOB_TYPE env var not set, skipping")
		}

		originalJobName := jobName
		jobName = trimJobName(jobName)

		if !strings.Contains(jobName, "sdn") && !strings.Contains(jobName, "ovn") {
			if jobType == "periodic" {
				e2e.Fail(fmt.Sprintf("%q job name %q does not have network type in name (expected one of: 'sdn', 'ovn')", jobType, originalJobName))
				return
			}
		}
		network, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		switch network.Status.NetworkType {
		case "OpenShiftSDN":
			if !strings.Contains(jobName, "sdn") {
				if jobType == "periodic" {
					e2e.Fail(fmt.Sprintf("job name %q does not have network type in name (expected `sdn`)", jobName))
				} else {
					result.Flakef("job name %q does not have network type in name (expected `sdn`)", jobName)
				}
			}
		case "OVNKubernetes":
			if !strings.Contains(jobName, "ovn") {
				if jobType == "periodic" {
					e2e.Fail(fmt.Sprintf("job name %q does not have network type in name (expected `ovn`)", jobName))
				} else {
					result.Flakef("job name %q does not have network type in name (expected `ovn`)", jobName)
				}
			}
		}
	})
})
