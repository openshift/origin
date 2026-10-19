package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	utilimage "github.com/openshift/origin/test/extended/util/image"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Admission control tests: exercise API server admission behavior (patch semantics,
// namespace/project admission latency, SubjectAccessReview enforcement, CRD/CR watch
// lifecycle across admission changes, and APIServer CRD field validation).
var _ = g.Describe("[sig-api-machinery] API_Server", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("default")

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Admission-control tests requiring OpenShift control-plane APIs are not supported on MicroShift")
		}
	})

	// Longduration: loops 15x creating/deleting a namespace+app, asserting the create-to-ready-to-delete
	// window stays under 90s to catch raft/cache delay regressions in namespace admission.
	g.It("[OTP][OCP-10350] Verify whether raft/cache delay is compensated in namespace admission", ote.Informing(), func() {
		tmpnamespace := "ocp-10350" + compat_otp.GetRandomString()
		defer oc.AsAdmin().Run("delete").Args("ns", tmpnamespace, "--ignore-not-found").Execute()
		g.By("1.) Create new namespace")
		expectedOutageTime := 90
		for i := 0; i < 15; i++ {
			var namespaceErr error
			projectSuccTime := time.Now()
			err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				var namespaceOutput string
				namespaceOutput, namespaceErr = oc.WithoutNamespace().Run("create").Args("ns", tmpnamespace).Output()
				if namespaceErr == nil {
					e2e.Logf("oc create ns %v created successfully", tmpnamespace)
					projectSuccTime = time.Now()
					o.Expect(namespaceOutput).Should(o.ContainSubstring(fmt.Sprintf("namespace/%v created", tmpnamespace)), fmt.Sprintf("namespace/%v not created", tmpnamespace))
					return true, nil
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("oc create ns %v failed :: %v", tmpnamespace, namespaceErr))

			g.By("2.) Create new app")
			var apperr error
			errApp := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				apperr = oc.WithoutNamespace().Run("new-app").Args(utilimage.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.55"), "-n", tmpnamespace, "--import-mode=PreserveOriginal").Execute()
				if apperr != nil {
					return false, nil
				}
				e2e.Logf("oc new app succeeded")
				return true, nil
			})
			compat_otp.AssertWaitPollNoErr(errApp, fmt.Sprintf("oc new app failed :: %v", apperr))

			var poderr error
			errPod := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				var podOutput string
				podOutput, poderr = oc.WithoutNamespace().Run("get").Args("pod", "-n", tmpnamespace, "--no-headers").Output()
				if poderr == nil && strings.Contains(podOutput, "Running") {
					e2e.Logf("Pod %v succesfully", podOutput)
					return true, nil
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(errPod, fmt.Sprintf("Pod not running :: %v", poderr))

			g.By("3.) Delete new namespace")
			var delerr error
			projectdelerr := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				delerr = oc.Run("delete").Args("namespace", tmpnamespace).Execute()
				if delerr != nil {
					return false, nil
				}
				e2e.Logf("oc delete namespace succeeded")
				return true, nil
			})
			compat_otp.AssertWaitPollNoErr(projectdelerr, fmt.Sprintf("oc delete namespace failed :: %v", delerr))

			var chkNamespaceErr error
			errDel := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				var chkNamespaceOutput string
				chkNamespaceOutput, chkNamespaceErr = oc.WithoutNamespace().Run("get").Args("namespace", tmpnamespace, "--ignore-not-found").Output()
				if chkNamespaceErr == nil && strings.TrimSpace(chkNamespaceOutput) == "" {
					e2e.Logf("Namespace deleted %v successfully", tmpnamespace)
					return true, nil
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(errDel, fmt.Sprintf("Namespace %v not deleted successfully, still visible after delete :: %v", tmpnamespace, chkNamespaceErr))

			projectDelTime := time.Now()
			diff := projectDelTime.Sub(projectSuccTime)
			e2e.Logf("#### Namespace success and delete time(s) :: %f ####\n", diff.Seconds())
			if int(diff.Seconds()) > expectedOutageTime {
				e2e.Failf("#### Test case Failed in %d run :: The Namespace success and deletion outage time lasted %d longer than we expected %d", i, int(diff.Seconds()), expectedOutageTime)
			}
			e2e.Logf("#### Test case passed in %d run :: Namespace success and delete time(s) :: %f ####\n", i, diff.Seconds())
		}
	})

	// Longduration/Disruptive: Tests that CR watches terminate gracefully when the CRD is modified or deleted.
	// Upstream crd_watch.go covers CR lifecycle events (Added/Modified/Deleted).
	// This test focuses on watch behavior when the CRD itself changes.
	g.It("[OTP][OCP-24219] Custom resource watchers should terminate instead of hang when its CRD is deleted or modified [Serial]", ote.Informing(), func() {
		g.By("1) Create a new project")
		projectName := fmt.Sprintf("ocp24219-%d", time.Now().UnixNano())
		err := oc.AsAdmin().Run("new-project").Args(projectName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().Run("delete").Args("project", projectName).Execute()
		namespace := projectName

		crdTemplate := apiserverAuthFixture("ocp24219-crd.yaml")
		g.By("2) Create CRD and wait for it to become established")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crdTemplate).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", crdTemplate, "--ignore-not-found=true").Execute()
			oc.AsAdmin().WithoutNamespace().Run("wait").Args("--for=delete", "crd/testcrs.example.com", "--timeout=60s").Execute()
		}()

		err = oc.AsAdmin().WithoutNamespace().Run("wait").Args("--for=condition=Established", "crd/testcrs.example.com", "--timeout=60s").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		crTemplate := apiserverAuthFixture("ocp24219-cr.yaml")
		g.By("3) Create a custom resource instance")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crTemplate, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", crTemplate, "-n", namespace, "--ignore-not-found=true").Execute()

		resourcePath := fmt.Sprintf("/apis/example.com/v1/namespaces/%s/testcrs", namespace)
		crName := "ocp24219-test-cr"
		crdFullName := "crd/testcrs.example.com"
		kind := "OCP24219TestCR"

		g.By("4) Modify the CRD schema")
		crdTemplateCopy := copyToFile(crdTemplate, fmt.Sprintf("ocp24219-crd-copy-%d.yaml", time.Now().UnixNano()))
		compat_otp.ModifyYamlFileContent(crdTemplateCopy, []compat_otp.YamlReplace{
			{
				Path:  "spec.versions.0.schema.openAPIV3Schema.properties.spec.properties",
				Value: "b:\n  type: string",
			},
		})
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crdTemplateCopy).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("5) Start watching CRs after CRD modification (verifies watch can start after schema change)")
		cmd, backgroundBuf, _, err := oc.AsAdmin().Run("get").Args(fmt.Sprintf("--raw=%s?watch=True", resourcePath), "-n", namespace).Background()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			cmd.Process.Kill()
			cmd.Wait()
		}()
		time.Sleep(5 * time.Second) // allow watch to start

		g.By("6) Verify CR still exists before CRD deletion")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, crName, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "CR should exist before CRD deletion")

		g.By("7) Delete the CRD (this should trigger watch termination)")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(crdFullName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("8) Verify watch terminates gracefully with CR DELETED event (not hanging)")
		crDeleteMatchRegex, err := regexp.Compile(`"type":"DELETED".*"object":.*"kind":"` + kind + `"`)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() bool {
			match := crDeleteMatchRegex.MatchString(backgroundBuf.String())
			if !match {
				e2e.Logf("DEBUG: Watch buffer after CRD deletion: %s", backgroundBuf.String())
			}
			return match
		}, 2*time.Minute, 2*time.Second).Should(o.BeTrue(), "Watch should receive CR DELETED event when CRD is deleted, not hang")
	})

	// Longduration/Disruptive: applies an invalid apiserver CR config, then patches to a valid one and waits for kube-apiserver to roll out.
	g.It("[OTP][OCP-24389] Verify the CR admission of the APIServer CRD [APIServer_Disruptive][Slow][Disruptive]", ote.Informing(), func() {
		var (
			patchOut        string
			patchJsonRevert = `{"spec": {"additionalCORSAllowedOrigins": null}}`
			patchJson       = `{
			"spec": {
				"additionalCORSAllowedOrigins": [
				"(?i)//127\\.0\\.0\\.1(:|\\z)",
				"(?i)//localhost(:|\\z)",
				"(?i)//kubernetes\\.default(:|\\z)",
				"(?i)//kubernetes\\.default\\.svc\\.cluster\\.local(:|\\z)",
				"(?i)//kubernetes(:|\\z)",
				"(?i)//openshift\\.default(:|\\z)",
				"(?i)//openshift\\.default\\.svc(:|\\z)",
				"(?i)//openshift\\.default\\.svc\\.cluster\\.local(:|\\z)",
				"(?i)//kubernetes\\.default\\.svc(:|\\z)",
				"(?i)//openshift(:|\\z)"
			]}}`
		)

		g.By("Check if it's a proxy cluster")
		httpProxy, httpsProxy, _ := getGlobalProxy(oc)
		if strings.Contains(httpProxy, "http") || strings.Contains(httpsProxy, "https") {
			g.Skip("Skip for proxy platform")
		}

		apiServerRecover := func() {
			errKASO := waitCoBecomes(oc, "kube-apiserver", 100, map[string]string{"Progressing": "True"})
			compat_otp.AssertWaitPollNoErr(errKASO, "kube-apiserver operator is not start progressing in 100 seconds")
			e2e.Logf("Checking kube-apiserver operator should be Available in 1500 seconds")
			errKASO = waitCoBecomes(oc, "kube-apiserver", 1500, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"})
			compat_otp.AssertWaitPollNoErr(errKASO, "openshift-kube-apiserver pods revisions recovery not completed")
		}

		defer func() {
			if strings.Contains(patchOut, "patched") {
				err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver", "cluster", "--type=merge", "-p", patchJsonRevert).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				// Wait for kube-apiserver recover
				apiServerRecover()
			}
		}()

		g.By("1) Update apiserver config(additionalCORSAllowedOrigins) with invalid string type")
		patch := `{"spec": {"additionalCORSAllowedOrigins": "some string"}}`
		patchOut, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver", "cluster", "--type=merge", "-p", patch).Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(patchOut).Should(o.ContainSubstring(`body must be of type array: "string"`))

		g.By("2) Update apiserver config(additionalCORSAllowedOrigins) with valid config")
		patchOut, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver", "cluster", "--type=merge", "-p", patchJson).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		o.Expect(patchOut).Should(o.ContainSubstring("patched"))
		// Wait for kube-apiserver recover
		apiServerRecover()

		g.By("3) Verifying the additionalCORSAllowedOrigins by inspecting the HTTP response headers")
		urlStr, err := oc.Run("whoami").Args("--show-server").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		restConfig := oc.KubeFramework().ClientConfig()
		restConfig.Timeout = 30 * time.Second
		client, err := rest.HTTPClientFor(restConfig)
		o.Expect(err).NotTo(o.HaveOccurred())

		req, err := http.NewRequest("GET", urlStr, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		req.Header.Set("Origin", "http://localhost")

		resp, err := client.Do(req)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer resp.Body.Close()
		o.Expect(resp.Header.Get("Access-Control-Allow-Origin")).To(o.Equal("http://localhost"))
	})
})
