package apiserver

import (
	"context"
	"fmt"
	"regexp"
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

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Feature:PullSecret]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("apiserver-pullsecret")

	g.It("[OTP][OCP-12036] User can pull a private image from a registry when a pull secret is defined [Serial][apigroup:build.openshift.io][apigroup:apps.openshift.io]",
		ote.Informing(), func() {
			skipIfBaselineCapsMissingCapabilities(oc, "Build", "DeploymentConfig")
			skipIfProxyCluster(oc)
			architecture.SkipArchitectures(oc, architecture.MULTI)

			g.By("Create a new project required for this test execution")
			oc.SetupProject()
			namespace := oc.Namespace()

			g.By("Build hello-world from external source")
			helloWorldSource := "quay.io/openshifttest/ruby-27:1.2.0~https://github.com/openshift/ruby-hello-world"
			buildName := fmt.Sprintf("pullsecret-test-%s", strings.ToLower(compat_otp.RandStr(5)))
			err := oc.Run("new-build").Args(helloWorldSource, "--name="+buildName, "-n", namespace, "--import-mode=PreserveOriginal").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Wait for hello-world build to success")
			buildClient := oc.BuildClient().BuildV1().Builds(oc.Namespace())
			err = compat_otp.WaitForABuild(buildClient, buildName+"-1", nil, nil, nil)
			if err != nil {
				compat_otp.DumpBuildLogs(buildName, oc)
			}
			compat_otp.AssertWaitPollNoErr(err, "build is not complete")

			g.By("Get dockerImageRepository value from imagestreams test")
			dockerImageRepository1, err := oc.Run("get").Args("imagestreams", buildName, "-o=jsonpath={.status.dockerImageRepository}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			dockerServer := strings.Split(strings.TrimSpace(dockerImageRepository1), "/")
			o.Expect(dockerServer).NotTo(o.BeEmpty())

			g.By("Create another project with the second user")
			oc.SetupProject()

			g.By("Get access token")
			token, err := oc.Run("whoami").Args("-t").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Give user admin permission")
			username := oc.Username()
			err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-admin", username).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create secret for private image under project")
			err = oc.WithoutNamespace().AsAdmin().Run("create").Args("secret", "docker-registry", "user1-dockercfg", "--docker-email=any@any.com", "--docker-server="+dockerServer[0], "--docker-username="+username, "--docker-password="+token, "-n", oc.Namespace()).NotShowInfo().Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create new deploymentconfig from the dockerImageRepository")
			deploymentConfigYaml, err := oc.Run("create").Args("deploymentconfig", "frontend", "--image="+dockerImageRepository1, "--dry-run=client", "-o=yaml").OutputToFile("pullsecret-dc.yaml")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Modify the deploymentconfig and create a new deployment")
			compat_otp.ModifyYamlFileContent(deploymentConfigYaml, []compat_otp.YamlReplace{
				{Path: "spec.template.spec.containers.0.imagePullPolicy", Value: "Always"},
				{Path: "spec.template.spec.imagePullSecrets", Value: "- name: user1-dockercfg"},
			})
			err = oc.Run("create").Args("-f", deploymentConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check if pod is properly running with expected status")
			podsList := getPodsListByLabel(oc.AsAdmin(), oc.Namespace(), "deploymentconfig=frontend")
			compat_otp.AssertPodToBeReady(oc, podsList[0], oc.Namespace())
		})

	g.It("[OTP][OCP-11905] Use well-formed pull secret with incorrect credentials will fail to build and deploy [Serial][apigroup:build.openshift.io][apigroup:apps.openshift.io]",
		ote.Informing(), func() {
			skipIfBaselineCapsMissingCapabilities(oc, "Build", "DeploymentConfig")
			skipIfProxyCluster(oc)
			architecture.SkipArchitectures(oc, architecture.MULTI)

			g.By("Create a new project required for this test execution")
			oc.SetupProject()
			namespace := oc.Namespace()

			g.By("Build hello-world from external source")
			helloWorldSource := "quay.io/openshifttest/ruby-27:1.2.0~https://github.com/openshift/ruby-hello-world"
			buildName := fmt.Sprintf("pullsecret-wrong-%s", strings.ToLower(compat_otp.RandStr(5)))
			err := oc.Run("new-build").Args(helloWorldSource, "--name="+buildName, "-n", namespace, "--import-mode=PreserveOriginal").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Wait for ImageStream import to complete")
			err = wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
				isOutput, err := oc.Run("get").Args("imagestream", "ruby-27", "-n", namespace, "-o=jsonpath={.status.tags[?(@.tag=='1.2.0')].items[0].dockerImageReference}").Output()
				if err != nil {
					return false, nil
				}
				return strings.TrimSpace(isOutput) != "", nil
			})
			compat_otp.AssertWaitPollNoErr(err, "ImageStream import did not complete")

			g.By("Wait for hello-world build to success")
			buildClient := oc.BuildClient().BuildV1().Builds(oc.Namespace())
			err = compat_otp.WaitForABuild(buildClient, buildName+"-1", nil, nil, nil)
			if err != nil {
				compat_otp.DumpBuildLogs(buildName, oc)
			}
			compat_otp.AssertWaitPollNoErr(err, "build is not complete")

			g.By("Get dockerImageRepository value from imagestreams test")
			dockerImageRepository1, err := oc.Run("get").Args("imagestreams", buildName, "-o=jsonpath={.status.dockerImageRepository}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			dockerServer := strings.Split(strings.TrimSpace(dockerImageRepository1), "/")
			o.Expect(dockerServer).NotTo(o.BeEmpty())

			g.By("Create another project with the second user")
			oc.SetupProject()

			g.By("Give user admin permission")
			username := oc.Username()
			err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-admin", username).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create secret for private image under project with wrong password")
			err = oc.WithoutNamespace().AsAdmin().Run("create").Args("secret", "docker-registry", "user1-dockercfg", "--docker-email=any@any.com", "--docker-server="+dockerServer[0], "--docker-username="+username, "--docker-password=password", "-n", oc.Namespace()).NotShowInfo().Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create new deploymentconfig from the dockerImageRepository")
			deploymentConfigYaml, err := oc.Run("create").Args("deploymentconfig", "frontend", "--image="+dockerImageRepository1, "--dry-run=client", "-o=yaml").OutputToFile("pullsecret-wrong-dc.yaml")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Modify the deploymentconfig and create a new deployment")
			compat_otp.ModifyYamlFileContent(deploymentConfigYaml, []compat_otp.YamlReplace{
				{Path: "spec.template.spec.containers.0.imagePullPolicy", Value: "Always"},
				{Path: "spec.template.spec.imagePullSecrets", Value: "- name: user1-dockercfg"},
			})
			err = oc.Run("create").Args("-f", deploymentConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check if pod is running with the expected status")
			err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
				podOutput, err := oc.Run("get").Args("pod").Output()
				if err == nil {
					matched, _ := regexp.MatchString("frontend-1-.*(ImagePullBackOff|ErrImagePull)", podOutput)
					if matched {
						return true, nil
					}
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(err, "pod did not show up with the expected status")
		})

	g.It("[OTP][OCP-11138] Deploy will fail with incorrectly formed pull secrets [apigroup:build.openshift.io][apigroup:apps.openshift.io][apigroup:image.openshift.io]",
		ote.Informing(), func() {
			skipIfBaselineCapsMissingCapabilities(oc, "Build", "DeploymentConfig", "ImageRegistry")
			skipIfProxyCluster(oc)
			architecture.SkipArchitectures(oc, architecture.MULTI)

			g.By("Create a new project required for this test execution")
			oc.SetupProject()
			namespace := oc.Namespace()

			g.By("Build hello-world from external source")
			helloWorldSource := "quay.io/openshifttest/ruby-27:1.2.0~https://github.com/openshift/ruby-hello-world"
			buildName := fmt.Sprintf("pullsecret-bad-%s", strings.ToLower(compat_otp.RandStr(5)))
			err := oc.Run("new-build").Args(helloWorldSource, "--name="+buildName, "-n", namespace, "--import-mode=PreserveOriginal").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Wait for ImageStream import to complete")
			err = wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
				isOutput, err := oc.Run("get").Args("imagestream", "ruby-27", "-n", namespace, "-o=jsonpath={.status.tags[?(@.tag=='1.2.0')].items[0].dockerImageReference}").Output()
				if err != nil {
					return false, nil
				}
				// Check if the image reference is populated
				return strings.TrimSpace(isOutput) != "", nil
			})
			compat_otp.AssertWaitPollNoErr(err, "ImageStream import did not complete")

			g.By("Wait for hello-world build to success")
			err = compat_otp.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildName+"-1", nil, nil, nil)
			if err != nil {
				compat_otp.DumpBuildLogs(buildName, oc)
			}
			compat_otp.AssertWaitPollNoErr(err, "build is not complete")

			g.By("Get dockerImageRepository value from imagestreams test")
			dockerImageRepository1, err := oc.Run("get").Args("imagestreams", buildName, "-o=jsonpath={.status.dockerImageRepository}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Create another project")
			oc.SetupProject()

			g.By("Create new deploymentconfig from the dockerImageRepository")
			deploymentConfigYaml, err := oc.Run("create").Args("deploymentconfig", "frontend", "--image="+dockerImageRepository1, "--dry-run=client", "-o=yaml").OutputToFile("pullsecret-bad-dc.yaml")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Modify the deploymentconfig and create a new deployment with nonexistent secret")
			compat_otp.ModifyYamlFileContent(deploymentConfigYaml, []compat_otp.YamlReplace{
				{Path: "spec.template.spec.containers.0.imagePullPolicy", Value: "Always"},
				{Path: "spec.template.spec.imagePullSecrets", Value: "- name: notexist-secret"},
			})
			err = oc.Run("create").Args("-f", deploymentConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check if pod is running with expected ImagePullBackOff status")
			err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
				podOutput, err := oc.Run("get").Args("pod").Output()
				if err == nil {
					matched, _ := regexp.MatchString("frontend-1-.*(ImagePullBackOff|ErrImagePull)", podOutput)
					return matched, nil
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(err, "pod did not show up with the expected status")

			g.By("Create generic secret from deploymentconfig")
			err = oc.Run("create").Args("secret", "generic", "notmatch-secret", "--from-file="+deploymentConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Modify the deploymentconfig again and create a new deployment")
			buildName = fmt.Sprintf("pullsecret-bad-new-%s", strings.ToLower(compat_otp.RandStr(5)))
			compat_otp.ModifyYamlFileContent(deploymentConfigYaml, []compat_otp.YamlReplace{
				{Path: "metadata.name", Value: buildName},
				{Path: "spec.template.spec.containers.0.imagePullPolicy", Value: "Always"},
				{Path: "spec.template.spec.imagePullSecrets", Value: "- name: notmatch-secret"},
			})
			err = oc.Run("create").Args("-f", deploymentConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check if pod is running with expected ImagePullBackOff status")
			err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
				podOutput, err := oc.Run("get").Args("pod").Output()
				if err == nil {
					matched, _ := regexp.MatchString(buildName+"-1-.*(ImagePullBackOff|ErrImagePull)", podOutput)
					return matched, nil
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(err, "pod did not show up with the expected status")
		})

	g.It("[OTP][OCP-70369] Use bound service account tokens when generating pull secrets [apigroup:image.openshift.io]",
		ote.Informing(), func() {
			randomSaAcc := "test-" + compat_otp.GetRandomString()

			oc.SetupProject()
			namespace := oc.Namespace()

			g.By("Check if Image registry is enabled")
			output, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("configs.imageregistry.operator.openshift.io/cluster", "-o", `jsonpath='{.spec.managementState}'`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if !strings.Contains(output, "Managed") {
				g.Skip("Skipping case as registry is not enabled")
			}

			g.By("Create serviceAccount " + randomSaAcc)
			err = oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", randomSaAcc, "-n", namespace).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Check if Token Secrets of SA are created")
			secretOutput := getResourceToBeReady(oc, asAdmin, withoutNamespace, "secrets", "-n", namespace, "-o", `jsonpath={range .items[*]}{.metadata.name}{" "}{end}`)
			o.Expect(secretOutput).ShouldNot(o.BeEmpty())
			o.Expect(secretOutput).ShouldNot(o.ContainSubstring("token"))
			o.Expect(secretOutput).Should(o.ContainSubstring("dockercfg"))

			g.By("Create a deployment that uses an image from the internal registry")
			podTemplate := apiserverAuthFixture("ocp-70369.yaml")
			params := []string{"-n", namespace, "-f", podTemplate, "-p", fmt.Sprintf("NAMESPACE=%s", namespace), "SERVICE_ACCOUNT_NAME=" + randomSaAcc}
			configFile := compat_otp.ProcessTemplate(oc, params...)
			err = oc.AsAdmin().Run("create").Args("-f", configFile, "-n", namespace).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			podName := getPodsList(oc.AsAdmin(), namespace)
			o.Expect(podName).NotTo(o.BeEmpty())
			compat_otp.AssertPodToBeReady(oc, podName[0], namespace)

			g.By("Verify the openshift.io/internal-registry-pull-secret-ref annotation in the ServiceAccount")
			serviceCaOutput := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pod", podName[0], "-n", namespace, "-o", `jsonpath={.spec.serviceAccount}`)
			o.Expect(serviceCaOutput).Should(o.ContainSubstring(randomSaAcc))
			imageSecretOutput := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pod", podName[0], "-n", namespace, "-o", `jsonpath={.spec.imagePullSecrets[*].name}`)
			o.Expect(imageSecretOutput).Should(o.ContainSubstring(randomSaAcc + "-dockercfg"))
			imageSaOutput := getResourceToBeReady(oc, asAdmin, withoutNamespace, "sa", randomSaAcc, "-n", namespace, "-o", `jsonpath={.metadata.annotations.openshift\.io/internal-registry-pull-secret-ref}`)
			o.Expect(imageSaOutput).Should(o.ContainSubstring(randomSaAcc + "-dockercfg"))

			g.By("Verify no reconciliation loops cause unbounded dockercfg secret creation")
			saName := "my-test-sa"
			saYAML := fmt.Sprintf(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: %s
`, saName)

			for i := 0; i < 10; i++ {
				output, err := oc.WithoutNamespace().AsAdmin().Run("create").Args("-n", namespace, "-f", "-").InputString(saYAML).Output()
				if err != nil {
					if !strings.Contains(output, "AlreadyExists") {
						e2e.Failf("Failed to create ServiceAccount: %v", err.Error())
					}
					err = oc.WithoutNamespace().AsAdmin().Run("replace").Args("-n", namespace, "-f", "-").InputString(saYAML).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				time.Sleep(2 * time.Second)
			}

			saList := getResourceToBeReady(oc, asAdmin, withoutNamespace, "-n", namespace, "sa", saName, "-o=jsonpath={.metadata.name}")
			o.Expect(saList).NotTo(o.BeEmpty())

			saNameSecretTypes, err := getResource(oc, asAdmin, withoutNamespace, "-n", namespace, "secrets", `-o`, `jsonpath={range .items[?(@.metadata.ownerReferences[0].name=="`+saName+`")]}{.type}{"\n"}{end}`)
			o.Expect(err).NotTo(o.HaveOccurred())

			dockerCfgCount := 0
			serviceAccountTokenCount := 0
			for _, secretType := range strings.Split(saNameSecretTypes, "\n") {
				switch secretType {
				case "kubernetes.io/dockercfg":
					dockerCfgCount++
				case "kubernetes.io/service-account-token":
					serviceAccountTokenCount++
				}
			}
			if dockerCfgCount != 1 || serviceAccountTokenCount != 0 {
				e2e.Failf("Expected 1 dockercfg secret and 0 token secret, but found %d dockercfg secrets and %d token secrets", dockerCfgCount, serviceAccountTokenCount)
			}
		})
})
