package operators

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	operatorWait = 1 * time.Minute
)

var _ = g.Describe("[Feature:Machines] Managed cluster should", func() {
	defer g.GinkgoRecover()

	var (
		oc         = exutil.NewCLI("machine", exutil.KubeConfigPath())
		machine    = exutil.FixturePath("testdata", "machines", "machine-example.yaml")
		machineset = exutil.FixturePath("testdata", "machines", "machineset-example.yaml")
	)

	g.It("create machine/machineset", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("getting exist machines parameters")
		allMachinesets, err := listMachinesets(dc)
		if err != nil {
			e2e.Failf("unable to fetch machinesets: %v", err)
		}

		if len(allMachinesets) == 0 {
			e2e.Failf("cluster should have machinesets")
		}

		for _, m := range allMachinesets {
			profileId := m.Get("spec.template.spec.providerSpec.value.iamInstanceProfile.id").String()
			clusterId := profileId[0 : len(profileId)-15]
			ami := m.Get("spec.template.spec.providerSpec.value.ami.id").String()
			region := m.Get("spec.template.spec.providerSpec.value.placement.region").String()

			g.By("create a machine")
			configFile, err := oc.AsAdmin().Run("process").Args("-f", machine, "-p", fmt.Sprintf("CLUSTERID=%s", clusterId), "-p", fmt.Sprintf("AMI=%s", ami), "-p", fmt.Sprintf("REGION=%s", region), "-n", "openshift-machine-api").OutputToFile("config.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().Run("create").Args("-f", configFile, "-n", "openshift-machine-api").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			output, err := oc.AsAdmin().Run("get").Args("machine", "-n", "openshift-machine-api").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("mymachine"))

			g.By("create a machineset")
			configFile, err = oc.AsAdmin().Run("process").Args("-f", machineset, "-p", fmt.Sprintf("CLUSTERID=%s", clusterId), "-p", fmt.Sprintf("AMI=%s", ami), "-p", fmt.Sprintf("REGION=%s", region), "-n", "openshift-machine-api").OutputToFile("config.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().Run("create").Args("-f", configFile, "-n", "openshift-machine-api").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			output, err = oc.AsAdmin().Run("get").Args("machineset", "-n", "openshift-machine-api").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("mymachineset"))
			break
		}
	})
})

var _ = g.Describe("[Feature:Machines][Smoke] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have machine resources", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(c.CoreV1().Namespaces())

		g.By("ensuring every node is linked to a machine api resource")
		allNodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		nodeNames := sets.NewString()
		for i := range allNodes.Items {
			node := &allNodes.Items[i]
			nodeNames.Insert(node.ObjectMeta.Name)
		}

		if len(nodeNames) == 0 {
			e2e.Failf("Missing nodes on the cluster")
		}

		machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})
		var lastMachines []objx.Map
		if err := wait.PollImmediate(3*time.Second, operatorWait, func() (bool, error) {
			obj, err := machineClient.List(metav1.ListOptions{})
			if err != nil {
				e2e.Logf("Unable to check for machines: %v", err)
				return false, nil
			}
			machines := objx.Map(obj.UnstructuredContent())
			items := objects(machines.Get("items"))
			lastMachines = items
			if len(items) == 0 {
				e2e.Logf("No machine objects found")
				return true, nil
			}
			for _, machine := range items {
				nodeName := nodeNameFromNodeRef(machine)
				nodeNames.Delete(nodeName)
			}

			if len(nodeNames) > 0 {
				e2e.Logf("Machine resources missing for nodes: %s", strings.Join(nodeNames.List(), ", "))
				return false, nil
			}
			return true, nil
		}); err != nil {
			buf := &bytes.Buffer{}
			w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
			fmt.Fprintf(w, "NAMESPACE\tNAME\tNODE NAME\n")
			for _, machine := range lastMachines {
				ns := machine.Get("metadata.namespace").String()
				name := machine.Get("metadata.name").String()
				nodeName := nodeNameFromNodeRef(machine)
				fmt.Fprintf(w, "%s\t%s\t%s\n",
					ns,
					name,
					nodeName,
				)
			}
			w.Flush()
			e2e.Logf("Machines:\n%s", buf.String())
			e2e.Failf("Machine resources missing for nodes: %s", strings.Join(nodeNames.List(), ", "))
		}
	})
})
