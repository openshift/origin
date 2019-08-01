package operators

import (
	"fmt"
	"strconv"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Operators] Cluster autoscaler operator should", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLI("cao", exutil.KubeConfigPath())
		clusterautoscaler = exutil.FixturePath("testdata", "machines", "clusterautoscaler.yaml")
		machineautoscaler = exutil.FixturePath("testdata", "machines", "machineautoscaler.yaml")
	)

	g.It("listens and deploys cluster-autoscaler based on ClusterAutoscaler resource", func() {
		g.By("create clusterautoscaler")
		err := oc.AsAdmin().Run("create").Args("-f", clusterautoscaler).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := oc.AsAdmin().Run("get").Args("clusterautoscaler", "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("default"))
		output, err = oc.AsAdmin().Run("get").Args("pod", "-n", "openshift-machine-api").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("cluster-autoscaler-default"))
	})

	g.It("listens and annotations machineSets based on MachineAutoscaler resource", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		skipUnlessMachineAPIOperator(c.CoreV1().Namespaces())

		g.By("validating machineset invariants")
		machinesets, err := listWorkerMachineSets(dc)
		if err != nil {
			e2e.Failf("unable to fetch machinesets: %v", err)
		}
		if len(machinesets) == 0 {
			e2e.Failf("cluster should have machinesets")
		}

		g.By("Creating machineautoscalers")
		for _, machineset := range machinesets {
			replicas, error := strconv.Atoi(machineset.Get("spec.replicas").String())
			if error != nil {
				fmt.Println("unable to fetch replicas")
			}
			if replicas > 0 {
				machinesetName := machineset.Get("metadata.name").String()

				configFile, err := oc.AsAdmin().Run("process").Args("-f", machineautoscaler, "-p", fmt.Sprintf("NAME=%s", machinesetName), "-n", "openshift-machine-api").OutputToFile("config.json")
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.AsAdmin().Run("create").Args("-f", configFile, "-n", "openshift-machine-api").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				output, err := oc.AsAdmin().Run("get").Args("machineautoscaler", "-n", "openshift-machine-api").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.ContainSubstring(machinesetName))
				output, err = oc.AsAdmin().Run("get").Args("machineset", "-n", "openshift-machine-api", machinesetName, "-o=jsonpath={.metadata.annotations}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.ContainSubstring("cluster-api-autoscaler-node-group-max-size"))
			}
		}
	})

})
