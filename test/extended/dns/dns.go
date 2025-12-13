package dns

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"

	ocpv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

func createDNSPod(namespace, probeCmd string, nodeSelector map[string]string) *kapiv1.Pod {
	pod := &kapiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dns-test-" + string(uuid.NewUUID()),
			Namespace: namespace,
		},
		Spec: kapiv1.PodSpec{
			RestartPolicy:   kapiv1.RestartPolicyNever,
			SecurityContext: e2epod.GetRestrictedPodSecurityContext(),
			NodeSelector:    nodeSelector,
			Tolerations: []kapiv1.Toleration{
				{
					Effect:   "NoSchedule",
					Key:      "node-role.kubernetes.io/master",
					Operator: kapiv1.TolerationOpExists,
				},
			},
			Containers: []kapiv1.Container{
				{
					Name:            "querier",
					Image:           imageutils.GetE2EImage(imageutils.JessieDnsutils),
					Command:         []string{"/bin/sh", "-c", probeCmd},
					SecurityContext: e2epod.GetRestrictedContainerSecurityContext(),
				},
			},
		},
	}
	var uid int64 = 65536
	pod.Spec.SecurityContext.RunAsUser = &uid
	return pod
}

func digForNames(namesToResolve []string, expect sets.String) string {
	fileNamePrefix := "test"
	var probeCmd string
	for _, name := range namesToResolve {
		// Resolve by TCP and UDP DNS.  Use $$(...) because $(...) is
		// expanded by kubernetes (though this won't expand so should
		// remain a literal, safe > sorry).
		lookup := "A"
		if strings.HasPrefix(name, "_") {
			lookup = "SRV"
		}
		fileName := fmt.Sprintf("%s_udp@%s", fileNamePrefix, name)
		expect.Insert(fileName)
		probeCmd += fmt.Sprintf(`test -n "$$(dig +notcp +noall +answer +search %s %s)" && echo %q;`, name, lookup, fileName)
		fileName = fmt.Sprintf("%s_tcp@%s", fileNamePrefix, name)
		expect.Insert(fileName)
		probeCmd += fmt.Sprintf(`test -n "$$(dig +tcp +noall +answer +search %s %s)" && echo %q;`, name, lookup, fileName)
	}
	return probeCmd
}

func digForCNAMEs(namesToResolve []string, expect sets.String) string {
	fileNamePrefix := "test"
	var probeCmd string
	for _, name := range namesToResolve {
		// Resolve by TCP and UDP DNS.  Use $$(...) because $(...) is
		// expanded by kubernetes (though this won't expand so should
		// remain a literal, safe > sorry).
		lookup := "CNAME"
		fileName := fmt.Sprintf("%s_udp@%s", fileNamePrefix, name)
		expect.Insert(fileName)
		probeCmd += fmt.Sprintf(`test -n "$$(dig +notcp +noall +answer +search %s %s)" && echo %q;`, name, lookup, fileName)
		fileName = fmt.Sprintf("%s_tcp@%s", fileNamePrefix, name)
		expect.Insert(fileName)
		probeCmd += fmt.Sprintf(`test -n "$$(dig +tcp +noall +answer +search %s %s)" && echo %q;`, name, lookup, fileName)
	}
	return probeCmd
}

func digForSRVs(namesToResolve []string, expect sets.String) string {
	fileNamePrefix := "test"
	var probeCmd string
	for _, name := range namesToResolve {
		// Resolve by TCP and UDP DNS.  Use $$(...) because $(...) is
		// expanded by kubernetes (though this won't expand so should
		// remain a literal, safe > sorry).
		lookup := "SRV"
		fileName := fmt.Sprintf("%s_udp@%s", fileNamePrefix, name)
		expect.Insert(fileName)
		probeCmd += fmt.Sprintf(`test -n "$$(dig +notcp +noall +additional +search %s %s)" && echo %q;`, name, lookup, fileName)
		fileName = fmt.Sprintf("%s_tcp@%s", fileNamePrefix, name)
		expect.Insert(fileName)
		probeCmd += fmt.Sprintf(`test -n "$$(dig +tcp +noall +additional +search %s %s)" && echo %q;`, name, lookup, fileName)
	}
	return probeCmd
}

func digForARecords(records map[string][]string, expect sets.String) string {
	var probeCmd string
	fileNamePrefix := "test"
	for name, ips := range records {
		fileName := fmt.Sprintf("%s_endpoints@%s", fileNamePrefix, name)
		probeCmd += fmt.Sprintf(`[ "$$(dig +short +notcp +noall +answer +search %s A | sort | xargs echo)" = "%s" ] && echo %q;`, name, strings.Join(ips, " "), fileName)
		expect.Insert(fileName)
	}
	return probeCmd
}

func digForAAAARecords(records map[string][]string, expect sets.String) string {
	var probeCmd string
	fileNamePrefix := "test"
	for name, ips := range records {
		fileName := fmt.Sprintf("%s_endpoints_v6@%s", fileNamePrefix, name)
		probeCmd += fmt.Sprintf(`[ "$$(dig +short +notcp +noall +answer +search %s AAAA | sort | xargs echo)" = "%s" ] && echo %q;`, name, strings.Join(ips, " "), fileName)
		expect.Insert(fileName)
	}
	return probeCmd
}

func reverseIP(ip string) string {
	a := strings.Split(ip, ".")
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
	return strings.Join(a, ".")
}

func digForPTRRecords(records map[string]string, expect sets.String) string {
	var probeCmd string
	fileNamePrefix := "test"
	for ip, name := range records {
		fileName := fmt.Sprintf("%s_ptr@%s", fileNamePrefix, ip)
		probeCmd += fmt.Sprintf(`[ "$(dig +short +notcp +noall +answer +search %s.in-addr.arpa PTR)" = "%s" ] && echo %q;`, reverseIP(ip), name, fileName)
		//probeCmd += fmt.Sprintf(`echo "$(dig +short +notcp +noall +answer +search %s.in-addr.arpa PTR)" "# %s %s" %q;`, reverseIP(ip), reverseIP(ip), name, fileName)
		expect.Insert(fileName)
	}
	return probeCmd
}

func digForPod(namespace string, expect sets.String) string {
	var probeCmd string
	fileNamePrefix := "test"
	podARecByUDPFileName := fmt.Sprintf("%s_udp@PodARecord", fileNamePrefix)
	podARecByTCPFileName := fmt.Sprintf("%s_tcp@PodARecord", fileNamePrefix)
	probeCmd += fmt.Sprintf(`podARec=$$(hostname -i| awk -F. '{print $$1"-"$$2"-"$$3"-"$$4".%s.pod.cluster.local"}');`, namespace)
	probeCmd += fmt.Sprintf(`test -n "$$(dig +notcp +noall +answer +search $${podARec} A)" && echo %q;`, podARecByUDPFileName)
	probeCmd += fmt.Sprintf(`test -n "$$(dig +tcp +noall +answer +search $${podARec} A)" && echo %q;`, podARecByTCPFileName)
	expect.Insert(podARecByUDPFileName, podARecByTCPFileName)
	return probeCmd
}

func repeatCommand(times int, cmd ...string) string {
	probeCmd := fmt.Sprintf("for i in `seq 1 %d`; do ", times)
	probeCmd += strings.Join(cmd, " ")
	probeCmd += "sleep 1; done"
	return probeCmd
}

func assertLinesExist(lines sets.String, expect int, r io.Reader) error {
	count := make(map[string]int)
	unrecognized := sets.NewString()
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if lines.Has(line) {
			count[line]++
		} else {
			unrecognized.Insert(line)
		}
	}
	for k := range lines {
		if count[k] != expect {
			return fmt.Errorf("unexpected count %d/%d for %q: %v", count[k], expect, k, unrecognized)
		}
	}
	if unrecognized.Len() > 0 {
		return fmt.Errorf("unexpected matches from output: %v", unrecognized)
	}
	return nil
}

// PodSucceeded returns true if the pod has succeeded, false if the pod has not yet
// reached running state, or an error in any other case.
func PodSucceeded(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
	}
	switch t := event.Object.(type) {
	case *kapiv1.Pod:
		switch t.Status.Phase {
		case kapiv1.PodSucceeded:
			return true, nil
		case kapiv1.PodFailed:
			return false, fmt.Errorf("pod failed: %#v", t)
		}
	}
	return false, nil
}

func validateDNSResults(f *e2e.Framework, pod *kapiv1.Pod, fileNames sets.String, expect int) {
	By("submitting the pod to kubernetes")
	podClient := f.ClientSet.CoreV1().Pods(f.Namespace.Name)
	defer func() {
		By("deleting the pod")
		defer GinkgoRecover()
		podClient.Delete(context.Background(), pod.Name, *metav1.NewDeleteOptions(0))
	}()
	updated, err := podClient.Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		e2e.Failf("Failed to create %s pod: %v", pod.Name, err)
	}

	w, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Watch(context.Background(), metav1.SingleObject(metav1.ObjectMeta{Name: pod.Name, ResourceVersion: updated.ResourceVersion}))
	if err != nil {
		e2e.Failf("Failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), e2e.PodStartTimeout)
	defer cancel()
	if _, err = watchtools.UntilWithoutRetry(ctx, w, PodSucceeded); err != nil {
		e2e.Failf("Failed: %v", err)
	}

	By("retrieving the pod logs")
	r, err := podClient.GetLogs(pod.Name, &kapiv1.PodLogOptions{Container: "querier"}).Stream(context.Background())
	if err != nil {
		e2e.Failf("Failed to get pod logs %s: %v", pod.Name, err)
	}
	out, err := ioutil.ReadAll(r)
	if err != nil {
		e2e.Failf("Failed to read pod logs %s: %v", pod.Name, err)
	}

	// Try to find results for each expected name.
	By("looking for the results for each expected name from probers")

	if err := assertLinesExist(fileNames, expect, bytes.NewBuffer(out)); err != nil {
		e2e.Logf("Got results from pod:\n%s", out)
		e2e.Failf("Unexpected results: %v", err)
	}

	e2e.Logf("DNS probes using %s succeeded\n", pod.Name)
}

func createServiceSpec(serviceName string, isHeadless bool, externalName string, selector map[string]string) *kapiv1.Service {
	s := &kapiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: kapiv1.ServiceSpec{
			Ports: []kapiv1.ServicePort{
				{Port: 80, Name: "http", Protocol: "TCP"},
			},
			Selector: selector,
		},
	}
	if isHeadless {
		s.Spec.ClusterIP = "None"
	}
	if len(externalName) > 0 {
		s.Spec.Type = kapiv1.ServiceTypeExternalName
		s.Spec.ExternalName = externalName
		s.Spec.ClusterIP = ""
	}
	return s
}

func createDualStackServiceSpec(serviceName string, isHeadless bool, externalName string, selector map[string]string) *kapiv1.Service {
	s := createServiceSpec(serviceName, isHeadless, externalName, selector)
	s.Spec.IPFamilies = []kapiv1.IPFamily{
		kapiv1.IPv4Protocol,
		kapiv1.IPv6Protocol,
	}

	ipFamilyPolicy := kapiv1.IPFamilyPolicyRequireDualStack
	s.Spec.IPFamilyPolicy = &ipFamilyPolicy

	return s
}

func createEndpointSpec(name string) *kapiv1.Endpoints {
	return &kapiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subsets: []kapiv1.EndpointSubset{
			{
				Addresses: []kapiv1.EndpointAddress{
					{IP: "1.1.1.1", Hostname: "endpoint1"},
					{IP: "1.1.1.2"},
				},
				NotReadyAddresses: []kapiv1.EndpointAddress{
					{IP: "2.1.1.1"},
					{IP: "2.1.1.2"},
				},
				Ports: []kapiv1.EndpointPort{
					{Port: 80},
				},
			},
		},
	}
}

func createEndpointSliceSpec(name, serviceName string, addressType discoveryv1.AddressType) *discoveryv1.EndpointSlice {
	port := int32(80)
	es := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + strings.ToLower(string(addressType)),
			Labels: map[string]string{
				discoveryv1.LabelServiceName: serviceName,
			},
		},
		AddressType: addressType,
		Ports: []discoveryv1.EndpointPort{{
			Port: &port,
		}},
	}

	switch addressType {
	case discoveryv1.AddressTypeIPv4:
		es.Endpoints = []discoveryv1.Endpoint{{
			Addresses: []string{
				"3.3.3.3",
				"4.4.4.4",
			},
			Hostname: stringPtr(strings.ToLower(string(addressType))),
		}}
	case discoveryv1.AddressTypeIPv6:
		es.Endpoints = []discoveryv1.Endpoint{{
			Addresses: []string{
				"2001:4860:4860::3333",
				"2001:4860:4860::4444",
			},
			Hostname: stringPtr(strings.ToLower(string(addressType))),
		}}
	}

	return es
}

func stringPtr(s string) *string {
	return &s
}

func ipsForEndpoints(ep *kapiv1.Endpoints) []string {
	ips := sets.NewString()
	for _, sub := range ep.Subsets {
		for _, addr := range sub.Addresses {
			ips.Insert(addr.IP)
		}
	}
	return ips.List()
}

func ipsForEndpointSlice(es *discoveryv1.EndpointSlice) []string {
	ips := sets.NewString()
	for _, endpoint := range es.Endpoints {
		ips.Insert(endpoint.Addresses...)
	}

	return ips.List()
}

// validateLocalDNSPodPreference confirms DNS queries prefer the local DNS Pod. It fails if a DNS Pod other than the
// local DNS pod responds to the DNS query.
func validateLocalDNSPodPreference(queryPodExec *exutil.PodExecutor, localDNSPodName string, extraDigArgs ...string) {
	// This queries for the hostname of the server responding to the DNS request enabled by the chaos plugin.
	// More information: https://coredns.io/plugins/chaos/
	hostnameDig := strings.Join(append(append([]string{"dig", "+short", "+noall", "+answer"}, extraDigArgs...), "CH", "TXT", "hostname.bind"), " ")
	const attempts = 10

	By(fmt.Sprintf("running this command %d times: %s", attempts, hostnameDig))
	for i := 1; i <= attempts; i++ {
		digOut, err := queryPodExec.Exec(hostnameDig)
		dnsPodFound := strings.Trim(digOut, `"`)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dnsPodFound).To(o.Equal(localDNSPodName), "expected DNS chaos query to always use the local DNS pod %q, but received %q", localDNSPodName, dnsPodFound)
	}
}

var _ = Describe("[sig-network-edge] DNS", func() {
	f := e2e.NewDefaultFramework("dns")
	oc := exutil.NewCLI("dns-dualstack")
	nodeSelector := make(map[string]string)

	BeforeEach(func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology != ocpv1.ExternalTopologyMode {
			nodeSelector["node-role.kubernetes.io/master"] = ""
		}
	})

	It("should answer endpoint and wildcard queries for the cluster", Label("Size:L"), func() {
		ctx := context.Background()
		createOpts := metav1.CreateOptions{}
		if _, err := f.ClientSet.CoreV1().Services(f.Namespace.Name).Create(ctx, createServiceSpec("headless", true, "", nil), createOpts); err != nil {
			e2e.Failf("unable to create headless service: %v", err)
		}
		if _, err := f.ClientSet.CoreV1().Endpoints(f.Namespace.Name).Create(ctx, createEndpointSpec("headless"), createOpts); err != nil {
			e2e.Failf("unable to create clusterip endpoints: %v", err)
		}
		if _, err := f.ClientSet.CoreV1().Services(f.Namespace.Name).Create(ctx, createServiceSpec("clusterip", false, "", nil), createOpts); err != nil {
			e2e.Failf("unable to create clusterip service: %v", err)
		}
		if _, err := f.ClientSet.CoreV1().Endpoints(f.Namespace.Name).Create(ctx, createEndpointSpec("clusterip"), createOpts); err != nil {
			e2e.Failf("unable to create clusterip endpoints: %v", err)
		}
		if _, err := f.ClientSet.CoreV1().Services(f.Namespace.Name).Create(ctx, createServiceSpec("externalname", true, "www.google.com", nil), createOpts); err != nil {
			e2e.Failf("unable to create externalName service: %v", err)
		}

		ep, err := f.ClientSet.CoreV1().Endpoints("default").Get(ctx, "kubernetes", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to find endpoints for kubernetes.default: %v", err)
		}
		kubeEndpoints := ipsForEndpoints(ep)

		readyEndpoints := ipsForEndpoints(createEndpointSpec(""))

		// All the names we need to be able to resolve.
		expect := sets.NewString()
		times := 10
		cmd := repeatCommand(
			times,
			// the DNS pod should be able to resolve these names
			digForNames([]string{
				// answer wildcards on default service
				"prefix.kubernetes.default",
				"prefix.kubernetes.default.svc",
				"prefix.kubernetes.default.svc.cluster.local",

				// answer wildcards on clusterIP services
				fmt.Sprintf("prefix.clusterip.%s", f.Namespace.Name),
			}, expect),

			// the DNS pod should be able to get additional A records for this service
			digForSRVs([]string{
				fmt.Sprintf("_http._tcp.externalname.%s.svc", f.Namespace.Name),
			}, expect),

			// the DNS pod should be able to get a CNAME for this service
			digForCNAMEs([]string{
				fmt.Sprintf("externalname.%s.svc", f.Namespace.Name),
			}, expect),

			// the DNS pod should be able to look up endpoints for names and wildcards
			digForARecords(map[string][]string{
				"kubernetes.default.endpoints": kubeEndpoints,

				fmt.Sprintf("headless.%s.svc", f.Namespace.Name):        readyEndpoints,
				fmt.Sprintf("headless.%s.endpoints", f.Namespace.Name):  readyEndpoints,
				fmt.Sprintf("clusterip.%s.endpoints", f.Namespace.Name): readyEndpoints,

				fmt.Sprintf("endpoint1.headless.%s.endpoints", f.Namespace.Name):  {"1.1.1.1"},
				fmt.Sprintf("endpoint1.clusterip.%s.endpoints", f.Namespace.Name): {"1.1.1.1"},
			}, expect),

			// the DNS pod should be able to find an endpoint hostname via a PTR record for the IP
			digForPTRRecords(map[string]string{
				"1.1.1.1": fmt.Sprintf("endpoint1.headless.%s.svc.cluster.local.", f.Namespace.Name),
				"1.1.1.2": "", // has no hostname
				"2.1.1.1": "", // has no hostname
			}, expect),

			// the DNS pod should respond to its own request
			digForPod(f.Namespace.Name, expect),
		)

		By("Running these commands:" + cmd + "\n")

		// Run a pod which probes DNS and exposes the results by HTTP.
		By("creating a pod to probe DNS")
		pod := createDNSPod(f.Namespace.Name, cmd, nodeSelector)
		validateDNSResults(f, pod, expect, times)
	})

	It("should answer A and AAAA queries for a dual-stack service [apigroup:config.openshift.io]", Label("Size:M"), func() {
		// Only run this test on dual-stack enabled clusters.
		networkConfig, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to get cluster network config: %v", err)
		}
		usingIPv4 := false
		usingIPv6 := false
		for _, clusterNetworkEntry := range networkConfig.Status.ClusterNetwork {
			addr, _, err := net.ParseCIDR(clusterNetworkEntry.CIDR)
			if err != nil {
				continue
			}
			if addr.To4() != nil {
				usingIPv4 = true
			} else {
				usingIPv6 = true
			}
		}

		if !usingIPv4 || !usingIPv6 {
			Skip("skipping test on non dual-stack enabled platform")
		}

		By("creating a dual-stack service on a dual-stack cluster")

		ctx := context.Background()
		serviceName := "v4v6"
		createOpts := metav1.CreateOptions{}
		service, err := f.ClientSet.CoreV1().Services(f.Namespace.Name).Create(ctx, createDualStackServiceSpec(serviceName, false, "", nil), createOpts)
		if err != nil {
			e2e.Failf("unable to create dual-stack service: %v", err)
		}

		v4 := createEndpointSliceSpec("dns-test", serviceName, discoveryv1.AddressTypeIPv4)
		if _, err := f.ClientSet.DiscoveryV1().EndpointSlices(f.Namespace.Name).Create(ctx, v4, createOpts); err != nil {
			e2e.Failf("unable to create endpointslice %s: %v", v4.Name, err)
		}

		v6 := createEndpointSliceSpec("dns-test", serviceName, discoveryv1.AddressTypeIPv6)
		if _, err := f.ClientSet.DiscoveryV1().EndpointSlices(f.Namespace.Name).Create(ctx, v6, createOpts); err != nil {
			e2e.Failf("unable to create endpointslice %s: %v", v6.Name, err)
		}

		v4ips := ipsForEndpointSlice(v4)
		v6ips := ipsForEndpointSlice(v6)

		var v4ClusterIP string
		var v6ClusterIP string

		for i, ipFamily := range service.Spec.IPFamilies {
			if ipFamily == kapiv1.IPv4Protocol {
				v4ClusterIP = service.Spec.ClusterIPs[i]
			} else if ipFamily == kapiv1.IPv6Protocol {
				v6ClusterIP = service.Spec.ClusterIPs[i]
			}
		}

		// All the names we need to be able to resolve.
		expect := sets.NewString()
		times := 10
		cmd := repeatCommand(
			times,
			// Verify that <service>.<namespace>.svc resolves as expected
			// using A and AAAA queries.
			digForARecords(map[string][]string{
				fmt.Sprintf("%s.%s.svc", serviceName, f.Namespace.Name): {v4ClusterIP},
			}, expect),
			digForAAAARecords(map[string][]string{
				fmt.Sprintf("%s.%s.svc", serviceName, f.Namespace.Name): {v6ClusterIP},
			}, expect),
			// Verify that service endpointslices resolve as expected using A and AAAA queries.
			digForARecords(map[string][]string{
				fmt.Sprintf("%s.%s.%s.svc", strings.ToLower(string(v4.AddressType)), serviceName, f.Namespace.Name): v4ips,
			}, expect),
			digForAAAARecords(map[string][]string{
				fmt.Sprintf("%s.%s.%s.svc", strings.ToLower(string(v6.AddressType)), serviceName, f.Namespace.Name): v6ips,
			}, expect),
		)

		By("Running these commands:" + cmd + "\n")

		// Run a pod which probes DNS and exposes the results.
		By("creating a pod to probe DNS")
		pod := createDNSPod(f.Namespace.Name, cmd, nodeSelector)
		validateDNSResults(f, pod, expect, times)
	})

	It("should answer queries using the local DNS endpoint", Label("Size:M"), func() {
		ctx := context.Background()

		By("starting an exec pod for running queries")

		queryPodExec, err := exutil.NewPodExecutor(oc, "dnsquery", image.ShellImage())
		o.Expect(err).NotTo(o.HaveOccurred())

		queryPod, err := f.ClientSet.CoreV1().Pods(oc.Namespace()).Get(ctx, "dnsquery", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("DNS query pod is on node: %q", queryPod.Spec.NodeName)

		By("finding the exec pod's local DNS endpoint")

		// We find the local dns pod by filtering for pods in the openshift-dns namespace and owned by default
		// dns daemonset, then making sure that the pod is on the same node, is running, and is not being deleted.
		// Note: We expect there to be a pod co-located with query pod at all times, even if there is a taint applied
		// to the cluster under test. The taint should apply both the query pod and DNS pod together.
		dnsPodLabel := exutil.ParseLabelsOrDie("dns.operator.openshift.io/daemonset-dns=default")
		isLocalReadyPod := func(pod kapiv1.Pod) bool {
			return exutil.CheckPodIsReady(pod) && pod.Spec.NodeName == queryPod.Spec.NodeName
		}
		localDnsPodNames, err := exutil.WaitForPods(f.ClientSet.CoreV1().Pods("openshift-dns"), dnsPodLabel, isLocalReadyPod, 1, 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(localDnsPodNames).To(o.HaveLen(1))
		localDnsPodName := localDnsPodNames[0]
		e2e.Logf("Local DNS Pod: %q", localDnsPodName)

		By("validating UDP local DNS Pod Preference")
		validateLocalDNSPodPreference(queryPodExec, localDnsPodName)

		By("validating TCP local DNS Pod Preference")
		validateLocalDNSPodPreference(queryPodExec, localDnsPodName, "+tcp")
	})
})
