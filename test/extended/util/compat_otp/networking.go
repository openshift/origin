package compat_otp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func CheckNetworkType(oc *exutil.CLI) string {
	output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.defaultNetwork.type}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.ToLower(output)
}

// check until CNO operator status reports True, False, False for Available, Progressing, Degraded status,
func CheckNetworkOperatorStatus(oc *exutil.CLI) error {
	err := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "network").Output()
		if err != nil {
			e2e.Logf("Fail to get clusteroperator network, error:%s. Trying again", err)
			return false, nil
		}
		matched, _ := regexp.MatchString("True.*False.*False", output)
		if matched {
			return true, nil
		}
		e2e.Logf("Network operator state is:%s", output)
		return false, nil
	})
	return err
}

// GetIPVersionStackType gets IP-version Stack type of the cluster
func GetIPVersionStackType(oc *exutil.CLI) (ipvStackType string) {
	svcNetwork, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("network.operator", "cluster", "-o=jsonpath={.spec.serviceNetwork}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Count(svcNetwork, ":") >= 2 && strings.Count(svcNetwork, ".") >= 2 {
		ipvStackType = "dualstack"
	} else if strings.Count(svcNetwork, ":") >= 2 {
		ipvStackType = "ipv6single"
	} else if strings.Count(svcNetwork, ".") >= 2 {
		ipvStackType = "ipv4single"
	}
	e2e.Logf("The test cluster IP-version Stack type is :\"%s\".", ipvStackType)
	return ipvStackType
}
func AssertOrCheckMCP(oc *exutil.CLI, mcp string, interval, timeout time.Duration, skip bool) error {
	var machineCount string
	err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, false, func(ctx context.Context) (bool, error) {
		machineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcp, "-o=jsonpath={.status.machineCount}{\" \"}{.status.readyMachineCount}{\" \"}{.status.unavailableMachineCount}{\" \"}{.status.degradedMachineCount}").Output()
		indexCount := strings.Fields(machineCount)
		if strings.Compare(indexCount[0], indexCount[1]) == 0 && strings.Compare(indexCount[2], "0") == 0 && strings.Compare(indexCount[3], "0") == 0 {
			return true, nil
		}
		return false, nil
	})
	e2e.Logf("MachineCount:ReadyMachineCountunavailableMachineCountdegradedMachineCount: %v", machineCount)
	if err != nil {
		if skip {
			g.Skip(fmt.Sprintf("the mcp %v is not correct status, so skip it", machineCount))
		}
		return fmt.Errorf("case: %v\nerror: %s", g.CurrentSpecReport().FullText(), fmt.Sprintf("macineconfigpool %v update failed", mcp))
	}
	return nil
}
