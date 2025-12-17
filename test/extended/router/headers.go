package router

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework/pod"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:operator.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "router", "router-http-echo-server.yaml")
		oc         = exutil.NewCLIWithPodSecurityLevel("router-headers", admissionapi.LevelBaseline)

		routerIP  string
		metricsIP string
		infra     *configv1.Infrastructure
		network   *configv1.Network
	)

	g.BeforeEach(func() {
		var err error
		infra, err = oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		network, err = oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		routerIP, err = exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		metricsIP, err = exutil.WaitForRouterInternalIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("should set Forwarded headers appropriately", g.Label("Size:M"), func() {
			o.Expect(infra).NotTo(o.BeNil())
			o.Expect(network).NotTo(o.BeNil())

			platformType := infra.Status.Platform
			if infra.Status.PlatformStatus != nil {
				platformType = infra.Status.PlatformStatus.Type
			}
			switch platformType {
			case configv1.AWSPlatformType, configv1.AzurePlatformType, configv1.GCPPlatformType:
				// supported
			default:
				g.Skip(fmt.Sprintf("BZ 1772125 -- not verified on platform type %q", platformType))
			}

			defer func() {
				// This should be done if the test fails but
				// for now always dump the logs.
				// if g.CurrentSpecReport().Failed()
				dumpRouterHeadersLogs(oc, g.CurrentSpecReport().FullText())
			}()

			ns := oc.KubeFramework().Namespace.Name
			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			g.By(fmt.Sprintf("creating an http echo server from a config file %q", configPath))

			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			var clientIP string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(ns).Get(context.Background(), "execpod", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(pod.Status.PodIP) == 0 {
					return false, nil
				}

				clientIP = pod.Status.PodIP
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// router expected to listen on port 80
			routerURL := fmt.Sprintf("http://%s", exutil.IPUrl(routerIP))

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s/healthz", net.JoinHostPort(metricsIP, "1936"))
			err = waitForRouterOKResponseExec(ns, execPod.Name, healthzURI, metricsIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			host := "router-headers.example.com"
			g.By(fmt.Sprintf("waiting for the route to become active"))
			err = waitForRouterOKResponseExec(ns, execPod.Name, routerURL, host, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("making a request and reading back the echoed headers"))
			var payload string
			payload, err = getRoutePayloadExec(ns, execPod.Name, routerURL, host)
			o.Expect(err).NotTo(o.HaveOccurred())

			// The trailing \n is being stripped, so add it back
			payload = payload + "\n"

			// parse the echoed request
			reader := bufio.NewReader(strings.NewReader(payload))
			req, err := http.ReadRequest(reader)
			o.Expect(err).NotTo(o.HaveOccurred())

			// check that the header is what we expect
			g.By(fmt.Sprintf("inspecting the echoed headers"))
			ffHeader := req.Header.Get("X-Forwarded-For")

			ignoreClientIP := false
			switch platformType {
			case configv1.AWSPlatformType:
				// On AWS we can only assert that we
				// get an X-Forwarded-For header; we
				// cannot assert its value because of
				// the following:
				//
				// The test runs as:
				//
				// # curl -s --header 'Host: router-headers.example.com' "http://a6d5a355fbd0f432da218598659513d5-219208002.us-east-1.elb.amazonaws.com"
				//
				// The curl address is routerIP, which
				// comes from:
				//
				// $ oc get service -n openshift-ingress -o yaml router-default
				// ...
				// apiVersion: v1
				// kind: Service
				// metadata:
				//   annotations:
				//     service.beta.kubernetes.io/aws-load-balancer-proxy-protocol: '*'
				// ...
				// status:
				//  loadBalancer:
				//    ingress:
				//    - hostname: a6d5a355fbd0f432da218598659513d5-219208002.us-east-1.elb.amazonaws.com
				//
				// If we resolve ingress.hostname we get:
				//
				// $ dig a6d5a355fbd0f432da218598659513d5-219208002.us-east-1.elb.amazonaws.com +short
				// 18.214.169.21
				// 3.233.35.82
				//
				// Looking at the route for the HTTP GET we see:
				//
				// traceroute to 18.214.169.21 (18.214.169.21), 30 hops max, 46 byte packets
				//  1  ip-10-128-2-1.ec2.internal (10.128.2.1)
				//  2  ip-10-0-39-215.ec2.internal (10.0.39.215)
				//  3  216.182.226.180 (216.182.226.180)
				//
				// At (2) we hit the elastic-IP. Our
				// path back is via the public-facing
				// side of the elastic IP address
				// (which is 35.175.101.212) -- egress
				// from the POD will now have source
				// addresses NAT'd to this elastic IP.
				// This is reflected in the
				// results/headers:
				//
				// GET / HTTP/1.1
				// User-Agent: curl/7.61.1
				// Accept: */*
				// Host: router-headers.example.com
				// X-Forwarded-Host: router-headers.example.com
				// X-Forwarded-Port: 80
				// X-Forwarded-Proto: http
				// Forwarded: for=35.175.101.212;host=router-headers.example.com;proto=http;proto-version=""
				// X-Forwarded-For: 35.175.101.212
				//
				// And the X-Forwarded-For value
				// (35.175.101.212) will never match
				// `clientIP` given the route the GET
				// request takes. So for AWS we just
				// expect the header to be present.
				ignoreClientIP = true
			}

			if network.Status.NetworkType == "OVNKubernetes" {
				// Similarly to AWS, the connection is
				// NAT'd to an unknown address when
				// the network plugin is
				// OVNKubernetes, so we must disable
				// the check in this case too.
				ignoreClientIP = true
			}

			if ignoreClientIP {
				if ffHeader == "" {
					e2e.Failf("Expected X-Forwarded-For header; All headers: %#v", req.Header)
				}
			} else {
				if ffHeader != clientIP {
					e2e.Failf("Unexpected header: '%s' (expected %s); All headers: %#v", ffHeader, clientIP, req.Header)
				}
			}
		})
	})
})

func dumpRouterHeadersLogs(oc *exutil.CLI, name string) {
	log, _ := pod.GetPodLogs(context.TODO(), oc.AdminKubeClient(), oc.KubeFramework().Namespace.Name, "router-headers", "router")
	e2e.Logf("Weighted Router test %s logs:\n %s", name, log)
}

func getRoutePayloadExec(ns, execPodName, url, host string) (string, error) {
	cmd := fmt.Sprintf(`
		set -e
		rc=0
		payload=$( curl -s --header 'Host: %s' %q ) || rc=$?
		if [[ "${rc:-0}" -eq 0 ]]; then
			printf "${payload}"
			exit 0
		else
			echo "error ${rc}" 1>&2
			exit 1
		fi
		`, host, url)
	output, err := e2eoutput.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}
