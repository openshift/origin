package nvidia

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"

	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"

	exutil "github.com/openshift/origin/test/extended/util"
)

func NewDriverToolkitProber(oc *exutil.CLI, clientset clientset.Interface) *DriverToolkit {
	return &DriverToolkit{oc: oc, clientset: clientset}
}

type DriverToolkit struct {
	oc        *exutil.CLI
	clientset clientset.Interface
}

type OSReleaseInfo struct {
	RHCOSVersion string
}

func (dtk DriverToolkit) GetOSReleaseInfo(t testing.TB, node *corev1.Node) (OSReleaseInfo, error) {
	args := []string{
		fmt.Sprintf("node/%s", node.Name), "--quiet", "--", "cat", "/host/etc/os-release",
	}
	g.By(fmt.Sprintf("calling oc debug %v", args))

	// TODO: can we get it using the API, if the information is stored in an object
	// it can be flaky, maybe add a retry
	out, err := dtk.oc.AsAdmin().Run("debug").Args(args...).Output()
	if err != nil {
		return OSReleaseInfo{}, fmt.Errorf("failed to debug into %s - %w", node.Name, err)
	}
	t.Logf("output of oc debug:\n%s\n", out)

	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		after, found := strings.CutPrefix(strings.TrimSpace(sc.Text()), "OSTREE_VERSION=")
		if !found {
			continue
		}
		return OSReleaseInfo{
			RHCOSVersion: strings.Trim(strings.TrimSpace(after), "'"),
		}, nil
	}

	return OSReleaseInfo{}, fmt.Errorf("OSTREE_VERSION not found in output")
}
