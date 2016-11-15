package dns

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func createDNSPod(namespace, probeCmd string) *api.Pod {
	pod := &api.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: registered.GroupOrDie(api.GroupName).GroupVersion.String(),
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "dns-test-" + string(uuid.NewUUID()),
			Namespace: namespace,
		},
		Spec: api.PodSpec{
			RestartPolicy: api.RestartPolicyNever,
			Containers: []api.Container{
				{
					Name:    "querier",
					Image:   "gcr.io/google_containers/dnsutils:e2e",
					Command: []string{"sh", "-c", probeCmd},
				},
			},
		},
	}
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
		return false, errors.NewNotFound(unversioned.GroupResource{Resource: "pods"}, "")
	}
	switch t := event.Object.(type) {
	case *api.Pod:
		switch t.Status.Phase {
		case api.PodSucceeded:
			return true, nil
		case api.PodFailed:
			return false, fmt.Errorf("pod failed: %#v", t)
		}
	}
	return false, nil
}

func validateDNSResults(f *e2e.Framework, pod *api.Pod, fileNames sets.String, expect int) {
	By("submitting the pod to kubernetes")
	podClient := f.Client.Pods(f.Namespace.Name)
	defer func() {
		By("deleting the pod")
		defer GinkgoRecover()
		podClient.Delete(pod.Name, api.NewDeleteOptions(0))
	}()
	updated, err := podClient.Create(pod)
	if err != nil {
		e2e.Failf("Failed to create %s pod: %v", pod.Name, err)
	}

	w, err := f.Client.Pods(f.Namespace.Name).Watch(api.SingleObject(api.ObjectMeta{Name: pod.Name, ResourceVersion: updated.ResourceVersion}))
	if err != nil {
		e2e.Failf("Failed: %v", err)
	}
	if _, err = watch.Until(e2e.PodStartTimeout, w, PodSucceeded); err != nil {
		e2e.Failf("Failed: %v", err)
	}

	By("retrieving the pod logs")
	r, err := podClient.GetLogs(pod.Name, &api.PodLogOptions{Container: "querier"}).Stream()
	if err != nil {
		e2e.Failf("Failed to get pod logs %s: %v", pod.Name, err)
	}
	out, err := ioutil.ReadAll(r)
	if err != nil {
		e2e.Failf("Failed to read pod logs %s: %v", pod.Name, err)
	}

	// Try to find results for each expected name.
	By("looking for the results for each expected name from probiers")

	if err := assertLinesExist(fileNames, expect, bytes.NewBuffer(out)); err != nil {
		e2e.Logf("Got results from pod:\n%s", out)
		e2e.Failf("Unexpected results: %v", err)
	}

	e2e.Logf("DNS probes using %s succeeded\n", pod.Name)
}

func createServiceSpec(serviceName string, isHeadless bool, externalName string, selector map[string]string) *api.Service {
	s := &api.Service{
		ObjectMeta: api.ObjectMeta{
			Name: serviceName,
		},
		Spec: api.ServiceSpec{
			Ports: []api.ServicePort{
				{Port: 80, Name: "http", Protocol: "TCP"},
			},
			Selector: selector,
		},
	}
	if isHeadless {
		s.Spec.ClusterIP = "None"
	}
	if len(externalName) > 0 {
		s.Spec.Type = api.ServiceTypeExternalName
		s.Spec.ExternalName = externalName
		s.Spec.ClusterIP = ""
	}
	return s
}

func createEndpointSpec(name string) *api.Endpoints {
	return &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			Name: name,
		},
		Subsets: []api.EndpointSubset{
			{
				Addresses: []api.EndpointAddress{
					{IP: "1.1.1.1", Hostname: "endpoint1"},
					{IP: "1.1.1.2"},
				},
				NotReadyAddresses: []api.EndpointAddress{
					{IP: "2.1.1.1"},
					{IP: "2.1.1.2"},
				},
				Ports: []api.EndpointPort{
					{Port: 80},
				},
			},
		},
	}
}

func ipsForEndpoints(ep *api.Endpoints) []string {
	ips := sets.NewString()
	for _, sub := range ep.Subsets {
		for _, addr := range sub.Addresses {
			ips.Insert(addr.IP)
		}
	}
	return ips.List()
}

var _ = Describe("DNS", func() {
	f := e2e.NewDefaultFramework("dns")

	It("should answer endpoint and wildcard queries for the cluster [Conformance]", func() {
		if _, err := f.Client.Services(f.Namespace.Name).Create(createServiceSpec("headless", true, "", nil)); err != nil {
			e2e.Failf("unable to create headless service: %v", err)
		}
		if _, err := f.Client.Endpoints(f.Namespace.Name).Create(createEndpointSpec("headless")); err != nil {
			e2e.Failf("unable to create clusterip endpoints: %v", err)
		}
		if _, err := f.Client.Services(f.Namespace.Name).Create(createServiceSpec("clusterip", false, "", nil)); err != nil {
			e2e.Failf("unable to create clusterip service: %v", err)
		}
		if _, err := f.Client.Endpoints(f.Namespace.Name).Create(createEndpointSpec("clusterip")); err != nil {
			e2e.Failf("unable to create clusterip endpoints: %v", err)
		}
		if _, err := f.Client.Services(f.Namespace.Name).Create(createServiceSpec("externalname", true, "www.google.com", nil)); err != nil {
			e2e.Failf("unable to create externalName service: %v", err)
		}

		ep, err := f.Client.Endpoints("default").Get("kubernetes")
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

			// the DNS pod should respond to its own request
			digForPod(f.Namespace.Name, expect),
		)

		By("Running these commands:" + cmd + "\n")

		// Run a pod which probes DNS and exposes the results by HTTP.
		By("creating a pod to probe DNS")
		pod := createDNSPod(f.Namespace.Name, cmd)
		validateDNSResults(f, pod, expect, times)
	})
})
