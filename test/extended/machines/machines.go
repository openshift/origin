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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	operatorWait = 1 * time.Minute
)

var _ = g.Describe("[sig-cluster-lifecycle][Feature:Machines] Managed cluster should", func() {
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

func skipUnlessMachineAPIOperator(c coreclient.NamespaceInterface) {
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		_, err := c.Get("openshift-machine-api", metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if errors.IsNotFound(err) {
			e2e.Skipf("The cluster machines are not managed by machine api operator")
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
