package networking

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/liornoy/node-comm-lib/pkg/client"
	"github.com/liornoy/node-comm-lib/pkg/commatrix"
	"github.com/liornoy/node-comm-lib/pkg/consts"
	"github.com/liornoy/node-comm-lib/pkg/endpointslices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commutil "github.com/openshift/origin/pkg/commatrix"
	exutil "github.com/openshift/origin/test/extended/util"
)

var cs *client.ClientSet

var _ = g.Context("[sig-network][Feature:commatrix][Serial]", func() {
	oc := exutil.NewCLIWithoutNamespace("baremetal").AsAdmin()
	hostServicesWithRandomPorts := []string{"rpc", "crio", "livenessprobe", "gcp-cloud-contr", "ovnkube"}
	ignorePorts := map[string]bool{"80": false, "443": false}

	g.AfterEach(func() {
		g.By("fetching all custom EndpointSlices and deleting them")
		customeSlices, err := cs.EndpointSlices("default").List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		for _, slice := range customeSlices.Items {
			if !strings.Contains(slice.Name, "commatrix-test") {
				continue
			}
			err := cs.EndpointSlices("default").Delete(context.TODO(), slice.Name, metav1.DeleteOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())
		}
	})

	g.Context("create a comm matrix", func() {
		g.It("should cover all ports that the nodes are actually listening on", func() {
			artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")
			err := os.MkdirAll(artifactsDir, 0755)
			o.Expect(err).NotTo(o.HaveOccurred())

			cs, err = client.New("")
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("generating the ss comm matrix")
			expectedComMat, err := commutil.GenerateSSComMatrix(oc, artifactsDir)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("generating custom EndpointSlices for host services")
			nodes, err := cs.Nodes().List(context.TODO(), metav1.ListOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			nodeNameToNodeRoles := commatrix.GetNodesRoles(nodes)
			nodeRoleToNodeName := commutil.ReverseMap(nodeNameToNodeRoles)

			staticHostServices, err := commutil.CustomHostServicesDefinion()
			o.Expect(err).ToNot(o.HaveOccurred())

			randomPortHostServices := make([]commatrix.ComDetails, 0)
			for _, cd := range expectedComMat.Matrix {
				for _, hostServiceName := range hostServicesWithRandomPorts {
					if strings.Contains(cd.ServiceName, hostServiceName) {
						randomPortHostServices = append(randomPortHostServices, cd)
						continue
					}
				}
			}

			comDetails := append(staticHostServices, randomPortHostServices...)

			for _, cd := range comDetails {
				endpointSlice, err := commutil.ComDetailsToEPSlice(&cd, nodeRoleToNodeName)
				o.Expect(err).ToNot(o.HaveOccurred())

				_, err = cs.EndpointSlices("default").Create(context.TODO(), &endpointSlice, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					o.Expect(err).ToNot(o.HaveOccurred())
				}
			}

			g.By("generating the communication matrix from the cluster's endpointslices")
			epSliceQuery, err := endpointslices.NewQuery(cs)
			o.Expect(err).ToNot(o.HaveOccurred())

			ingressSlice := epSliceQuery.
				WithHostNetwork().
				WithLabels(map[string]string{consts.IngressLabel: ""}).
				WithServiceType(corev1.ServiceTypeNodePort).
				WithServiceType(corev1.ServiceTypeLoadBalancer).
				Query()

			endpointSliceMat, err := commatrix.CreateComMatrix(cs, ingressSlice)
			o.Expect(err).ToNot(o.HaveOccurred())

			diff1 := endpointSliceMat.Diff(expectedComMat, ignorePorts)
			diff2 := expectedComMat.Diff(endpointSliceMat, ignorePorts)

			g.By("Writing test artifacts")
			err = os.WriteFile(filepath.Join(artifactsDir, "endpointslice-commatrix"), []byte(endpointSliceMat.String()), 0644)
			o.Expect(err).ToNot(o.HaveOccurred())

			err = os.WriteFile(filepath.Join(artifactsDir, "ss-commatrix"), []byte(expectedComMat.String()), 0644)
			o.Expect(err).ToNot(o.HaveOccurred())

			err = os.WriteFile(filepath.Join(artifactsDir, "endpointslice-diff-ss"), []byte(diff1.String()), 0644)
			o.Expect(err).ToNot(o.HaveOccurred())

			err = os.WriteFile(filepath.Join(artifactsDir, "ss-diff-endpointslice"), []byte(diff2.String()), 0644)
			o.Expect(err).ToNot(o.HaveOccurred())

			if _, ok := os.LookupEnv("APPLY_FIREWALL"); ok {
				g.By("Applying commatrix firewall rules")

				err := createAndApplyNftablesRules(oc, &endpointSliceMat, "master")
				o.Expect(err).ToNot(o.HaveOccurred())

				err = createAndApplyNftablesRules(oc, &endpointSliceMat, "worker")
				o.Expect(err).ToNot(o.HaveOccurred())
				return
			}

			o.Expect(diff1.Matrix).To(o.BeEmpty(), "test failed, the following ports are found in the endpointSlice matrix but not in the ss matrix:")
			o.Expect(diff2.Matrix).To(o.BeEmpty(), "test failed, the following ports are found in the ss matrix but not in the endpointSlice matrix:")
		})
	})
})

func createAndApplyNftablesRules(oc *exutil.CLI, m *commatrix.ComMatrix, role string) error {
	ports := make([]commatrix.ComDetails, 0)

	for _, cd := range m.Matrix {
		if cd.NodeRole == role {
			ports = append(ports, cd)
		}
	}

	if len(ports) == 0 {
		return nil
	}

	nftCommand := createNftablesCommand(ports)

	nodes, err := cs.Nodes().List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())

	nodeNameToNodeRole := commatrix.GetNodesRoles(nodes)

	for _, n := range nodes.Items {
		if !strings.Contains(nodeNameToNodeRole[n.Name], role) {
			continue

		}
		_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, n.Name, "openshift-cluster-node-tuning-operator", nftCommand...)
		if err != nil {
			return err
		}

	}

	return nil
}

func createNftablesCommand(cds []commatrix.ComDetails) []string {
	if len(cds) == 0 {
		return nil
	}

	res := make([]string, 0)
	tcpPorts := []string{}
	udpPorts := []string{}

	for _, cd := range cds {
		if cd.Protocol == "TCP" {
			tcpPorts = append(tcpPorts, cd.Port)
		}
		if cd.Protocol == "UDP" {
			udpPorts = append(udpPorts, cd.Port)
		}
	}

	res = append(res, strings.Split("nft add table ip commatrix_filter &&", " ")...)
	res = append(res, strings.Split("nft add chain ip commatrix_filter input { type filter hook input priority 0\\; policy drop\\; } &&", " ")...)
	res = append(res, strings.Split("nft add rule ip commatrix_filter input iifname \"lo\" accept\\; &&", " ")...)
	res = append(res, strings.Split("nft add rule ip commatrix_filter input tcp dport 22 accept\\; &&", " ")...)

	if len(tcpPorts) > 0 {
		tcpPortsList := strings.Join(tcpPorts, ", ")
		tcpPortsRule := fmt.Sprintf("nft add rule ip commatrix_filter input tcp dport {  %s } accept", tcpPortsList)
		res = append(res, strings.Split(tcpPortsRule, " ")...)

	}

	if len(udpPorts) > 0 {
		udpPortsList := strings.Join(udpPorts, ", ")
		udpPortsRule := fmt.Sprintf("&& nft add rule ip commatrix_filter input udp dport { %s } accept", udpPortsList)
		res = append(res, strings.Split(udpPortsRule, " ")...)
	}

	return res
}
