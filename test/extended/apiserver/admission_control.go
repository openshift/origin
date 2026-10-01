package apiserver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Admission control tests: exercise API server admission behavior (patch semantics,
// namespace/project admission latency, SubjectAccessReview enforcement, CRD/CR watch
// lifecycle across admission changes, and APIServer CRD field validation).
var _ = g.Describe("[sig-api-machinery] API_Server", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("default")

	// author: zxiao@redhat.com
	// This case is for bug 1297910
	g.It("[OTP][OCP-09853] patch operation should use patched object to check admission control", ote.Informing(), func() {
		g.By("1) Create new project required for this test execution")
		oc.SetupProject()
		namespace := oc.Namespace()

		g.By("2) Use admin user to create quota and limits for project")

		g.By("2.1) Create quota")
		template := apiserverAuthFixture("ocp9853-quota.yaml")
		err := oc.AsAdmin().Run("create").Args("-f", template, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2.2) Create limits")
		template = apiserverAuthFixture("ocp9853-limits.yaml")
		err = oc.AsAdmin().Run("create").Args("-f", template, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(`2.3) Create pod and wait for "hello-openshift" pod to be ready`)
		template = apiserverAuthFixture("hello-pod.json")
		err = oc.Run("create").Args("-f", template, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		podName := "hello-openshift"
		compat_otp.AssertPodToBeReady(oc, podName, namespace)

		g.By("3) Update pod's image using patch command")
		patch := `{"spec":{"containers":[{"name":"hello-openshift","image":"quay.io/openshifttest/hello-openshift:1.2.0"}]}}`
		output, err := oc.Run("patch").Args("pod", podName, "-n", namespace, "-p", patch).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("4) Check if pod running")
		compat_otp.AssertPodToBeReady(oc, podName, namespace)
	})

	// author: rgangwar@redhat.com
	// Longduration: loops 15x creating/deleting a namespace+app, asserting the create-to-ready-to-delete
	// window stays under 90s to catch raft/cache delay regressions in namespace admission.
	g.It("[OTP][OCP-10350] compensate for raft/cache delay in namespace admission", ote.Informing(), func() {
		tmpnamespace := "ocp-10350" + compat_otp.GetRandomString()
		defer oc.AsAdmin().Run("delete").Args("ns", tmpnamespace, "--ignore-not-found").Execute()
		g.By("1.) Create new namespace")
		// Description of case: we observe how long it takes to delete one Terminating namespace that has a
		// Terminating pod when the cluster is under some load. Wait up to 200 seconds and also calculate the
		// actual time so that when it FIRST hits > 200 seconds, we fail it IMMEDIATELY. This way we know the
		// actual time DIRECTLY from the test logs, useful to file a performance bug with PRESENT evidence.
		expectedOutageTime := 90
		for i := 0; i < 15; i++ {
			var namespaceErr error
			projectSuccTime := time.Now()
			err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				namespaceOutput, namespaceErr := oc.WithoutNamespace().Run("create").Args("ns", tmpnamespace).Output()
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
				apperr := oc.WithoutNamespace().Run("new-app").Args("quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83", "-n", tmpnamespace, "--import-mode=PreserveOriginal").Execute()
				if apperr != nil {
					return false, nil
				}
				e2e.Logf("oc new app succeeded")
				return true, nil
			})
			compat_otp.AssertWaitPollNoErr(errApp, fmt.Sprintf("oc new app failed :: %v", apperr))

			var poderr error
			errPod := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				podOutput, poderr := oc.WithoutNamespace().Run("get").Args("pod", "-n", tmpnamespace, "--no-headers").Output()
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
				chkNamespaceOutput, chkNamespaceErr := oc.WithoutNamespace().Run("get").Args("namespace", tmpnamespace, "--ignore-not-found").Output()
				if chkNamespaceErr == nil && strings.Contains(chkNamespaceOutput, "") {
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

	// author: zxiao@redhat.com
	g.It("[OTP][OCP-22565] Check if the given user or group have the privilege via SubjectAccessReview [origin_platformexp_214][REST]", ote.Informing(), func() {
		isExternalOIDCCluster, err := compat_otp.IsExternalOIDCCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if isExternalOIDCCluster {
			g.Skip("Skipping the test as we are running against an external OIDC cluster.")
		}

		g.By("1) Create new project required for this test execution")
		oc.SetupProject()
		namespace := oc.Namespace()
		username := oc.Username()

		// helper function for executing post request to SubjectAccessReview
		postSubjectAccessReview := func(username string, namespace string, step string, expectStatus string) {
			g.By(fmt.Sprintf("%s>>) Get base URL for API requests", step))
			baseURL, err := oc.Run("whoami").Args("--show-server").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("%s>>) Get access token", step))
			token, err := oc.Run("whoami").Args("-t").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			authHeader := fmt.Sprintf(`Authorization: Bearer %s`, token)

			g.By(fmt.Sprintf("%s>>) Submit POST request to API SubjectAccessReview", step))
			url := baseURL + filepath.Join("/apis/authorization.openshift.io/v1/namespaces", namespace, "localsubjectaccessreviews")
			e2e.Logf("Get post SubjectAccessReview REST API server %s", url)

			postMap := map[string]string{
				"kind":       "LocalSubjectAccessReview",
				"apiVersion": "authorization.openshift.io/v1",
				"verb":       "create",
				"resource":   "pods",
				"user":       username,
			}
			postJSON, err := json.Marshal(postMap)
			o.Expect(err).NotTo(o.HaveOccurred())

			command := fmt.Sprintf("curl -X POST %s -w '%s' -o /dev/null -k -H '%s' -H 'Content-Type: application/json' -d '%s'", url, "%{http_code}", authHeader, string(postJSON))
			postSubjectAccessReviewStatus, err := exec.Command("bash", "-c", command).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(string(postSubjectAccessReviewStatus)).To(o.Equal(expectStatus))
		}

		// setup role for user and post to API
		testUserAccess := func(role string, step string, expectStatus string) {
			g.By(fmt.Sprintf("%s>>) Remove default role [admin] from the current user [%s]", step, username))
			errAdmRole := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 30*time.Second, false, func(cxt context.Context) (bool, error) {
				rolebindingOutput, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("rolebinding/admin", "-n", namespace, "--no-headers", "-oname").Output()
				if rolebindingOutput == "rolebinding.rbac.authorization.k8s.io/admin" {
					policyerr := oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-role-from-user", "admin", username, "-n", namespace).Execute()
					if policyerr != nil {
						return false, nil
					}
				}
				return true, nil
			})
			rolebindingOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("rolebinding", "-n", namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Rolebinding of user %v :: %v", username, rolebindingOutput)
			compat_otp.AssertWaitPollNoErr(errAdmRole, fmt.Sprintf("Not able to delete admin role for user :: %v :: %v", username, errAdmRole))

			g.By(fmt.Sprintf("%s>>) Add new role [%s] to the current user [%s]", step, role, username))
			err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-role-to-user", role, username, "-n", namespace).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("%s>>) POST to SubjectAccessReview API for user %s under namespace %s, expect status %s", step, username, namespace, expectStatus))
			postSubjectAccessReview(username, namespace, step, expectStatus)
		}

		g.By("2) Test user access with role [view], expect failure")
		testUserAccess("view", "2", "403")

		g.By("3) Test user access with role [edit], expect failure")
		testUserAccess("edit", "3", "403")

		g.By("4) Test user access with role [admin], expect success")
		testUserAccess("admin", "4", "201")
	})

	// author: zxiao@redhat.com
	// Longduration/Disruptive: modifies a CRD in-flight and observes watch behavior across the change.
	g.It("[OTP][OCP-24219] Custom resource watchers should terminate instead of hang when its CRD is deleted or modified [Disruptive]", ote.Informing(), func() {
		g.By("1) Create a new project required for this test execution")
		projectName := fmt.Sprintf("ocp24219-%d", time.Now().UnixNano())
		oc.AsAdmin().Run("new-project").Args(projectName).Execute()
		defer oc.AsAdmin().Run("delete").Args("project", projectName).Execute()
		namespace := projectName

		crdTemplate := apiserverAuthFixture("ocp24219-crd.yaml")
		g.By(fmt.Sprintf("2) Apply custom resource definition from file %s", crdTemplate))
		oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crdTemplate).Execute()
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", crdTemplate, "--ignore-not-found=true").Execute()
			oc.AsAdmin().WithoutNamespace().Run("wait").Args("--for=delete", "crd/testcrs.example.com", "--timeout=60s").Execute()
		}()

		crTemplate := apiserverAuthFixture("ocp24219-cr.yaml")
		g.By(fmt.Sprintf("3) Apply custom resource from file %s", crTemplate))
		oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crTemplate, "-n", namespace).Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", crTemplate, "-n", namespace, "--ignore-not-found=true").Execute()

		resourcePath := fmt.Sprintf("/apis/example.com/v1/namespaces/%s/testcrs", namespace)
		g.By(fmt.Sprintf("4) Start watching custom resource at %s", resourcePath))
		cmd1, backgroundBuf, _, err := oc.AsAdmin().Run("get").Args(fmt.Sprintf("--raw=%s?watch=True", resourcePath), "-n", namespace).Background()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			cmd1.Process.Kill()
			cmd1.Wait()
		}()
		time.Sleep(5 * time.Second) // wait for watch to start

		g.By("5) Modify custom resource and apply change")
		crTemplateCopy := copyToFile(crTemplate, fmt.Sprintf("ocp24219-cr-copy-%d.yaml", time.Now().UnixNano()))
		compat_otp.ModifyYamlFileContent(crTemplateCopy, []compat_otp.YamlReplace{
			{
				Path:  "spec.a",
				Value: "This change to the CR results in a MODIFIED event",
			},
		})
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crTemplateCopy, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("6) Validate that CR modification event is received")
		o.Eventually(func() bool {
			return strings.Contains(backgroundBuf.String(), "MODIFIED")
		}, 2*time.Minute, 2*time.Second).Should(o.BeTrue(), "MODIFIED event not detected")

		g.By("7) Modify the CRD and apply change")
		crdTemplateCopy := copyToFile(crdTemplate, fmt.Sprintf("ocp24219-crd-copy-%d.yaml", time.Now().UnixNano()))
		compat_otp.ModifyYamlFileContent(crdTemplateCopy, []compat_otp.YamlReplace{
			{
				Path:  "spec.versions.0.schema.openAPIV3Schema.properties.spec.properties",
				Value: "b:\n  type: string",
			},
		})
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crdTemplateCopy).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("8) Start a second watch after CRD modification")
		cmd2, backgroundBuf2, _, err := oc.AsAdmin().Run("get").Args(fmt.Sprintf("--raw=%s?watch=True", resourcePath), "-n", namespace).Background()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			cmd2.Process.Kill()
			cmd2.Wait()
		}()
		time.Sleep(5 * time.Second) // allow second watch to start

		crName := "ocp24219-test-cr"
		crdFullName := "crd/testcrs.example.com"
		kind := "OCP24219TestCR"

		g.By("9) Ensure CR exists before CRD deletion")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, crName, "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Expected the CR to exist before deleting CRD")

		g.By("10) Delete the CRD")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(crdFullName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("11) Verify CR deletion event after CRD is deleted")
		crDeleteMatchRegex, err := regexp.Compile(`"type":"DELETED".*"object":.*"kind":"` + kind + `"`)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() bool {
			match := crDeleteMatchRegex.MatchString(backgroundBuf2.String())
			if !match {
				e2e.Logf("DEBUG: Buffer after CRD deletion: %s", backgroundBuf2.String())
			}
			return match
		}, 2*time.Minute, 2*time.Second).Should(o.BeTrue(), "CR deletion event not found in watch stream")
	})

	// author: dpunia@redhat.com
	// Longduration/Disruptive: applies an invalid apiserver CR config, then patches to a valid one and waits for kube-apiserver to roll out.
	g.It("[OTP][OCP-24389] Verify the CR admission of the APIServer CRD [Slow][Disruptive]", ote.Informing(), func() {
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

		g.By("1) Update apiserver config(additionalCORSAllowedOrigins) with invalid config `no closing (parentheses`")
		patch := `{"spec": {"additionalCORSAllowedOrigins": ["no closing (parentheses"]}}`
		patchOut, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver", "cluster", "--type=merge", "-p", patch).Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(patchOut).Should(o.ContainSubstring(`"no closing (parentheses": not a valid regular expression`))

		g.By("2) Update apiserver config(additionalCORSAllowedOrigins) with invalid string type")
		patch = `{"spec": {"additionalCORSAllowedOrigins": "some string"}}`
		patchOut, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver", "cluster", "--type=merge", "-p", patch).Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(patchOut).Should(o.ContainSubstring(`body must be of type array: "string"`))

		g.By("3) Update apiserver config(additionalCORSAllowedOrigins) with valid config")
		patchOut, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver", "cluster", "--type=merge", "-p", patchJson).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		o.Expect(patchOut).Should(o.ContainSubstring("patched"))
		// Wait for kube-apiserver recover
		apiServerRecover()

		g.By("4) Verifying the additionalCORSAllowedOrigins by inspecting the HTTP response headers")
		urlStr, err := oc.Run("whoami").Args("--show-server").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		req, err := http.NewRequest("GET", urlStr, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		req.Header.Set("Origin", "http://localhost")

		tr := &http.Transport{}
		if os.Getenv("HTTPS_PROXY") != "" || os.Getenv("https_proxy") != "" {
			httpsProxyURL, err := url.Parse(os.Getenv("https_proxy"))
			o.Expect(err).NotTo(o.HaveOccurred())
			tr.Proxy = http.ProxyURL(httpsProxyURL)
		}
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client := &http.Client{
			Transport: tr,
			Timeout:   time.Second * 30,
		}

		resp, err := client.Do(req)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer resp.Body.Close()
		o.Expect(resp.Header.Get("Access-Control-Allow-Origin")).To(o.Equal("http://localhost"))
	})
})
