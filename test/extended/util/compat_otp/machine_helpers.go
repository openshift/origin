package compat_otp

import (
	"context"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

// We are no longer updating this file because we deprecated it,
// the new file is test/extended/util/clusterinfra/machine_helpers.go
// This file is not deleted because there are some old dependencies
const (
	MachineAPINamespace = "openshift-machine-api"
	//MapiMachineset means the fullname of mapi machineset
	MapiMachineset = "machinesets.machine.openshift.io"
	//MapiMachine means the fullname of mapi machine
	MapiMachine = "machines.machine.openshift.io"
)

func ExtendedCheckPlatform(ctx context.Context, oc *exutil.CLI) string {
	if CheckAKSCluster(ctx, oc) {
		return "azure"
	}
	return CheckPlatform(oc)
}

// CheckPlatform check the cluster's platform
func CheckPlatform(oc *exutil.CLI) string {
	output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}").Output()
	return strings.ToLower(output)
}

// SkipForSNOCluster skip for SNO cluster
func SkipForSNOCluster(oc *exutil.CLI) {
	//Only 1 master, 1 worker node and with the same hostname.
	masterNodes, _ := GetClusterNodesBy(oc, "master")
	workerNodes, _ := GetClusterNodesBy(oc, "worker")
	if len(masterNodes) == 1 && len(workerNodes) == 1 && masterNodes[0] == workerNodes[0] {
		g.Skip("Skip for SNO cluster.")
	}
}

func CompareMachineCreationTime(oc *exutil.CLI, ms1 string, ms2 string) bool {
	p10CreationTime, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+ms1, "-o=jsonpath={.items[0].metadata.creationTimestamp}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	p20CreationTime, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-n", "openshift-machine-api", "-l", "machine.openshift.io/cluster-api-machineset="+ms2, "-o=jsonpath={.items[0].metadata.creationTimestamp}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	t1, _ := time.Parse(time.RFC3339, p10CreationTime)
	t2, _ := time.Parse(time.RFC3339, p20CreationTime)
	return !(t1.Before(t2) || t1.Equal(t2))
}
