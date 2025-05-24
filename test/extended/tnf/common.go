package tnf

import (
	"context"

	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getInfraStatus(oc *exutil.CLI) v1.InfrastructureStatus {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(),
		"cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return infra.Status
}

func runOnNodeNS(oc *exutil.CLI, nodeName, namespace, command string) (string, string, error) {
	return oc.AsAdmin().Run("debug").Args("-n", namespace, "node/"+nodeName, "--", "chroot", "/host", "/bin/bash", "-c", command).Outputs()
}
