package kernel

import (
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

func runPiStressFifo(oc *exutil.CLI) {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1"}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "error occured running pi_stress with the fifo algorithm")
}

func runPiStressRR(oc *exutil.CLI) {
	args := []string{rtPodName, "--", "pi_stress", "--duration=600", "--groups=1", "--rr"}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "error occured running pi_stress with the round robin algorithm")
}

func runDeadlineTest(oc *exutil.CLI) {
	args := []string{rtPodName, "--", "deadline_test"}
	_, err := oc.SetNamespace(rtNamespace).Run("exec").Args(args...).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "error occured running deadline_test")
}
