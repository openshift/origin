package apiserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/openshift/origin/test/extended/util/compat_otp/architecture"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Feature:Webhooks]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver-webhooks")
	var tmpdir string

	g.BeforeEach(func() {
		tmpdir = filepath.Join(os.TempDir(), "apiserver-webhooks-"+compat_otp.GetRandomString()+"/")
		o.Expect(os.MkdirAll(tmpdir, 0755)).To(o.Succeed())
	})

	g.AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	setupOPAWebhook := func(webhookConfigFile string) {
		skipIfBaselineCapsMissingCapabilities(oc, "Build", "DeploymentConfig")

		errNS := oc.WithoutNamespace().AsAdmin().Run("delete").Args("ns", "opa", "--ignore-not-found").Execute()
		o.Expect(errNS).NotTo(o.HaveOccurred())

		var (
			caKeypem          = tmpdir + "/caKey.pem"
			caCertpem         = tmpdir + "/caCert.pem"
			serverKeypem      = tmpdir + "/serverKey.pem"
			serverconf        = tmpdir + "/server.conf"
			serverWithSANcsr  = tmpdir + "/serverWithSAN.csr"
			serverCertWithSAN = tmpdir + "/serverCertWithSAN.pem"
		)

		g.By("Check if it's a proxy cluster with techpreview")
		featureTech, err := getResource(oc, asAdmin, withoutNamespace, "featuregate", "cluster", "-o=jsonpath={.spec.featureSet}")
		o.Expect(err).NotTo(o.HaveOccurred())
		httpProxy, _, _ := getGlobalProxy(oc)
		if strings.Contains(httpProxy, "http") && strings.Contains(featureTech, "TechPreview") {
			g.Skip("Skip for proxy platform with techpreview")
		}

		architecture.SkipNonAmd64SingleArch(oc)

		g.By("Create certificates with SAN")
		for _, cmd := range []string{
			fmt.Sprintf("openssl genrsa -out %v 2048", caKeypem),
			fmt.Sprintf(`openssl req -x509 -new -nodes -key %v -days 100000 -out %v -subj "/CN=wb_ca"`, caKeypem, caCertpem),
			fmt.Sprintf("openssl genrsa -out %v 2048", serverKeypem),
		} {
			_, err := exec.Command("bash", "-c", cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		serverconfCMD := fmt.Sprintf(`cat > %v << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
IP.1 = 127.0.0.1
DNS.1 = opa.opa.svc
EOF`, serverconf)
		_, err = exec.Command("bash", "-c", serverconfCMD).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, cmd := range []string{
			fmt.Sprintf(`openssl req -new -key %v -out %v -subj "/CN=opa.opa.svc" -config %v`, serverKeypem, serverWithSANcsr, serverconf),
			fmt.Sprintf(`openssl x509 -req -in %v -CA %v -CAkey %v -CAcreateserial -out %v -days 100000 -extensions v3_req -extfile %s`, serverWithSANcsr, caCertpem, caKeypem, serverCertWithSAN, serverconf),
		} {
			_, err := exec.Command("bash", "-c", cmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Create new secret with SAN cert")
		opaOutput, opaerr := oc.Run("create").Args("namespace", "opa").Output()
		o.Expect(opaerr).NotTo(o.HaveOccurred())
		o.Expect(opaOutput).Should(o.ContainSubstring("namespace/opa created"))
		_, opaerr = oc.Run("create").Args("secret", "tls", "opa-server", "--cert="+serverCertWithSAN, "--key="+serverKeypem, "-n", "opa").Output()
		o.Expect(opaerr).NotTo(o.HaveOccurred())

		g.By("Create admission webhook")
		policyOutput, policyerr := oc.WithoutNamespace().Run("adm").Args("policy", "add-scc-to-user", "privileged", "-z", "default", "-n", "opa").Output()
		o.Expect(policyerr).NotTo(o.HaveOccurred())
		o.Expect(policyOutput).Should(o.ContainSubstring(`clusterrole.rbac.authorization.k8s.io/system:openshift:scc:privileged added: "default"`))

		admissionTemplate := apiserverAuthFixture("ocp55494-admission-controller.yaml")
		admissionOutput, admissionerr := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", admissionTemplate).Output()
		o.Expect(admissionerr).NotTo(o.HaveOccurred())
		admissionOutput1 := regexp.MustCompile(`\n`).ReplaceAllString(admissionOutput, "")
		admissionOutput2 := `clusterrolebinding.rbac.authorization.k8s.io/opa-viewer.*role.rbac.authorization.k8s.io/configmap-modifier.*rolebinding.rbac.authorization.k8s.io/opa-configmap-modifier.*service/opa.*deployment.apps/opa.*configmap/opa-default-system-main`
		o.Expect(admissionOutput1).Should(o.MatchRegexp(admissionOutput2))

		g.By("Create webhook with certificates with SAN")
		csrpemcmd := `cat ` + serverCertWithSAN + ` | base64 | tr -d '\n'`
		csrpemcert, csrpemErr := exec.Command("bash", "-c", csrpemcmd).Output()
		o.Expect(csrpemErr).NotTo(o.HaveOccurred())
		webhookTemplate := apiserverAuthFixture(webhookConfigFile)
		compat_otp.CreateClusterResourceFromTemplate(oc.NotShowInfo(), "--ignore-unknown-parameters=true", "-f", webhookTemplate, "-n", "opa", "-p", `SERVERCERT=`+string(csrpemcert))

		g.By("Wait for OPA deployment to be ready")
		opaPodsReadyErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
			opaPodsOutput, err := oc.Run("get").Args("-n", "opa", "pods", "-l", "app=opa", "-o", `jsonpath={.items[?(@.status.phase=="Running")].metadata.name}`).Output()
			if err != nil || strings.TrimSpace(opaPodsOutput) == "" {
				return false, nil
			}
			opaPodsReadyOutput, err := oc.Run("get").Args("-n", "opa", "pods", "-l", "app=opa", "-o", `jsonpath={.items[*].status.conditions[?(@.type=="Ready")].status}`).Output()
			if err != nil {
				return false, nil
			}
			for _, status := range strings.Fields(opaPodsReadyOutput) {
				if status != "True" {
					return false, nil
				}
			}
			return true, nil
		})
		compat_otp.AssertWaitPollNoErr(opaPodsReadyErr, "OPA deployment not ready")
	}

	g.It("[OTP][OCP-55494] When using webhooks fails to rollout latest deploymentconfig [Disruptive][apigroup:apps.openshift.io]",
		ote.Informing(), func() {
			randomStr := compat_otp.GetRandomString()
			dcpolicyrepo := tmpdir + "/dc-policy.repo"

			defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("ns", "opa", "--ignore-not-found").Execute()
			defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("ns", "test-ns"+randomStr, "--ignore-not-found").Execute()
			defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", "opa-validating-webhook", "--ignore-not-found").Execute()
			defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrolebinding.rbac.authorization.k8s.io/opa-viewer", "--ignore-not-found").Execute()
			defer oc.WithoutNamespace().AsAdmin().Run("adm").Args("policy", "remove-scc-from-user", "privileged", "-z", "default", "-n", "opa").Execute()

			setupOPAWebhook("ocp55494-webhook-configuration.yaml")

			g.By("Check rollout latest deploymentconfig")
			tmpnsOutput, tmpnserr := oc.Run("create").Args("ns", "test-ns"+randomStr).Output()
			o.Expect(tmpnserr).NotTo(o.HaveOccurred())
			o.Expect(tmpnsOutput).Should(o.ContainSubstring(fmt.Sprintf("namespace/test-ns%v created", randomStr)))

			tmplabelOutput, tmplabelerr := oc.Run("label").Args("ns", "test-ns"+randomStr, "openpolicyagent.org/webhook=ignore").Output()
			o.Expect(tmplabelerr).NotTo(o.HaveOccurred())
			o.Expect(tmplabelOutput).Should(o.ContainSubstring(fmt.Sprintf("namespace/test-ns%v labeled", randomStr)))

			var deployerr error
			deployconfigerr := wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
				_, deployerr = oc.WithoutNamespace().AsAdmin().Run("create").Args("deploymentconfig", "mydc", "--image", "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83", "-n", "test-ns"+randomStr).Output()
				return deployerr == nil, nil
			})
			compat_otp.AssertWaitPollNoErr(deployconfigerr, fmt.Sprintf("Not able to create mydc deploymentconfig :: %v", deployerr))

			waiterrRollout := wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
				rollOutput, _ := oc.WithoutNamespace().AsAdmin().Run("rollout").Args("latest", "dc/mydc", "-n", "test-ns"+randomStr).Output()
				return strings.Contains(rollOutput, "rolled out"), nil
			})
			compat_otp.AssertWaitPollNoErr(waiterrRollout, "deploymentconfig.apps.openshift.io/mydc not rolled out")

			g.By("Change configmap policy and rollout")
			dcpolicycmd := fmt.Sprintf(`cat > %v << EOF
package kubernetes.admission
deny[msg] {
    input.request.kind.kind == "DeploymentConfig"
    msg:= "No entry for you"
}
EOF`, dcpolicyrepo)
			_, dcpolicycmdErr := exec.Command("bash", "-c", dcpolicycmd).Output()
			o.Expect(dcpolicycmdErr).NotTo(o.HaveOccurred())
			dcpolicyOutput, dcpolicyerr := oc.WithoutNamespace().Run("create").Args("configmap", "dc-policy", `--from-file=`+dcpolicyrepo, "-n", "opa").Output()
			o.Expect(dcpolicyerr).NotTo(o.HaveOccurred())
			o.Expect(dcpolicyOutput).Should(o.ContainSubstring(`configmap/dc-policy created`))

			waiterrRollout = wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
				rollOutput, _ := oc.WithoutNamespace().AsAdmin().Run("rollout").Args("latest", "dc/mydc", "-n", "test-ns"+randomStr).Output()
				return strings.Contains(rollOutput, "No entry for you"), nil
			})
			compat_otp.AssertWaitPollNoErr(waiterrRollout, "deploymentconfig not rolled out with new policy")
		})

	g.It("[OTP][OCP-77919] HPA/oc scale and DeploymentConfig should be working with OPA webhooks [Disruptive][apigroup:apps.openshift.io]",
		ote.Informing(), func() {
			randomStr := compat_otp.GetRandomString()

			defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("ns", "opa", "--ignore-not-found").Execute()
			defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("ns", "test-ns"+randomStr, "--ignore-not-found").Execute()
			defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", "opa-validating-webhook", "--ignore-not-found").Execute()
			defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("clusterrolebinding.rbac.authorization.k8s.io/opa-viewer", "--ignore-not-found").Execute()
			defer oc.WithoutNamespace().AsAdmin().Run("adm").Args("policy", "remove-scc-from-user", "privileged", "-z", "default", "-n", "opa").Execute()

			setupOPAWebhook("ocp77919-webhook-configuration.yaml")

			g.By("Check rollout latest deploymentconfig")
			tmpnsOutput, tmpnserr := oc.Run("create").Args("ns", "test-ns"+randomStr).Output()
			o.Expect(tmpnserr).NotTo(o.HaveOccurred())
			o.Expect(tmpnsOutput).Should(o.ContainSubstring(fmt.Sprintf("namespace/test-ns%v created", randomStr)))

			tmplabelOutput, tmplabelErr := oc.Run("label").Args("ns", "test-ns"+randomStr, "openpolicyagent.org/webhook=ignore").Output()
			o.Expect(tmplabelErr).NotTo(o.HaveOccurred())
			o.Expect(tmplabelOutput).Should(o.ContainSubstring(fmt.Sprintf("namespace/test-ns%v labeled", randomStr)))

			var deployErr error
			deployConfigErr := wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
				_, deployErr = oc.WithoutNamespace().AsAdmin().Run("create").Args("deploymentconfig", "mydc", "--image", "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83", "-n", "test-ns"+randomStr).Output()
				return deployErr == nil, nil
			})
			compat_otp.AssertWaitPollNoErr(deployConfigErr, fmt.Sprintf("Not able to create mydc deploymentconfig :: %v", deployErr))

			waiterrRollout := wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
				rollOutput, _ := oc.WithoutNamespace().AsAdmin().Run("rollout").Args("latest", "dc/mydc", "-n", "test-ns"+randomStr).Output()
				return strings.Contains(rollOutput, "rolled out"), nil
			})
			compat_otp.AssertWaitPollNoErr(waiterrRollout, "deploymentconfig.apps.openshift.io/mydc not rolled out")

			g.By("Try to scale deployment config, oc scale should work without error")
			waitScaleErr := wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 120*time.Second, false, func(ctx context.Context) (bool, error) {
				scaleErr := oc.WithoutNamespace().AsAdmin().Run("scale").Args("dc/mydc", "--replicas=10", "-n", "test-ns"+randomStr).Execute()
				return scaleErr == nil, nil
			})
			compat_otp.AssertWaitPollNoErr(waitScaleErr, "deploymentconfig.apps.openshift.io/mydc not scaled out")
		})

	g.It("[OTP][OCP-53230] Kubernetes validating admission webhook bypass [Serial][apigroup:admissionregistration.k8s.io]",
		ote.Informing(), func() {
			skipIfProxyCluster(oc)

			g.By("Get a node name required by test")
			nodeName, getNodeErr := compat_otp.GetFirstMasterNode(oc)
			o.Expect(getNodeErr).NotTo(o.HaveOccurred())
			o.Expect(nodeName).NotTo(o.Equal(""))

			g.By("Create custom webhook and service")
			webhookDeployTemplate := apiserverAuthFixture("webhook-deploy.yaml")
			defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", webhookDeployTemplate).Execute()
			err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", webhookDeployTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			podName := getPodsList(oc.AsAdmin(), "validationwebhook")
			o.Expect(podName).NotTo(o.BeEmpty())
			compat_otp.AssertPodToBeReady(oc, podName[0], "validationwebhook")

			caBundle := execCommandOnPod(oc, podName[0], "validationwebhook", `cat /usr/src/app/ca.crt | base64 | tr -d "\n"`)
			o.Expect(caBundle).NotTo(o.BeEmpty())

			g.By("Register the above created webhook")
			webhookRegistrationTemplate := apiserverAuthFixture("webhook-registration.yaml")
			params := []string{"-n", "validationwebhook", "-f", webhookRegistrationTemplate, "-p", "NAME=validationwebhook.validationwebhook.svc", "NAMESPACE=validationwebhook", "CABUNDLE=" + caBundle}
			webhookRegistrationConfigFile := compat_otp.ProcessTemplate(oc, params...)
			defer func() {
				err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", webhookRegistrationConfigFile).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}()
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", webhookRegistrationConfigFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			parameters := []string{
				`{"changeAllowed": "false"}`,
				`{"changeAllowed": "true"}`,
			}

			for index, param := range parameters {
				g.By(fmt.Sprintf("Node label addition fails due to validation webhook denial %d", index+1))
				out, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("node", nodeName, "-p", fmt.Sprintf(`{"metadata": {"labels": %s}}`, param)).Output()
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("denied the request: Validation failed"))
			}
		})
})

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Feature:UpgradeWebhooks]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver-upgrade-webhooks")

	g.It("[OTP][OCP-50362] Verify cluster handles bad admission webhooks correctly [Serial][apigroup:config.openshift.io][apigroup:admissionregistration.k8s.io][apigroup:apiextensions.k8s.io]",
		ote.Informing(), func() {
			var (
				namespace                    = "ocp-50362-" + compat_otp.GetRandomString()
				serviceName                  = "example-service"
				serviceNamespace             = "example-namespace"
				badValidatingWebhookName     = "test-validating-cfg"
				badMutatingWebhookName       = "test-mutating-cfg"
				badCrdWebhookName            = "testcrdwebhooks.tests.com"
				kubeApiserverCoStatus        = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
				kasRolloutStatus             = map[string]string{"Available": "True", "Progressing": "True", "Degraded": "False"}
				webHookErrorConditionTypes   = []string{"ValidatingAdmissionWebhookConfigurationError", "MutatingAdmissionWebhookConfigurationError", "CRDConversionWebhookConfigurationError"}
				status                       = "True"
				webhookServiceFailureReasons = []string{"WebhookServiceNotFound", "WebhookServiceNotReady", "WebhookServiceConnectionError"}
			)

			validatingWebHook := admissionWebhook{
				name: badValidatingWebhookName, webhookname: "test.validating.com",
				servicenamespace: serviceNamespace, servicename: serviceName, namespace: namespace,
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				template: apiserverAuthFixture("ValidatingWebhookConfigurationTemplate.yaml"),
			}
			mutatingWebHook := admissionWebhook{
				name: badMutatingWebhookName, webhookname: "test.mutating.com",
				servicenamespace: serviceNamespace, servicename: serviceName, namespace: namespace,
				apigroups: "authorization.k8s.io", apiversions: "v1", operations: "*", resources: "subjectaccessreviews",
				template: apiserverAuthFixture("MutatingWebhookConfigurationTemplate.yaml"),
			}
			crdWebHook := admissionWebhook{
				name: badCrdWebhookName, webhookname: "tests.com",
				servicenamespace: serviceNamespace, servicename: serviceName, namespace: namespace,
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				template: apiserverAuthFixture("CRDWebhookConfigurationTemplate.yaml"),
			}

			defer func() {
				_ = oc.Run("delete").Args("ValidatingWebhookConfiguration", badValidatingWebhookName, "--ignore-not-found").Execute()
				_ = oc.Run("delete").Args("MutatingWebhookConfiguration", badMutatingWebhookName, "--ignore-not-found").Execute()
				_ = oc.Run("delete").Args("crd", badCrdWebhookName, "--ignore-not-found").Execute()
				_ = oc.WithoutNamespace().Run("delete").Args("project", namespace, "--ignore-not-found").Execute()
			}()

			kasStatusBefore := getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
			if !reflect.DeepEqual(kasStatusBefore, kubeApiserverCoStatus) && !reflect.DeepEqual(kasStatusBefore, kasRolloutStatus) {
				g.Skip("kube-apiserver operator is not in a stable status")
			}

			g.By("1 - Create bad admission webhook configurations")
			o.Expect(oc.WithoutNamespace().Run("new-project").Args(namespace).Execute()).To(o.Succeed())
			validatingWebHook.createAdmissionWebhookFromTemplate(oc)
			_, isAvailable := checkIfResourceAvailable(oc, "ValidatingWebhookConfiguration", []string{badValidatingWebhookName}, "")
			o.Expect(isAvailable).To(o.BeTrue())

			mutatingWebHook.createAdmissionWebhookFromTemplate(oc)
			_, isAvailable = checkIfResourceAvailable(oc, "MutatingWebhookConfiguration", []string{badMutatingWebhookName}, "")
			o.Expect(isAvailable).To(o.BeTrue())

			crdWebHook.createAdmissionWebhookFromTemplate(oc)
			_, isAvailable = checkIfResourceAvailable(oc, "crd", []string{badCrdWebhookName}, "")
			o.Expect(isAvailable).To(o.BeTrue())

			g.By("2 - Verify kube-apiserver reports webhook configuration errors and remains stable")
			compareAPIServerWebhookConditions(oc, webhookServiceFailureReasons, status, webHookErrorConditionTypes)
			compareAPIServerWebhookConditions(oc, "AdmissionWebhookMatchesVirtualResource", status, []string{"VirtualResourceAdmissionError"})

			currentKAStatus := getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
			if !(reflect.DeepEqual(currentKAStatus, kasStatusBefore) || reflect.DeepEqual(currentKAStatus, kubeApiserverCoStatus)) {
				e2e.Failf("kube-apiserver operator status changed after creating bad admission webhooks")
			}

			g.By("3 - Delete bad webhooks and verify errors clear")
			_ = oc.Run("delete").Args("ValidatingWebhookConfiguration", badValidatingWebhookName, "--ignore-not-found").Execute()
			_ = oc.Run("delete").Args("MutatingWebhookConfiguration", badMutatingWebhookName, "--ignore-not-found").Execute()
			_ = oc.Run("delete").Args("crd", badCrdWebhookName, "--ignore-not-found").Execute()

			allWebhookErrors := append(webHookErrorConditionTypes, "VirtualResourceAdmissionError")
			compareAPIServerWebhookConditions(oc, "", "False", allWebhookErrors)

			currentKAStatus = getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
			if !(reflect.DeepEqual(currentKAStatus, kasStatusBefore) || reflect.DeepEqual(currentKAStatus, kubeApiserverCoStatus)) {
				e2e.Failf("kube-apiserver operator status changed after deleting bad webhooks")
			}
		})

	g.It("[OTP][OCP-50223] Checks on different bad admission webhook errors and kube-apiserver status [Serial][apigroup:admissionregistration.k8s.io][apigroup:apiextensions.k8s.io][Timeout:15m]",
		ote.Informing(), func() {
			var (
				validatingWebhookNameNotFound     = "test-validating-notfound-cfg"
				mutatingWebhookNameNotFound       = "test-mutating-notfound-cfg"
				crdWebhookNameNotFound            = "testcrdwebhooks.tests.com"
				validatingWebhookNameNotReachable = "test-validating-notreachable-cfg2"
				mutatingWebhookNameNotReachable   = "test-mutating-notreachable-cfg2"
				crdWebhookNameNotReachable        = "testcrdwebhoks.tsts.com"
				serviceName                       = "example-service"
				serviceNameNotFound               = "service-unknown"
				kubeApiserverCoStatus             = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
				webhookConditionErrors            = []string{"ValidatingAdmissionWebhookConfigurationError", "MutatingAdmissionWebhookConfigurationError", "CRDConversionWebhookConfigurationError"}
				webhookServiceFailureReasons      = []string{"WebhookServiceNotFound", "WebhookServiceNotReady", "WebhookServiceConnectionError"}
			)

			preConfigKasStatus := getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
			if preConfigKasStatus["Available"] != "True" {
				g.Skip(fmt.Sprintf("kube-apiserver operator is not Available: %v", preConfigKasStatus))
			}

			oc.SetupProject()

			validatingWebHook := admissionWebhook{
				name: validatingWebhookNameNotFound, webhookname: "test.validating.com",
				servicenamespace: oc.Namespace(), servicename: serviceName, namespace: oc.Namespace(),
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				template: apiserverAuthFixture("ValidatingWebhookConfigurationTemplate.yaml"),
			}
			mutatingWebHook := admissionWebhook{
				name: mutatingWebhookNameNotFound, webhookname: "test.mutating.com",
				servicenamespace: oc.Namespace(), servicename: serviceName, namespace: oc.Namespace(),
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				template: apiserverAuthFixture("MutatingWebhookConfigurationTemplate.yaml"),
			}
			crdWebHook := admissionWebhook{
				name: crdWebhookNameNotFound, webhookname: "tests.com",
				servicenamespace: oc.Namespace(), servicename: serviceName, namespace: oc.Namespace(),
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				singularname: "testcrdwebhooks", pluralname: "testcrdwebhooks", kind: "TestCrdWebhook", shortname: "tcw", version: "v1beta1",
				template: apiserverAuthFixture("CRDWebhookConfigurationCustomTemplate.yaml"),
			}

			defer func() {
				_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookNameNotFound, "--ignore-not-found").Execute()
				_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookNameNotFound, "--ignore-not-found").Execute()
				_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", crdWebhookNameNotFound, "--ignore-not-found").Execute()
				_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookNameNotReachable, "--ignore-not-found").Execute()
				_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookNameNotReachable, "--ignore-not-found").Execute()
				_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", crdWebhookNameNotReachable, "--ignore-not-found").Execute()
				_ = oc.AsAdmin().Run("delete").Args("service", serviceName, "-n", oc.Namespace(), "--ignore-not-found").Execute()
			}()

			g.By("Cleanup any existing webhook resources from previous runs")
			_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("ValidatingWebhookConfiguration", validatingWebhookNameNotFound, "--ignore-not-found").Execute()
			_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("MutatingWebhookConfiguration", mutatingWebhookNameNotFound, "--ignore-not-found").Execute()
			_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", crdWebhookNameNotFound, "--ignore-not-found").Execute()

			validatingWebHook.createAdmissionWebhookFromTemplate(oc)
			mutatingWebHook.createAdmissionWebhookFromTemplate(oc)
			crdWebHook.createAdmissionWebhookFromTemplate(oc)

			compareAPIServerWebhookConditions(oc, webhookServiceFailureReasons, "True", webhookConditionErrors)
			kasOperatorCheckForStep(oc, preConfigKasStatus, "6", "bad admission webhooks configured")

			clusterIP, err := oc.AsAdmin().Run("get").Args("service", "kubernetes", "-o=jsonpath={.spec.clusterIP}", "-n", "default").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			newServiceIP := getServiceIP(oc, clusterIP)

			webhookService := webhookService{
				name: serviceName, clusterip: newServiceIP, namespace: oc.Namespace(),
				template: apiserverAuthFixture("ServiceTemplate.yaml"),
			}
			preConfigKasStatus = getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
			webhookService.createServiceFromTemplate(oc)
			kasOperatorCheckForStep(oc, preConfigKasStatus, "8", "creating services for admission webhooks")
			compareAPIServerWebhookConditions(oc, webhookServiceFailureReasons, "True", webhookConditionErrors)

			validatingWebHookUnknown := admissionWebhook{
				name: validatingWebhookNameNotReachable, webhookname: "test.validating2.com",
				servicenamespace: oc.Namespace(), servicename: serviceNameNotFound, namespace: oc.Namespace(),
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				template: apiserverAuthFixture("ValidatingWebhookConfigurationTemplate.yaml"),
			}
			mutatingWebHookUnknown := admissionWebhook{
				name: mutatingWebhookNameNotReachable, webhookname: "test.mutating2.com",
				servicenamespace: oc.Namespace(), servicename: serviceNameNotFound, namespace: oc.Namespace(),
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				template: apiserverAuthFixture("MutatingWebhookConfigurationTemplate.yaml"),
			}
			crdWebHookUnknown := admissionWebhook{
				name: crdWebhookNameNotReachable, webhookname: "tsts.com",
				servicenamespace: oc.Namespace(), servicename: serviceNameNotFound, namespace: oc.Namespace(),
				apigroups: "", apiversions: "v1", operations: "CREATE", resources: "pods",
				singularname: "testcrdwebhoks", pluralname: "testcrdwebhoks", kind: "TestCrdwebhok", shortname: "tcwk", version: "v1beta1",
				template: apiserverAuthFixture("CRDWebhookConfigurationCustomTemplate.yaml"),
			}

			preConfigKasStatus = getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
			validatingWebHookUnknown.createAdmissionWebhookFromTemplate(oc)
			mutatingWebHookUnknown.createAdmissionWebhookFromTemplate(oc)
			crdWebHookUnknown.createAdmissionWebhookFromTemplate(oc)
			kasOperatorCheckForStep(oc, preConfigKasStatus, "10", "creating webhook configurations with unknown service references")
			compareAPIServerWebhookConditions(oc, webhookServiceFailureReasons, "True", webhookConditionErrors)

			preConfigKasStatus = getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
			for _, args := range [][]string{
				{"ValidatingWebhookConfiguration", validatingWebhookNameNotReachable},
				{"MutatingWebhookConfiguration", mutatingWebhookNameNotReachable},
				{"crd", crdWebhookNameNotReachable},
				{"ValidatingWebhookConfiguration", validatingWebhookNameNotFound},
				{"MutatingWebhookConfiguration", mutatingWebhookNameNotFound},
				{"crd", crdWebhookNameNotFound},
			} {
				o.Expect(oc.AsAdmin().WithoutNamespace().Run("delete").Args(args...).Execute()).To(o.Succeed())
			}
			_ = oc.AsAdmin().Run("delete").Args("service", serviceName, "-n", oc.Namespace(), "--ignore-not-found").Execute()
			kasOperatorCheckForStep(oc, preConfigKasStatus, "12", "deleting bad webhooks")
			compareAPIServerWebhookConditions(oc, "", "False", webhookConditionErrors)
		})

	g.It("[OTP][OCP-50188] Prepare upgrade cluster under APF stress [Slow][Disruptive][apigroup:config.openshift.io][Timeout:40m]",
		ote.Informing(), func() {
			dirname := "/tmp/-OCP-40667/"
			exceptions := "panicked: false"
			defer os.RemoveAll(dirname)
			architecture.SkipNonAmd64SingleArch(oc)
			o.Expect(os.MkdirAll(dirname, 0755)).To(o.Succeed())

			if err := clusterHealthcheck(oc, "OCP-40667/log"); err != nil {
				g.Skip(fmt.Sprintf("cluster health check failed before APF stress preparation: %v", err))
			}

			g.By("Verify priority level configuration")
			for _, plc := range []struct{ name, shares string }{
				{"workload-low", "100"},
				{"global-default", "20"},
			} {
				output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("prioritylevelconfiguration", plc.name, "-o", `jsonpath={.spec.limited.nominalConcurrencyShares}`).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.Equal(plc.shares))
			}

			cpuAvgWorker, memAvgWorker := checkClusterLoad(oc, "worker", dirname+"nodes.log")
			cpuAvgMaster, memAvgMaster := checkClusterLoad(oc, "master", dirname+"nodes.log")
			if cpuAvgMaster >= 70 || memAvgMaster >= 70 || cpuAvgWorker >= 60 || memAvgWorker >= 60 {
				g.Skip(fmt.Sprintf("cluster load too high for stress test: master CPU=%d%% MEM=%d%%, worker CPU=%d%% MEM=%d%%", cpuAvgMaster, memAvgMaster, cpuAvgWorker, memAvgWorker))
			}
			stressNs := loadCPUMemWorkload(oc, 1200)
			defer oc.AsAdmin().Run("delete").Args("namespace", stressNs, "--ignore-not-found=true").Output()

			g.By("Verify kube-burner stress pods complete and cluster remains healthy")
			errPod := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, 1500*time.Second, false, func(ctx context.Context) (bool, error) {
				podOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-A").Output()
				if err != nil {
					e2e.Logf("transient error listing pods, will retry: %v", err)
					return false, nil
				}
				_ = os.WriteFile(dirname+"pod.log", []byte(podOutput), 0644)
				cmd := fmt.Sprintf(`cat %s | grep -iE 'cpu-stress' | grep -i 'Running' || true`, dirname+"pod.log")
				podLogs, err := exec.Command("bash", "-c", cmd).Output()
				if err != nil {
					e2e.Logf("transient error checking pod status, will retry: %v", err)
					return false, nil
				}
				return len(podLogs) == 0, nil
			})
			compat_otp.AssertWaitPollNoErr(errPod, "kube-burner stress pods did not complete successfully")

			o.Expect(clusterNodesHealthcheck(oc, 100, dirname+"log")).To(o.Succeed())
			o.Expect(clusterOperatorHealthcheck(oc, 500, dirname+"log")).To(o.Succeed())

			podList, err := compat_otp.GetAllPodsWithLabel(oc, "openshift-kube-apiserver", "apiserver")
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, kasPod := range podList {
				kasOutput, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", "openshift-kube-apiserver", string(kasPod)).Output()
				_ = os.WriteFile(dirname+"kas.log."+string(kasPod), []byte(kasOutput), 0644)
			}
			noRouteLogs, _ := exec.Command("bash", "-c", fmt.Sprintf(`cat %s | grep -iE 'apf_controller.go|apf_filter.go' | grep 'no route' || true`, dirname+"kas.log.*")).Output()
			panicLogs, _ := exec.Command("bash", "-c", fmt.Sprintf(`cat %s | grep -i 'panic' | grep -Ev "%s" || true`, dirname+"kas.log.*", exceptions)).Output()
			cpuAvgVal, memAvgVal := 0, 0
			errLoad := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
				cpuAvgVal, memAvgVal = checkClusterLoad(oc, "master", dirname+"nodes_new.log")
				return cpuAvgVal <= 70 && memAvgVal <= 75, nil
			})
			compat_otp.AssertWaitPollNoErr(errLoad, fmt.Sprintf("master node CPU %d%% or memory %d%% remained high", cpuAvgVal, memAvgVal))

			if cpuAvgVal > 70 || memAvgVal > 75 || len(noRouteLogs) > 0 || len(panicLogs) > 0 {
				e2e.Failf("APF stress verification failed")
			}
		})

	g.It("[OTP] ClusterResourceQuota objects validation [apigroup:quota.openshift.io][apigroup:monitoring.coreos.com]", func() {
		const caseID = "ocp-54745"
		namespace := caseID + "-quota-test-" + compat_otp.GetRandomString()
		clusterQuotaName := caseID + "-crq-test"
		crqLimits := map[string]string{
			"pods": "4", "secrets": "10", "cpu": "7", "memory": "5Gi",
			"requests.cpu": "6", "requests.memory": "6Gi", "limits.cpu": "6", "limits.memory": "6Gi",
			"configmaps": "5", "count/deployments.apps": "1",
			"count/templates.template.openshift.io": "3", "count/servicemonitors.monitoring.coreos.com": "1",
		}

		defer func() {
			_ = oc.AsAdmin().WithoutNamespace().Run("delete", "project").Args(namespace).Execute()
			_ = oc.WithoutNamespace().AsAdmin().Run("delete").Args("clusterresourcequota", clusterQuotaName).Execute()
		}()

		o.Expect(oc.WithoutNamespace().AsAdmin().Run("create").Args("ns", namespace).Execute()).To(o.Succeed())
		o.Expect(oc.WithoutNamespace().AsAdmin().Run("create").Args("-n", namespace, "-f", apiserverAuthFixture("clusterresourcequota.yaml")).Execute()).To(o.Succeed())

		params := []string{"-n", namespace, "clusterresourequotaremplate", "-p",
			"NAME=" + clusterQuotaName, "LABEL=" + namespace,
			"PODS_LIMIT=" + crqLimits["pods"], "SECRETS_LIMIT=" + crqLimits["secrets"],
			"CPU_LIMIT=" + crqLimits["cpu"], "MEMORY_LIMIT=" + crqLimits["memory"],
			"REQUESTS_CPU=" + crqLimits["requests.cpu"], "REQUEST_MEMORY=" + crqLimits["requests.memory"],
			"LIMITS_CPU=" + crqLimits["limits.cpu"], "LIMITS_MEMORY=" + crqLimits["limits.memory"],
			"CONFIGMAPS_LIMIT=" + crqLimits["configmaps"],
			"TEMPLATE_COUNT=" + crqLimits["count/templates.template.openshift.io"],
			"SERVICE_MONITOR=" + crqLimits["count/servicemonitors.monitoring.coreos.com"],
			"DEPLOYMENT=" + crqLimits["count/deployments.apps"],
		}
		quotaConfigFile := compat_otp.ProcessTemplate(oc, params...)
		o.Expect(oc.WithoutNamespace().AsAdmin().Run("create").Args("-n", namespace, "-f", quotaConfigFile).Execute()).To(o.Succeed())

		createSecretsWithQuotaValidation(oc, namespace, clusterQuotaName, crqLimits, caseID)

		podsCount, err := oc.Run("get").Args("-n", namespace, "clusterresourcequota", clusterQuotaName, "-o", `jsonpath={.status.namespaces[*].status.used.pods}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		existingPodCount, _ := strconv.Atoi(strings.TrimSpace(podsCount))
		limits, _ := strconv.Atoi(crqLimits["pods"])
		podTemplate := apiserverAuthFixture("ocp54745-pod.yaml")
		for i := existingPodCount; i < limits-2; i++ {
			podname := fmt.Sprintf("%s-pod-%d", caseID, i)
			podParams := []string{"-n", namespace, "-f", podTemplate, "-p", "NAME=" + podname, "REQUEST_MEMORY=1Gi", "REQUEST_CPU=1", "LIMITS_MEMORY=1Gi", "LIMITS_CPU=1"}
			podConfigFile := compat_otp.ProcessTemplate(oc, podParams...)
			o.Expect(oc.AsAdmin().WithoutNamespace().Run("-n", namespace, "create").Args("-f", podConfigFile).Execute()).To(o.Succeed())
		}

		o.Expect(oc.WithoutNamespace().AsAdmin().Run("create").Args("-n", namespace, "-f", apiserverAuthFixture("service-monitor.yaml")).Execute()).To(o.Succeed())
		image := "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83"
		deploymentLimit, _ := strconv.Atoi(crqLimits["count/deployments.apps"])
		for count := 1; count < 3; count++ {
			appName := fmt.Sprintf("%s-app-%d", caseID, count)
			output, err := oc.AsAdmin().WithoutNamespace().Run("new-app").Args("--name="+appName, image, "-n", namespace).Output()
			if count <= deploymentLimit {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				o.Expect(output).To(o.MatchRegexp(`deployments\.apps.*forbidden: exceeded quota`))
			}
			smParams := []string{"-n", namespace, "servicemonitortemplate", "-p",
				fmt.Sprintf("NAME=%s-service-monitor-%d", caseID, count),
				"DEPLOYMENT=" + crqLimits["count/deployments.apps"],
			}
			serviceMonitor := compat_otp.ProcessTemplate(oc, smParams...)
			output, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-n", namespace, "-f", serviceMonitor).Output()
			smLimit, _ := strconv.Atoi(crqLimits["count/servicemonitors.monitoring.coreos.com"])
			if count <= smLimit {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				o.Expect(output).To(o.MatchRegexp(`servicemonitors.*forbidden: exceeded quota`))
			}
		}

		// Additional validation - test quota limits by creating more resources
		g.By("Verify quota limits by attempting to exceed them")
		for i := existingPodCount + (limits - 2); i <= limits; i++ {
			podname := fmt.Sprintf("%s-pod-%d", caseID, i)
			podParams := []string{"-n", namespace, "-f", podTemplate, "-p", "NAME=" + podname, "REQUEST_MEMORY=1Gi", "REQUEST_CPU=1", "LIMITS_MEMORY=1Gi", "LIMITS_CPU=1"}
			podConfigFile := compat_otp.ProcessTemplate(oc, podParams...)
			output, err := oc.AsAdmin().WithoutNamespace().Run("-n", namespace, "create").Args("-f", podConfigFile).Output()
			if i < limits {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				o.Expect(output).To(o.MatchRegexp(`pods.*forbidden: exceeded quota`))
			}
		}

		cmLimit, _ := strconv.Atoi(crqLimits["configmaps"])
		cmCount, err := oc.Run("get").Args("-n", namespace, "clusterresourcequota", clusterQuotaName, "-o", `jsonpath={.status.namespaces[*].status.used.configmaps}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmUsedCount, _ := strconv.Atoi(strings.TrimSpace(cmCount))
		for i := cmUsedCount; i <= cmLimit; i++ {
			configmapName := fmt.Sprintf("%s-configmap-%d", caseID, i)
			output, err := oc.Run("create").Args("-n", namespace, "configmap", configmapName).Output()
			if i < cmLimit {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				o.Expect(output).To(o.MatchRegexp(`configmaps.*forbidden: exceeded quota`))
			}
		}

		// Verify all resource quotas are within limits
		for _, resourceName := range []string{"pods", "secrets", "cpu", "memory", "configmaps"} {
			resource, err := oc.Run("get").Args("-n", namespace, "clusterresourcequota", clusterQuotaName, "-o", fmt.Sprintf(`jsonpath={.status.namespaces[*].status.used.%s}`, resourceName)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			usedResource, _ := strconv.Atoi(strings.Trim(strings.TrimSpace(resource), "mGi"))
			limit, _ := strconv.Atoi(strings.Trim(crqLimits[resourceName], "mGi"))
			if usedResource <= 0 || usedResource > limit {
				e2e.Failf("cluster resource quota for %s is out of expected bounds: used=%d limit=%d", resourceName, usedResource, limit)
			}
		}
	})

})
