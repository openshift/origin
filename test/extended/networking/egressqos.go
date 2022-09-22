package networking

import (
	"fmt"
	"os"
	"strconv"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-network][Feature:EgressQoS]", func() {
	// OpenShiftSDN does not have EgressQoS
	InOVNKubernetesContext(func() {
		const (
			yamlFile = "egressqos.yaml"
			// tcpdump args: http://darenmatthews.com/blog/?p=1199 , https://www.tucny.com/home/dscp-tos
			tcpdumpIPv4 = "dst %s and icmp and (ip and (ip[1] & 0xfc) >> 2 == %d)"
			tcpdumpIPv6 = "dst %s and icmp6 and (ip6 and (ip6[0:2] & 0xfc0) >> 6 == %d)"
		)

		// The tests create 2 pods on each of the available 2 nodes chosen - a regular ovn pod (ping pod)
		// and a host-networked pod (tcpdump pod). Then they open a tcpdump on the tcpdump pod that waits
		// for icmp packets to exit br-ex's physical nic towards a destination with a DSCP value.
		// While that happens, they ping that destination from the ping pods.
		// If everything works correctly the tcpdumps should exit without an error.
		// This struct contains the necessary config to run this setup.
		type targetConfig struct {
			ip         string  // which ip to ping
			tcpdumpPod *v1.Pod // a hostnetworked pod to tcpdump on
			intf       string  // which interface to listen on the host (br-ex's physical nic)
			tcpdumpTpl string  // the tcpdump filter template, should be passed a dst ip (string) and DSCP value (int)
			pingPod    *v1.Pod // the pod to ping the ip from, is on the same host as the tcpdumpPod
		}

		var (
			tmpDir           string
			node1, node2     *v1.Node
			target1, target2 *targetConfig
		)
		oc := exutil.NewCLIWithPodSecurityLevel("egressqos", admissionapi.LevelPrivileged)
		f := oc.KubeFramework()

		g.BeforeEach(func() {
			g.By("Creating a temp directory")
			var err error
			tmpDir, err = os.MkdirTemp("", "egressqos")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Choosing 2 different nodes")
			node1, node2, err = findAppropriateNodes(f, DIFFERENT_NODE)
			o.Expect(err).NotTo(o.HaveOccurred())

			target1, target2 = &targetConfig{}, &targetConfig{}

			tcpdumpImage, err := exutil.DetermineImageFromRelease(oc, "network-tools")
			o.Expect(err).NotTo(o.HaveOccurred())

			hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Setting the tcpdump pods")
			createSync := errgroup.Group{}
			createSync.Go(func() error {
				var err error
				target1.intf, err = findBridgePhysicalInterface(oc, node1.Name, "br-ex")
				if err != nil {
					return err
				}
				target1.tcpdumpPod, err = launchHostNetworkedPodForTCPDump(f, tcpdumpImage, node1.Name, "tcpdump-hostpod-")
				return err
			})

			createSync.Go(func() error {
				var err error
				target2.intf, err = findBridgePhysicalInterface(oc, node2.Name, "br-ex")
				if err != nil {
					return err
				}

				target2.tcpdumpPod, err = launchHostNetworkedPodForTCPDump(f, tcpdumpImage, node2.Name, "tcpdump-hostpod-")
				return err
			})

			err = createSync.Wait()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Setting the target ips")
			switch {
			case hasIPv4 && hasIPv6:
				target1.ip = "8.8.8.8"
				target1.tcpdumpTpl = tcpdumpIPv4
				target2.ip = "2001:4860:4860::8888"
				target2.tcpdumpTpl = tcpdumpIPv6
			case hasIPv4:
				target1.ip = "8.8.8.8"
				target1.tcpdumpTpl = tcpdumpIPv4
				target2.ip = "8.8.4.4"
				target2.tcpdumpTpl = tcpdumpIPv4
			default:
				target1.ip = "2001:4860:4860::8888"
				target1.tcpdumpTpl = tcpdumpIPv6
				target2.ip = "2001:4860:4860::8844"
				target2.tcpdumpTpl = tcpdumpIPv6
			}
		})

		g.AfterEach(func() {
			g.By("Removing the EgressQoS")
			yamlPath := tmpDir + "/" + yamlFile
			out, err := runOcWithRetry(oc.AsAdmin(), "delete", "-f", yamlPath, "--ignore-not-found")
			o.Expect(err).NotTo(o.HaveOccurred(), out)

			g.By("Removing the temp directory")
			os.RemoveAll(tmpDir)
		})

		// pingAndCheckDSCP runs tcpdump on the targets' tcpdump pods, using the right destination and DSCP value in
		// the tcpdump template. Then it pings the target ips from the ping pods. If one of the tcpdumps fail it returns
		// the error.
		pingAndCheckDSCP := func(oc *exutil.CLI, f *e2e.Framework, target1, target2 *targetConfig, dscp1, dscp2 int) error {
			tcpDumpSync := errgroup.Group{}
			pingSync := errgroup.Group{}

			checkDSCPOnPod := func(target *targetConfig, dscp int) error {
				_, err := oc.AsAdmin().Run("exec").Args(target.tcpdumpPod.Name, "-n", target.tcpdumpPod.Namespace, "--",
					"timeout", "10", "tcpdump", "-i", target.intf, "-c", "1", "-v", "--direction=out",
					fmt.Sprintf(target.tcpdumpTpl, target.ip, dscp)).Output()
				return err
			}

			pingFromSrcPod := func(target *targetConfig) error {
				_, err := oc.AsAdmin().Run("exec").Args(target.pingPod.Name, "-n", target.pingPod.Namespace, "--",
					"ping", "-c", "3", target.ip).Output()
				return err
			}

			tcpDumpSync.Go(func() error {
				return checkDSCPOnPod(target1, dscp1)
			})
			tcpDumpSync.Go(func() error {
				return checkDSCPOnPod(target2, dscp2)
			})

			pingSync.Go(func() error {
				return pingFromSrcPod(target1)
			})
			pingSync.Go(func() error {
				return pingFromSrcPod(target2)
			})

			pingErr := pingSync.Wait()
			err := tcpDumpSync.Wait()
			if err != nil {
				return fmt.Errorf("failed to detect ping with correct DSCP on pod, tcpdump err: %v, ping err: %v", err, pingErr)
			}

			return nil
		}

		g.It("EgressQoS should mark all egress traffic from the namespace", func() {
			dscpValue := 60
			yamlPath := tmpDir + "/" + yamlFile

			g.By("Creating an EgressQoS with a global DSCP rule")
			egressQoSConfig := fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-1) + `
`)

			err := os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "create", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the source pods to ping from")
			podSync := errgroup.Group{}
			podSync.Go(func() error {
				var err error
				target1.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node1.Name
				})
				return err
			})
			podSync.Go(func() error {
				var err error
				target2.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node2.Name
				})
				return err
			})

			err = podSync.Wait()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the correct DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-1, dscpValue-1)

			g.By("Updating the EgressQoS DSCP value")
			egressQoSConfig = fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-11) + `
`)

			err = os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "apply", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the updated DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-11, dscpValue-11)

			g.By("Deleting the EgressQoS")
			_, err = runOcWithRetry(oc.AsAdmin(), "delete", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is not marked with DSCP")
			pingAndCheckDSCP(oc, f, target1, target2, 0, 0)
		})

		g.It("EgressQoS should mark egress traffic based on destination CIDR", func() {
			dscpValue := 55
			yamlPath := tmpDir + "/" + yamlFile

			g.By("Creating an EgressQoS with rules using dstCIDR")
			egressQoSConfig := fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-1) + `
    dstCIDR: ` + target1.ip + "/32" + `
  - dscp: ` + strconv.Itoa(dscpValue-2) + `
    dstCIDR: ` + target2.ip + "/32" + `
`)

			err := os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "create", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the source pods to ping from")
			podSync := errgroup.Group{}
			podSync.Go(func() error {
				var err error
				target1.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node1.Name
				})
				return err
			})
			podSync.Go(func() error {
				var err error
				target2.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node2.Name
				})
				return err
			})

			err = podSync.Wait()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the correct DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-1, dscpValue-2)

			g.By("Updating the EgressQoS DSCP values")
			egressQoSConfig = fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-11) + `
    dstCIDR: ` + target1.ip + "/32" + `
  - dscp: ` + strconv.Itoa(dscpValue-12) + `
    dstCIDR: ` + target2.ip + "/32" + `
`)

			err = os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "apply", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the updated DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-11, dscpValue-12)

			g.By("Deleting the EgressQoS")
			_, err = runOcWithRetry(oc.AsAdmin(), "delete", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is not marked with DSCP")
			pingAndCheckDSCP(oc, f, target1, target2, 0, 0)
		})

		g.It("EgressQoS should mark egress traffic based on podSelectors", func() {
			dscpValue := 52
			yamlPath := tmpDir + "/" + yamlFile

			g.By("Creating an EgressQoS with rules using podSelectors")
			egressQoSConfig := fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-1) + `
    podSelector:
      matchLabels:
        name: Albus
  - dscp: ` + strconv.Itoa(dscpValue-2) + `
    podSelector:
      matchLabels:
        name: Severus
`)

			err := os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "create", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the source pods to ping from")
			podSync := errgroup.Group{}
			podSync.Go(func() error {
				var err error
				target1.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node1.Name
					p.Labels = map[string]string{"name": "Albus"}
				})
				return err
			})
			podSync.Go(func() error {
				var err error
				target2.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node2.Name
					p.Labels = map[string]string{"name": "Severus"}
				})
				return err
			})

			err = podSync.Wait()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the correct DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-1, dscpValue-2)

			g.By("Updating the EgressQoS DSCP values and podSelectors")
			egressQoSConfig = fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-11) + `
    podSelector:
      matchLabels:
        name: Harry
  - dscp: ` + strconv.Itoa(dscpValue-12) + `
    podSelector:
      matchLabels:
        name: Dobby
`)

			err = os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "apply", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Updating the first pod to match the first rule and the second pod to not match any rule")
			err = updatePodLabels(f, target1.pingPod, map[string]string{"name": "Harry"})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = updatePodLabels(f, target2.pingPod, nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the updated DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-11, 0)

			g.By("Deleting the EgressQoS")
			_, err = runOcWithRetry(oc.AsAdmin(), "delete", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is not marked with DSCP")
			pingAndCheckDSCP(oc, f, target1, target2, 0, 0)
		})

		g.It("EgressQoS should mark egress traffic based on both destination CIDR and podSelectors", func() {
			dscpValue := 48
			yamlPath := tmpDir + "/" + yamlFile

			g.By("Creating an EgressQoS with rules using both dstCIDR and podSelectors")
			egressQoSConfig := fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-1) + `
    dstCIDR: ` + target1.ip + "/32" + `
    podSelector:
      matchLabels:
        element: Air
  - dscp: ` + strconv.Itoa(dscpValue-2) + `
    dstCIDR: ` + target2.ip + "/32" + `
    podSelector:
      matchLabels:
        element: Water
`)

			err := os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "create", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating the source pods to ping from")
			podSync := errgroup.Group{}
			podSync.Go(func() error {
				var err error
				target1.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node1.Name
					p.Labels = map[string]string{"element": "Air"}
				})
				return err
			})
			podSync.Go(func() error {
				var err error
				target2.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ping-srcpod-", func(p *v1.Pod) {
					p.Spec.NodeName = node2.Name
					p.Labels = map[string]string{"element": "Water"}
				})
				return err
			})

			err = podSync.Wait()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the correct DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-1, dscpValue-2)

			g.By("Updating the EgressQoS DSCP values and selectors")
			egressQoSConfig = fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: EgressQoS
metadata:
  name: default
  namespace: ` + f.Namespace.Name + `
spec:
  egress:
  - dscp: ` + strconv.Itoa(dscpValue-11) + `
    dstCIDR: ` + target1.ip + "/32" + `
    podSelector:
      matchLabels:
        element: Earth
  - dscp: ` + strconv.Itoa(dscpValue-12) + `
    dstCIDR: ` + target2.ip + "/32" + `
    podSelector:
      matchLabels:
        element: Earth
`)

			err = os.WriteFile(yamlPath, []byte(egressQoSConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = runOcWithRetry(oc.AsAdmin(), "apply", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Updating the pods to match the podSelectors")
			err = updatePodLabels(f, target1.pingPod, map[string]string{"element": "Earth"})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = updatePodLabels(f, target2.pingPod, map[string]string{"element": "Earth"})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is marked with the updated DSCP values")
			pingAndCheckDSCP(oc, f, target1, target2, dscpValue-11, dscpValue-12)

			g.By("Deleting the EgressQoS")
			_, err = runOcWithRetry(oc.AsAdmin(), "delete", "-f", yamlPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Verifying the pods egress traffic is not marked with DSCP")
			pingAndCheckDSCP(oc, f, target1, target2, 0, 0)
		})
	})
})
