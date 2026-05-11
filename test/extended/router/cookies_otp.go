package router

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-network-edge] Network_Edge Component_Router [OTP]", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("route-cookies", admissionapi.LevelBaseline)

	g.It("[OTP] [OCP-11903] haproxy cookies based sticky session for unsecure routes [apigroup:route.openshift.io]", func() {
		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			customTemp          = filepath.Join(buildPruningBaseDir, "ingresscontroller-np.yaml")
			clientPod           = filepath.Join(buildPruningBaseDir, "test-client-pod.yaml")
			clientPodName       = "hello-pod"
			clientPodLabel      = "app=hello-pod"
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-deploy.yaml")
			srvrcInfo           = "web-server-deploy"
			unsecSvcName        = "service-unsecure"
			cookie              = "/data/OCP-11903-cookie"

			ingctrl = ingressControllerDescription{
				name:      "otp11903",
				namespace: "openshift-ingress-operator",
				domain:    "",
				template:  customTemp,
			}
		)

		g.By("1.0 Create a custom ingresscontroller")
		baseDomain := getBaseDomain(oc)
		ingctrl.domain = ingctrl.name + "." + baseDomain
		defer ingctrl.delete(oc)
		ingctrl.create(oc)
		ensureRouterDeployGenerationIs(oc, ingctrl.name, "1")

		g.By("2.0: Prepare file for testing")
		updateFilebySedCmd(testPodSvc, "replicas: 1", "replicas: 2")

		g.By("3.0: Create a client pod and two server pods and the service")
		ns := oc.Namespace()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", ns, "-f", clientPod).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		ensurePodWithLabelReady(oc, ns, clientPodLabel)
		srvPodList := createResourceFromWebServer(oc, ns, testPodSvc, srvrcInfo)

		g.By("4.0: Create a plain HTTP route")
		routehost := "unsecure11903" + "." + ingctrl.domain
		createRouteOTP(oc, ns, "http", unsecSvcName, unsecSvcName, []string{"--hostname=" + routehost})
		ensureRouteIsAdmittedByIngressController(oc, ns, unsecSvcName, ingctrl.name)

		g.By("5.0: Curl the http route, make sure the second server is hit")
		routerpod := getOneNewRouterPodFromRollingUpdate(oc, ingctrl.name)
		podIP := getPodv4Address(oc, routerpod, "openshift-ingress")
		toDst := routehost + ":80:" + podIP
		curlCmd := []string{"-n", ns, clientPodName, "--", "curl", "http://" + routehost, "-s", "-c", cookie, "--resolve", toDst, "--connect-timeout", "10"}
		expectOutput := []string{"Hello-OpenShift " + srvPodList[1] + " http-8080"}
		repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)

		g.By("6.0: Curl the http route again, make sure the first server is hit")
		curlCmd = []string{"-n", ns, clientPodName, "--", "curl", "http://" + routehost, "-s", "--resolve", toDst, "--connect-timeout", "10"}
		expectOutput = []string{"Hello-OpenShift " + srvPodList[0] + " http-8080"}
		repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)

		g.By("7.0: Curl the http route with the specified cookie for 10 times, expect all are forwarded to the second server")
		curlCmd = []string{"-n", ns, clientPodName, "--", "curl", "http://" + routehost, "-s", "-b", cookie, "--resolve", toDst, "--connect-timeout", "10"}
		expectOutput = []string{"Hello-OpenShift " + srvPodList[0] + " http-8080", "Hello-OpenShift " + srvPodList[1] + " http-8080"}
		_, result := repeatCmdOnClient(oc, curlCmd, expectOutput, 120, 10)
		o.Expect(result[1]).To(o.Equal(10))

		g.By(`8.0: Disable haproxy hash based sticky session for the route by adding 'disable cookies' annotation to it`)
		setAnnotation(oc, ns, "route/"+unsecSvcName, "haproxy.router.openshift.io/disable_cookies=true")

		g.By("9.0: Check the disable cookies configuration in haproxy")
		backendStart := fmt.Sprintf(`backend be_http:%s:%s`, ns, unsecSvcName)
		ensureHaproxyBlockConfigNotMatchRegexp(oc, routerpod, backendStart, []string{`cookie .+httponly`})

		g.By("10.0: Curl the http route with the specified cookie for 10 times again, expect the requests are forwarded to the two servers")
		_, result = repeatCmdOnClient(oc, curlCmd, expectOutput, 120, 10)
		o.Expect(result[0] + result[1]).To(o.Equal(10))
		o.Expect(result[0] * result[1] > 0).To(o.BeTrue())
	})

	g.It("[OTP] [OCP-12566] Cookie name should not use openshift prefix [apigroup:route.openshift.io]", func() {
		if !isCanaryRouteAvailable(oc) {
			g.Skip("Skip for the ingress canary route could not be available to the outside")
		}

		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-deploy.yaml")
			srvrcInfo           = "web-server-deploy"
			unsecSvcName        = "service-unsecure"
			fileDir             = "/tmp/OCP-12566-cookie"
			cookie              = fileDir + "/cookie12566"
			routeName           = "unsecureroute2"
		)

		g.By("1.0: Prepare file folder and file for testing")
		defer os.RemoveAll(fileDir)
		err := os.MkdirAll(fileDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2.0: Create two server pods and the service")
		ns := oc.Namespace()
		updateFilebySedCmd(testPodSvc, "replicas: 1", "replicas: 2")
		createResourceFromWebServer(oc, ns, testPodSvc, srvrcInfo)

		g.By("3.0: Create a plain HTTP route")
		routehost := "unsecure12566" + ".apps." + getBaseDomain(oc)
		createRouteOTP(oc, ns, "http", routeName, unsecSvcName, []string{"--hostname=" + routehost})
		ensureRouteIsAdmittedByIngressController(oc, ns, routeName, "default")

		g.By("4.0: Curl the http route, make sure one server is hit")
		curlCmd := fmt.Sprintf(`curl http://%s -s -c %s --connect-timeout 10`, routehost, cookie)
		expectOutput := []string{"Hello-OpenShift"}
		repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)

		g.By("5.0: Check the cookies which should not contain a OPENSHIFT prefix or a md5 hash")
		cmd := fmt.Sprintf(`cat %s`, cookie)
		cookiesOutput, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(cookiesOutput)).NotTo(o.ContainSubstring("OPENSHIFT"))
		o.Expect(string(cookiesOutput)).NotTo(o.MatchRegexp(`\d+\.\d+\.\d+\.\d+`))
	})

	g.It("[OTP] [OCP-15872] can set cookie name for unsecure routes by annotation [apigroup:route.openshift.io]", func() {
		if !isCanaryRouteAvailable(oc) {
			g.Skip("Skip for the ingress canary route could not be available to the outside")
		}

		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-deploy.yaml")
			srvrcInfo           = "web-server-deploy"
			unsecSvcName        = "service-unsecure"
		)

		g.By("1.0: Create two server pods and the service")
		updateFilebySedCmd(testPodSvc, "replicas: 1", "replicas: 2")
		ns := oc.Namespace()
		srvPodList := createResourceFromWebServer(oc, ns, testPodSvc, srvrcInfo)

		g.By("2.0: Create a plain HTTP route")
		routehost := "unsecure15872" + ".apps." + getBaseDomain(oc)
		createRouteOTP(oc, ns, "http", unsecSvcName, unsecSvcName, []string{"--hostname=" + routehost})
		ensureRouteIsAdmittedByIngressController(oc, ns, unsecSvcName, "default")

		g.By("3.0: Set a cookie name to the route by the annotation, and then ensure the change in haproxy")
		setAnnotation(oc, ns, "route/"+unsecSvcName, "router.openshift.io/cookie_name=unsecure-cookie_1")
		routerPod := getOneRouterPodNameByIC(oc, "default")
		ensureHaproxyBlockConfigContains(oc, routerPod, "be_http:"+ns+":"+unsecSvcName, []string{"unsecure-cookie_1"})

		g.By("4.0: Curl the http route, make sure the second server is hit and the cookie is set in the client side")
		curlCmd := fmt.Sprintf(`curl http://%s -sv --connect-timeout 10`, routehost)
		expectOutput := []string{"Hello-OpenShift " + srvPodList[1] + " http-8080"}
		output, _ := repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)
		o.Expect(output).To(o.MatchRegexp(`(s|S)et-(c|C)ookie: unsecure-cookie_1=[0-9a-zA-Z]+`))
	})

	g.It("[OTP] [OCP-35547] cookie-same-site route annotation accepts None Lax or Strict attribute for edge routes [apigroup:route.openshift.io]", func() {
		if !isCanaryRouteAvailable(oc) {
			g.Skip("Skip for the ingress canary route could not be available to the outside")
		}

		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-deploy.yaml")
			srvrcInfo           = "web-server-deploy"
			unsecSvcName        = "service-unsecure"
		)

		g.By("1.0: Create two server pods and the service")
		ns := oc.Namespace()
		createResourceFromWebServer(oc, ns, testPodSvc, srvrcInfo)

		g.By("2.0: Create an edge route")
		routehost := "edge35547" + ".apps." + getBaseDomain(oc)
		createRouteOTP(oc, ns, "edge", "edge35547", unsecSvcName, []string{"--hostname=" + routehost})
		ensureRouteIsAdmittedByIngressController(oc, ns, "edge35547", "default")

		g.By("3.0: Curl the edge route, and check set-cookie header, expect getting SameSite=None")
		curlCmd := fmt.Sprintf(`curl https://%s -sSkI  --connect-timeout 10`, routehost)
		expectOutput := []string{"set-cookie:"}
		result, _ := repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)
		o.Expect(result).To(o.ContainSubstring(`Secure; SameSite=None`))

		g.By("4.0: Add Strict annotation to the edge route, and then ensure the change in haproxy")
		setAnnotation(oc, ns, "route/edge35547", "router.openshift.io/cookie-same-site=Strict")
		routerPod := getOneRouterPodNameByIC(oc, "default")
		ensureHaproxyBlockConfigContains(oc, routerPod, "be_edge_http:"+ns+":edge35547", []string{"SameSite=Strict"})

		g.By("5.0: Curl the edge route again, and check set-cookie header, expect getting SameSite=Strict")
		result, _ = repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)
		o.Expect(result).To(o.ContainSubstring(`Secure; SameSite=Strict`))
	})

	g.It("[OTP] [OCP-35548] cookie-same-site route annotation accepts None Lax or Strict attribute for Reencrypt routes [apigroup:route.openshift.io]", func() {
		if !isCanaryRouteAvailable(oc) {
			g.Skip("Skip for the ingress canary route could not be available to the outside")
		}

		var (
			buildPruningBaseDir = exutil.FixturePath("testdata", "router")
			testPodSvc          = filepath.Join(buildPruningBaseDir, "web-server-signed-deploy.yaml")
			srvrcInfo           = "web-server-deploy"
			secsvcName          = "service-secure"
		)

		g.By("1.0: Create two server pods and the service")
		ns := oc.Namespace()
		createResourceFromWebServer(oc, ns, testPodSvc, srvrcInfo)

		g.By("2.0: Create a reencrypt route")
		routehost := "reen35548" + ".apps." + getBaseDomain(oc)
		createRouteOTP(oc, ns, "reencrypt", "reen35548", secsvcName, []string{"--hostname=" + routehost})
		ensureRouteIsAdmittedByIngressController(oc, ns, "reen35548", "default")

		g.By("3.0: Curl the reencrypt route, and check set-cookie header, expect getting SameSite=None")
		curlCmd := fmt.Sprintf(`curl https://%s -sSkI  --connect-timeout 10`, routehost)
		expectOutput := []string{"set-cookie:"}
		result, _ := repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)
		o.Expect(result).To(o.ContainSubstring(`Secure; SameSite=None`))

		g.By("4.0: Add Lax annotation to the reencrypt route, and then ensure the change in haproxy")
		setAnnotation(oc, ns, "route/reen35548", "router.openshift.io/cookie-same-site=Lax")
		routerPod := getOneRouterPodNameByIC(oc, "default")
		ensureHaproxyBlockConfigContains(oc, routerPod, "be_secure:"+ns+":reen35548", []string{"SameSite=Lax"})

		g.By("5.0: Curl the reencrypt route again, and check set-cookie header, expect getting SameSite=Lax")
		result, _ = repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)
		o.Expect(result).To(o.ContainSubstring(`Secure; SameSite=Lax`))

		g.By("6.0: Add Strict annotation to the reencrypt route, and then ensure the change in haproxy")
		setAnnotation(oc, ns, "route/reen35548", "router.openshift.io/cookie-same-site=Strict")
		ensureHaproxyBlockConfigContains(oc, routerPod, "be_secure:"+ns+":reen35548", []string{"SameSite=Strict"})

		g.By("7.0: Curl the reencrypt route for the 3rd time, and check set-cookie header, expect getting SameSite=Strict")
		result, _ = repeatCmdOnClient(oc, curlCmd, expectOutput, 60, 1)
		o.Expect(result).To(o.ContainSubstring(`Secure; SameSite=Strict`))
	})
})
