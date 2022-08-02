package ci

import (
	"context"
	"os"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-ci] [Early] prow job name", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("job-names")

	g.It("should match platform type", func() {
		jobName := os.Getenv("JOB_NAME")
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
			result.Flakef("job name %q does not contain platform type in name (%s)", jobName, platform)
		}

	})

	g.It("should match network type", func() {
		jobName := os.Getenv("JOB_NAME")
		if jobName == "" {
			e2eskipper.Skipf("JOB_NAME env var not set, skipping")
		}

		network, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		switch network.Status.NetworkType {
		case "OpenShiftSDN":
			if !strings.Contains(jobName, "sdn") {
				result.Flakef("job name %q does not have network type in name (expected `sdn`)", jobName)
			}
		case "OVNKubernetes":
			if !strings.Contains(jobName, "ovn") {
				result.Flakef("job name %q does not have network type in name (expected `ovn`)", jobName)
			}
		}
	})
})
