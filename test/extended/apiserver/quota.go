package apiserver

import (
	"bufio"
	"context"
	"fmt"
	"os"
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

// defaultRegistryServiceURL is the in-cluster service endpoint of the internal image registry.
const defaultRegistryServiceURL = "image-registry.openshift-image-registry.svc:5000"

// countResource returns the number of objects of the given resource type in the namespace.
func countResource(oc *exutil.CLI, resource string, namespace string) (int, error) {
	output, err := oc.Run("get").Args(resource, "-n", namespace, "-o", "jsonpath='{.items[*].metadata.name}'").Output()
	output = strings.Trim(strings.Trim(output, " "), "'")
	if output == "" {
		return 0, err
	}
	resources := strings.Split(output, " ")
	return len(resources), err
}

// ResourceQuota / project object-count quota tests: verify quota enforcement for images,
// image streams, and generic API object counts (pods/secrets/services/configmaps/quotas).
var _ = g.Describe("[sig-api-machinery] API_Server", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("default")
	var tmpdir string

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Image and object quota tests are not supported on MicroShift")
		}
	})

	g.JustBeforeEach(func() {
		tmpdir = "/tmp/-OCP-apiserver-quota-cases-" + compat_otp.GetRandomString() + "/"
		err := os.MkdirAll(tmpdir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.JustAfterEach(func() {
		os.RemoveAll(tmpdir)
		e2e.Logf("test dir %s is cleaned up", tmpdir)
	})

	g.It("[OTP][OCP-12158] When exceed openshift.io/images and storage limits will ban image tagging and registry push [Apiserver]", ote.Informing(), func() {
		if isBaselineCapsSet(oc) && !(isEnabledCapability(oc, "Build") && isEnabledCapability(oc, "DeploymentConfig") && isEnabledCapability(oc, "ImageRegistry")) {
			g.Skip("Skipping the test as baselinecaps have been set and some of API capabilities are not enabled!")
		}
		g.By("Check if it's a proxy cluster")
		httpProxy, httpsProxy, _ := getGlobalProxy(oc)
		if strings.Contains(httpProxy, "http") || strings.Contains(httpsProxy, "https") {
			g.Skip("Skip for proxy platform")
		}
		var (
			imageLimitRangeYamlFile = tmpdir + "image-limit-range.yaml"
			imageName1              = `quay.io/openshifttest/base-alpine@sha256:3126e4eed4a3ebd8bf972b2453fa838200988ee07c01b2251e3ea47e4b1f245c`
			imageName2              = `quay.io/openshifttest/hello-openshift:1.2.0`
			imageName3              = `quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83`
			imageStreamErr          error
		)

		g.By("1) Create new project required for this test execution")
		oc.SetupProject()
		namespace := oc.Namespace()
		defer oc.AsAdmin().Run("delete").Args("-f", imageLimitRangeYamlFile, "-n", namespace).Execute()

		imageLimitRangeYaml := `apiVersion: v1
kind: LimitRange
metadata:
  name: openshift-resource-limits
spec:
  limits:
    - type: openshift.io/Image
      max:
        storage: 1Gi
    - type: openshift.io/ImageStream
      max:
        openshift.io/image-tags: 20
        openshift.io/images: 1
`
		g.By("2) Create a resource quota limit of the image with images limit 1")
		f, err := os.Create(imageLimitRangeYamlFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer f.Close()
		w := bufio.NewWriter(f)
		_, err = w.WriteString(imageLimitRangeYaml)
		w.Flush()
		o.Expect(err).NotTo(o.HaveOccurred())

		quotaErr := oc.AsAdmin().Run("create").Args("-f", imageLimitRangeYamlFile, "-n", namespace).Execute()
		o.Expect(quotaErr).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("3.) Applying a mystream:v1 image tag to %s in an image stream should succeed", imageName1))
		tagErr := oc.AsAdmin().WithoutNamespace().Run("tag").Args(imageName1, "--source=docker", "mystream:v1", "-n", namespace).Execute()
		o.Expect(tagErr).NotTo(o.HaveOccurred())

		errImage := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
			var imageStreamOutput string
			imageStreamOutput, imageStreamErr = oc.AsAdmin().WithoutNamespace().Run("describe").Args("imagestream", "mystream", "-n", namespace).Output()
			if imageStreamErr == nil {
				if strings.Contains(imageStreamOutput, imageName1) {
					e2e.Logf("Image is tag with v1 successfully")
					return true, nil
				}
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errImage, fmt.Sprintf("Image is tag with v1 is not successfull %s", imageStreamErr))

		g.By(fmt.Sprintf("4.) Applying the mystream:v2 image tag to another %s in an image stream should fail due to the ImageStream max images limit", imageName2))
		tagErr = oc.AsAdmin().WithoutNamespace().Run("tag").Args(imageName2, "--source=docker", "mystream:v2", "-n", namespace).Execute()
		o.Expect(tagErr).NotTo(o.HaveOccurred())

		var imageStreamv2Err error
		errImageV2 := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
			var imageStreamv2Output string
			imageStreamv2Output, imageStreamv2Err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("imagestream", "mystream", "-n", namespace).Output()
			if imageStreamv2Err == nil {
				if strings.Contains(imageStreamv2Output, "Import failed") {
					e2e.Logf("Image is tag with v2 not successfull")
					return true, nil
				}
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errImageV2, fmt.Sprintf("Image is tag with v2 is successfull %s", imageStreamv2Err))

		g.By(`5.) Copying an image to the default internal registry of the cluster should be denied due to the max storage size limit for images`)
		destRegistry := "docker://" + defaultRegistryServiceURL + "/" + namespace + "/mystream:latest"
		publicImageUrl := "docker://" + imageName3
		var output string
		errPoll := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 120*time.Second, false, func(cxt context.Context) (bool, error) {
			output, err = exutil.CopyImageToInternalRegistry(oc, namespace, publicImageUrl, destRegistry)
			if err == nil {
				return false, fmt.Errorf("image copy unexpectedly succeeded when it should have been denied by quota")
			}
			if strings.Contains(output, "denied") {
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errPoll, "Failed to observe quota denial")
	})

	g.It("[OTP][OCP-12263] Verify openshift.io/images will not create image reference or push image to project when quota is exceeded[Apiserver]", ote.Informing(), func() {
		if isBaselineCapsSet(oc) && !(isEnabledCapability(oc, "Build") && isEnabledCapability(oc, "DeploymentConfig") && isEnabledCapability(oc, "ImageRegistry")) {
			g.Skip("Skipping the test as baselinecaps have been set and some of API capabilities are not enabled!")
		}

		g.By("Check if it's a proxy cluster")
		httpProxy, httpsProxy, _ := getGlobalProxy(oc)
		if strings.Contains(httpProxy, "http") || strings.Contains(httpsProxy, "https") {
			g.Skip("Skip for proxy platform")
		}

		var (
			imageLimitRangeYamlFile = tmpdir + "image-limit-range.yaml"
			imageName1              = `quay.io/openshifttest/base-alpine@sha256:3126e4eed4a3ebd8bf972b2453fa838200988ee07c01b2251e3ea47e4b1f245c`
			imageName2              = `quay.io/openshifttest/hello-openshift:1.2.0`
			imageName3              = `quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83`
			imageStreamErr          error
		)

		g.By("1) Create new project required for this test execution")
		oc.SetupProject()
		namespace := oc.Namespace()
		defer oc.AsAdmin().Run("delete").Args("-f", imageLimitRangeYamlFile, "-n", namespace).Execute()

		imageLimitRangeYaml := `apiVersion: v1
kind: LimitRange
metadata:
  name: openshift-resource-limits
spec:
  limits:
    - type: openshift.io/Image
      max:
        storage: 1Gi
    - type: openshift.io/ImageStream
      max:
        openshift.io/image-tags: 20
        openshift.io/images: 1
`
		g.By("2) Create a resource quota limit of the image with images limit 1")
		f, err := os.Create(imageLimitRangeYamlFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer f.Close()
		w := bufio.NewWriter(f)
		_, err = w.WriteString(imageLimitRangeYaml)
		w.Flush()
		o.Expect(err).NotTo(o.HaveOccurred())

		quotaErr := oc.AsAdmin().Run("create").Args("-f", imageLimitRangeYamlFile, "-n", namespace).Execute()
		o.Expect(quotaErr).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("3.) Applying a mystream:v1 image tag to %s in an image stream should succeed", imageName1))
		tagErr := oc.AsAdmin().WithoutNamespace().Run("tag").Args(imageName1, "--source=docker", "mystream:v1", "-n", namespace).Execute()
		o.Expect(tagErr).NotTo(o.HaveOccurred())

		// Inline steps will wait for tag 1 to get it imported successfully before adding tag 2 and this helps to avoid race-caused failure.Ref:OCPQE-7679.
		errImage := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
			var imageStreamOutput string
			imageStreamOutput, imageStreamErr = oc.AsAdmin().WithoutNamespace().Run("describe").Args("imagestream", "mystream", "-n", namespace).Output()
			if imageStreamErr == nil {
				if strings.Contains(imageStreamOutput, imageName1) {
					e2e.Logf("Image is tag with v1 successfully")
					return true, nil
				}
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errImage, fmt.Sprintf("Image is tag with v1 is not successfull %s", imageStreamErr))

		g.By(fmt.Sprintf("4.) Applying the mystream:v2 image tag to another %s in an image stream should fail due to the ImageStream max images limit", imageName2))
		tagErr = oc.AsAdmin().WithoutNamespace().Run("tag").Args(imageName2, "--source=docker", "mystream:v2", "-n", namespace).Execute()
		o.Expect(tagErr).NotTo(o.HaveOccurred())

		var imageStreamv2Err error
		errImageV2 := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
			var imageStreamv2Output string
			imageStreamv2Output, imageStreamv2Err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("imagestream", "mystream", "-n", namespace).Output()
			if imageStreamv2Err == nil {
				if strings.Contains(imageStreamv2Output, "Import failed") {
					e2e.Logf("Image is tag with v2 not successfull")
					return true, nil
				}
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errImageV2, fmt.Sprintf("Image is tag with v2 is successfull %s", imageStreamv2Err))

		g.By(`5.) Copying an image to the default internal registry of the cluster should be denied due to the max storage size limit for images`)
		destRegistry := "docker://" + defaultRegistryServiceURL + "/" + namespace + "/mystream:latest"
		publicImageUrl := "docker://" + imageName3
		var output string
		errPoll := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 120*time.Second, false, func(cxt context.Context) (bool, error) {
			output, err = exutil.CopyImageToInternalRegistry(oc, namespace, publicImageUrl, destRegistry)
			if err == nil {
				return false, fmt.Errorf("image copy unexpectedly succeeded when it should have been denied by quota")
			}
			if strings.Contains(output, "denied") {
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errPoll, "Failed to observe quota denial")
	})

	g.It("[OTP][OCP-12360] The number of created API objects can not exceed quota limitation [origin_platformexp_403]", ote.Informing(), func() {
		g.By("1) Create new project required for this test execution")
		oc.SetupProject()
		namespace := oc.Namespace()
		limit := 3

		g.By("2) Get quota limits according to used resouce count under namespace")
		type quotaLimits struct {
			podLimit           int
			resourcequotaLimit int
			secretLimit        int
			serviceLimit       int
			configmapLimit     int
		}

		var limits quotaLimits
		var err error

		limits.podLimit, err = countResource(oc, "pods", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		limits.podLimit += limit

		limits.resourcequotaLimit, err = countResource(oc, "resourcequotas", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		limits.resourcequotaLimit += limit + 1 // need to count the quota we added

		limits.secretLimit, err = countResource(oc, "secrets", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		limits.secretLimit += limit

		limits.serviceLimit, err = countResource(oc, "services", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		limits.serviceLimit += limit

		limits.configmapLimit, err = countResource(oc, "configmaps", namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
		limits.configmapLimit += limit

		e2e.Logf("Get limits of pods %d, resourcequotas %d, secrets %d, services %d, configmaps %d", limits.podLimit, limits.resourcequotaLimit, limits.secretLimit, limits.serviceLimit, limits.configmapLimit)

		filename := "ocp12360-quota.yaml"
		quotaName := "ocp12360-quota"
		g.By(fmt.Sprintf("3) Create quota with resource file %s", filename))
		template := apiserverAuthFixture(filename)
		params := []string{"-f", template, "-p", fmt.Sprintf("POD_LIMIT=%d", limits.podLimit), fmt.Sprintf("RQ_LIMIT=%d", limits.resourcequotaLimit), fmt.Sprintf("SECRET_LIMIT=%d", limits.secretLimit), fmt.Sprintf("SERVICE_LIMIT=%d", limits.serviceLimit), fmt.Sprintf("CM_LIMIT=%d", limits.configmapLimit), fmt.Sprintf("NAME=%s", quotaName)}
		configFile := compat_otp.ProcessTemplate(oc, params...)
		err = oc.AsAdmin().Run("create").Args("-f", configFile, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("4) Wait for quota to show up in command describe")
		quotaDescribeErr := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 20*time.Second, false, func(cxt context.Context) (bool, error) {
			describeOutput, err := oc.Run("describe").Args("quota", quotaName, "-n", namespace).Output()
			if isMatched, matchErr := regexp.Match("secrets.*[0-9]", []byte(describeOutput)); isMatched && matchErr == nil && err == nil {
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(quotaDescribeErr, "quota did not show up")

		g.By(fmt.Sprintf("5) Create multiple secrets with resource file %s, expect failure for secert creations that exceed quota limit", filename))
		for i := 1; i <= limit+1; i++ {
			secretName := fmt.Sprintf("ocp12360-secret-%d", i)
			output, err := oc.Run("create").Args("secret", "generic", secretName, "--from-literal=testkey=testvalue", "-n", namespace).Output()
			if i <= limit {
				g.By(fmt.Sprintf("5.%d) creating secret %s, within quota limit, expect success", i, secretName))
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				g.By(fmt.Sprintf("5.%d) creating secret %s, exceeds quota limit, expect failure", i, secretName))
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(output).To(o.MatchRegexp("secrets.*forbidden: exceeded quota"))
			}
		}

		filename = "ocp12360-pod.yaml"
		g.By(fmt.Sprintf("6) Create multiple pods with resource file %s, expect failure for pod creations that exceed quota limit", filename))
		template = apiserverAuthFixture(filename)
		for i := 1; i <= limit+1; i++ {
			podName := fmt.Sprintf("ocp12360-pod-%d", i)
			configFile := compat_otp.ProcessTemplate(oc, "-f", template, "-p", "NAME="+podName)
			output, err := oc.Run("create").Args("-f", configFile, "-n", namespace).Output()
			if i <= limit {
				g.By(fmt.Sprintf("6.%d) creating pod %s, within quota limit, expect success", i, podName))
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				g.By(fmt.Sprintf("6.%d) creating pod %s, exceeds quota limit, expect failure", i, podName))
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(output).To(o.MatchRegexp("pods.*forbidden: exceeded quota"))
			}
		}

		g.By(fmt.Sprintf("7) Create multiple services with resource file %s, expect failure for resource creations that exceed quota limit", filename))
		for i := 1; i <= limit+1; i++ {
			serviceName := fmt.Sprintf("ocp12360-service-%d", i)
			externalName := fmt.Sprintf("ocp12360-external-name-%d", i)
			output, err := oc.Run("create").Args("service", "externalname", serviceName, "-n", namespace, "--external-name", externalName).Output()
			if i <= limit {
				g.By(fmt.Sprintf("7.%d) creating service %s, within quota limit, expect success", i, serviceName))
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				g.By(fmt.Sprintf("7.%d) creating service %s, exceeds quota limit, expect failure", i, serviceName))
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(output).To(o.MatchRegexp("services.*forbidden: exceeded quota"))
			}
		}

		filename = "ocp12360-quota.yaml"
		g.By(fmt.Sprintf("8) Create multiple quota with resource file %s, expect failure for quota creations that exceed quota limit", filename))
		template = apiserverAuthFixture(filename)
		for i := 1; i <= limit+1; i++ {
			quotaName := fmt.Sprintf("ocp12360-quota-%d", i)
			params := []string{"-f", template, "-p", fmt.Sprintf("POD_LIMIT=%d", limits.podLimit), fmt.Sprintf("RQ_LIMIT=%d", limits.resourcequotaLimit), fmt.Sprintf("SECRET_LIMIT=%d", limits.secretLimit), fmt.Sprintf("SERVICE_LIMIT=%d", limits.serviceLimit), fmt.Sprintf("CM_LIMIT=%d", limits.configmapLimit), fmt.Sprintf("NAME=%s", quotaName)}
			configFile := compat_otp.ProcessTemplate(oc, params...)
			output, err := oc.AsAdmin().Run("create").Args("-f", configFile, "-n", namespace).Output()
			if i <= limit {
				g.By(fmt.Sprintf("8.%d) creating quota %s, within quota limit, expect success", i, quotaName))
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				g.By(fmt.Sprintf("8.%d) creating quota %s, exceeds quota limit, expect failure", i, quotaName))
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(output).To(o.MatchRegexp("resourcequotas.*forbidden: exceeded quota"))
			}
		}

		g.By(fmt.Sprintf("9) Create multiple configmaps with resource file %s, expect failure for configmap creations that exceed quota limit", filename))
		for i := 1; i <= limit+1; i++ {
			configmapName := fmt.Sprintf("ocp12360-configmap-%d", i)
			output, err := oc.Run("create").Args("configmap", configmapName, "-n", namespace).Output()
			if i <= limit {
				g.By(fmt.Sprintf("9.%d) creating configmap %s, within quota limit, expect success", i, configmapName))
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				g.By(fmt.Sprintf("9.%d) creating configmap %s, exceeds quota limit, expect failure", i, configmapName))
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(output).To(o.MatchRegexp("configmaps.*forbidden: exceeded quota"))
			}
		}
	})

	g.It("[OTP][OCP-11289] Check the imagestreams of quota in the project after build image [ConnectedOnly][Serial]", ote.Informing(), func() {
		if isBaselineCapsSet(oc) && !(isEnabledCapability(oc, "Build") && isEnabledCapability(oc, "DeploymentConfig")) {
			g.Skip("Skipping the test as baselinecaps have been set and some of API capabilities are not enabled!")
		}

		var (
			ocpObjectCountsYamlFile = tmpdir + "openshift-object-counts.yaml"
			expectedQuota           = "openshift.io/imagestreams:2"
		)
		g.By("1) Create a new project required for this test execution")
		oc.SetupProject()
		namespace := oc.Namespace()

		g.By("2) Create a ResourceQuota count of image stream")
		ocpObjectCountsYaml := `apiVersion: v1
kind: ResourceQuota
metadata:
  name: openshift-object-counts
spec:
  hard:
    openshift.io/imagestreams: "10"
`
		f, err := os.Create(ocpObjectCountsYamlFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer f.Close()
		w := bufio.NewWriter(f)
		_, err = fmt.Fprintf(w, "%s", ocpObjectCountsYaml)
		w.Flush()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer oc.AsAdmin().Run("delete").Args("-f", ocpObjectCountsYamlFile, "-n", namespace).Execute()
		quotaErr := oc.AsAdmin().Run("create").Args("-f", ocpObjectCountsYamlFile, "-n", namespace).Execute()
		o.Expect(quotaErr).NotTo(o.HaveOccurred())

		g.By("3. Checking the created Resource Quota of the Image Stream")
		quota := getResourceToBeReady(oc, asAdmin, withoutNamespace, "quota", "openshift-object-counts", `--template={{.status.used}}`, "-n", namespace)
		o.Expect(quota).Should(o.ContainSubstring("openshift.io/imagestreams:0"), "openshift-object-counts")

		checkImageStreamQuota := func(buildName string, step string) {
			buildErr := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 90*time.Second, false, func(cxt context.Context) (bool, error) {
				bs := getResourceToBeReady(oc, asAdmin, withoutNamespace, "builds", buildName, "-ojsonpath={.status.phase}", "-n", namespace)
				if strings.Contains(bs, "Complete") {
					e2e.Logf("Building of %s status:%v", buildName, bs)
					return true, nil
				}
				e2e.Logf("Building of %s is still not complete, continue to monitor ...", buildName)
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(buildErr, fmt.Sprintf("ERROR: Build status of %s is not complete!", buildName))

			g.By(fmt.Sprintf("%s.1 Checking the created Resource Quota of the Image Stream", step))
			quota := getResourceToBeReady(oc, asAdmin, withoutNamespace, "quota", "openshift-object-counts", `--template={{.status.used}}`, "-n", namespace)

			if !strings.Contains(quota, expectedQuota) {
				out, _ := getResource(oc, asAdmin, withoutNamespace, "imagestream", "-n", namespace)
				e2e.Logf("imagestream are used: %s", out)
				e2e.Failf("expected quota openshift-object-counts %s doesn't match the reality %s! Please check!", expectedQuota, quota)
			}
		}

		g.By("4. Create a source build using source code and check the build info")
		imgErr := oc.AsAdmin().WithoutNamespace().Run("new-build").Args(`quay.io/openshifttest/ruby-27:1.2.0~https://github.com/sclorg/ruby-ex.git`, "-n", namespace, "--import-mode=PreserveOriginal").Execute()
		o.Expect(imgErr).NotTo(o.HaveOccurred())
		checkImageStreamQuota("ruby-ex-1", "4")

		g.By("5. Starts a new build for the provided build config")
		sbErr := oc.AsAdmin().WithoutNamespace().Run("start-build").Args("ruby-ex", "-n", namespace).Execute()
		o.Expect(sbErr).NotTo(o.HaveOccurred())
		checkImageStreamQuota("ruby-ex-2", "5")
	})
})
