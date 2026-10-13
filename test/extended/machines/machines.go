package operators

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	v1 "github.com/openshift/api/config/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	operatorWait               = 1 * time.Minute
	masterMachineLabelSelector = "machine.openshift.io/cluster-api-machine-role=master"
)

var _ = g.Describe("[sig-cluster-lifecycle][Feature:Machines] Managed cluster should", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("control-plane-machines").AsAdmin()

	var cfg *rest.Config
	var c *kubernetes.Clientset
	var dc *dynamic.DynamicClient
	var err error

	g.BeforeEach(func() {
		cfg, err = e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err = e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err = dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("have machine resources [apigroup:machine.openshift.io]", func() {
		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("ensuring every node is linked to a machine api resource")
		allNodes, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		nodeNames := sets.NewString()
		for i := range allNodes.Items {
			node := &allNodes.Items[i]

			// If the node does not have the label, it may be a non machine node.
			if _, ok := node.Annotations["machine.openshift.io/machine"]; ok {
				nodeNames.Insert(node.ObjectMeta.Name)
			}
		}

		if len(nodeNames) == 0 {
			e2e.Failf("Missing nodes on the cluster")
		}

		machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})
		var lastMachines []objx.Map
		if err := wait.PollImmediate(3*time.Second, operatorWait, func() (bool, error) {
			obj, err := machineClient.List(context.Background(), metav1.ListOptions{})
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

	g.It("[sig-scheduling][Early] control plane machine set operator should not cause an early rollout", func() {
		machineClientSet, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).ToNot(o.HaveOccurred())

		pattern := `^([a-zA-Z0-9]+-)+master(?:-[a-zA-Z0-9]+)?-\d+$`

		g.By("checking for the openshift machine api operator")
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("ensuring every node is linked to a machine api resource")
		allControlPlaneMachines, err := machineClientSet.MachineV1beta1().Machines("openshift-machine-api").List(context.Background(), metav1.ListOptions{
			LabelSelector: masterMachineLabelSelector,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		regex, err := regexp.Compile(pattern)
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, m := range allControlPlaneMachines.Items {
			matched := regex.MatchString(m.Name)
			o.Expect(matched).To(o.BeTrue(), fmt.Sprintf("unexpected name of a control machine occured during early stages: %s", m.Name))
		}
	})

	g.It("[sig-scheduling][Early] control plane machine set operator should not have any events", func() {
		ctx := context.Background()

		infrastructure, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		// if the Cluster CR is not found it most likely means this is running on a UPI installation and does not have CPMSO support
		if errors.IsNotFound(err) {
			e2eskipper.Skipf("No Cluster object found, this frequently means there is no Control Plane Machine Set Operator, skipping test")
		}
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(infrastructure).ToNot(o.BeNil())

		// We want to skip this test on single-node clusters
		// as the control plane machine set does not get generated.
		// No other topology should match this condition.
		if infrastructure.Status.ControlPlaneTopology == v1.SingleReplicaTopologyMode {
			g.Skip("Skipping test due to a cluster being a single node cluster")
		}

		platform := infrastructure.Status.PlatformStatus.Type

		switch platform {
		case v1.AWSPlatformType,
			v1.AzurePlatformType,
			v1.GCPPlatformType,
			v1.NutanixPlatformType,
			v1.OpenStackPlatformType:
			// Continue with the test
		default:
			g.Skip(fmt.Sprintf("Skipping test on platform: %s", platform))
		}

		machineClientSet, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("getting the control plane machine set")
		cpmsClient := machineClientSet.MachineV1().ControlPlaneMachineSets("openshift-machine-api")
		cpms, err := cpmsClient.Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("getting the events from the control plane machine set")
		eventsClient := oc.KubeClient().CoreV1().Events("openshift-machine-api")
		fieldSelector := fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s", cpms.Kind, cpms.Name)
		cpmsEvents, err := eventsClient.List(ctx, metav1.ListOptions{
			FieldSelector: fieldSelector,
		})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("the control plane machine set should not have any events")
		o.Expect(cpmsEvents.Items).Should(o.BeEmpty())
	})
})

// skipUnlessMachineAPI is used to deterine if the Machine API is installed and running in a cluster.
// It is expected to skip the test if it determines that the Machine API is not installed/running.
// Use this early in a test that relies on Machine API functionality.
//
// It checks to see if the machine custom resource is installed in the cluster.
// If machines are not installed, or there are no machines in the cluster, it skips the test case.
// It then checks to see if the `openshift-machine-api` namespace is installed.
// If the namespace is not present it skips the test case.
func skipUnlessMachineAPIOperator(dc dynamic.Interface, c coreclient.NamespaceInterface) {
	machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})

	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		// Listing the resource will return an IsNotFound error when the CRD has not been installed.
		// Otherwise it would return an empty list if no Machines are in use, which should not be
		// possible if the MachineAPI operator is in use.
		machines, err := machineClient.List(context.Background(), metav1.ListOptions{})
		// If no error was returned and the list of Machines is populated, this cluster is using MachineAPI
		if err == nil {
			// If the Machine CRD exists but there are no Machine objects in the cluster we should
			// skip the test because any cluster that is using MachineAPI from the install will have
			// Machines for the control plane nodes at the minimum.
			if len(machines.Items) == 0 {
				e2eskipper.Skipf("The cluster supports the Machine CRD but has no Machines available")
			}

			return true, nil
		}

		// Not found error on the Machine CRD, cluster is not using MachineAPI
		if errors.IsNotFound(err) {
			e2eskipper.Skipf("The cluster does not support machine instances")
		}
		e2e.Logf("Unable to check for machine api operator: %v", err)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		// Check if the openshift-machine-api namespace is present, if not then this
		// cluster is not using MachineAPI.
		_, err := c.Get(context.Background(), "openshift-machine-api", metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if errors.IsNotFound(err) {
			e2eskipper.Skipf("The cluster machines are not managed by machine api operator")
		}
		e2e.Logf("Unable to check for machine api operator: %v", err)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func objects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, objx.Map(msi))
			}
		}
	}
	return values
}

func nodeNameFromNodeRef(item objx.Map) string {
	return item.Get("status.nodeRef.name").String()
}
