package images

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const podStartupTimeout = 3 * time.Minute

func getRandName() string {
	c := 20
	b := make([]byte, c)

	_, err := rand.Read(b)
	o.Expect(err).NotTo(o.HaveOccurred())

	return fmt.Sprintf("%02x", b)
}

func getCreds(oc *exutil.CLI) (string, string) {
	user, err := oc.Run("whoami").Args().Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	token, err := oc.Run("whoami").Args("-t").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	return strings.TrimSpace(user), strings.TrimSpace(token)
}

type testPod struct {
	oc   *exutil.CLI
	name string
	spec string
}

func NewTestPod(oc *exutil.CLI, name, spec string) *testPod {
	return &testPod{
		oc:   oc,
		name: name,
		spec: spec,
	}
}

func (pod *testPod) syncState(c kclientset.Interface, ns string, timeout time.Duration, state kapiv1.PodPhase) (err error) {
	label := labels.SelectorFromSet(map[string]string{
		"name": pod.name,
	})

	err = wait.Poll(2*time.Second, timeout,
		func() (bool, error) {
			podList, err := frameworkpod.WaitForPodsWithLabel(context.TODO(), c, ns, label)
			if err != nil {
				framework.Logf("Failed getting pods: %v", err)
				return false, nil // Ignore this error (nil) and try again in "Poll" time
			}
			pods := podList.Items

			if pods == nil || len(pods) == 0 {
				return true, nil
			}
			for _, p := range pods {
				if p.Status.Phase != state {
					return false, nil
				}
			}
			return true, nil
		})
	return err
}

func (pod *testPod) syncRunning(c kclientset.Interface, ns string, timeout time.Duration) (err error) {
	err = pod.syncState(c, ns, timeout, kapiv1.PodRunning)
	if err == nil {
		framework.Logf("All pods running in %s", ns)
	}
	return err
}

func (pod *testPod) NotErr(err error) error {
	if err != nil {
		exutil.DumpPodLogsStartingWithInNamespace(pod.name, pod.oc.Namespace(), pod.oc)
	}
	return err
}

func (pod *testPod) Run() *testPod {
	g.By("Set up and fetch the URL of external registry server")

	err := pod.oc.Run("create").Args("-f", "-").InputString(pod.spec).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(pod.NotErr(pod.syncRunning(pod.oc.AdminKubeClient(), pod.oc.Namespace(), podStartupTimeout))).NotTo(o.HaveOccurred())
	return pod
}

func (pod *testPod) ShellExec(command ...string) *exutil.CLI {
	args := []string{}

	args = append(args, pod.name, "-i", "-c", "shell", "--")
	for _, a := range command {
		args = append(args, a)
	}

	return pod.oc.Run("exec").Args(args...)
}

func genDockerConfig(pod *testPod, registryURL, user, token string) {
	config := fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, registryURL, base64.StdEncoding.EncodeToString([]byte(user+":"+token)))
	framework.Logf("Config file: %s", config)
	err := pod.ShellExec("bash", "-c", "cd /tmp; cat > config.json").InputString(config).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getRegistrySchema(pod *testPod, host string) (schema string, err error) {
	for _, schema = range []string{"https", "http"} {
		framework.Logf("try %s protocol", schema)

		err = pod.ShellExec("curl", "-s", "-k", "-L", "-o", "/dev/null", fmt.Sprintf("%s://%s/v2/", schema, host)).Execute()
		if err == nil {
			return
		}
	}
	return
}

func runHTTPRequest(pod *testPod, URL string, headers map[string]string) (string, error) {
	command := []string{"curl", "-s", "-v", "-k", "-L", "-o", "/dev/null"}
	for k, v := range headers {
		command = append(command, "-H", fmt.Sprintf("%s", k+":"+v))
	}
	command = append(command, URL)
	return pod.ShellExec(command...).Output()
}

func requestHasStatusCode(pod *testPod, URL, token, statusCode string) {
	headers := map[string]string{}
	if len(token) > 0 {
		headers["Authorization"] = "bearer " + token
	}

	out, err := runHTTPRequest(pod, URL, headers)
	o.Expect(err).NotTo(o.HaveOccurred())

	m := regexp.MustCompile(fmt.Sprintf(`(?m)^< HTTP/(?:1\.1|2) %s `, statusCode)).FindString(out)
	if len(m) == 0 {
		err = fmt.Errorf("unexpected status code (expected %s): %s", statusCode, out)
	}
	o.Expect(pod.NotErr(err)).NotTo(o.HaveOccurred())
}

func testNewBuild(oc *exutil.CLI) (string, string) {
	isName := "mirror-" + getRandName()
	istName := isName + ":latest"

	testDockerfile := fmt.Sprintf(`FROM %[4]s
RUN echo %[1]s > /1
RUN echo %[2]s > /2
RUN echo %[3]s > /3
`,
		getRandName(),
		getRandName(),
		getRandName(),
		image.ShellImage(),
	)

	err := oc.Run("new-build").Args("-D", "-", "--to", istName).InputString(testDockerfile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("starting a test build")
	bc, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Get(context.Background(), isName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(*bc.Spec.Source.Dockerfile).To(o.Equal(testDockerfile))

	g.By("expecting the Dockerfile build is in Complete phase")
	err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), isName+"-1", nil, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("checking for the imported tag: %s", istName))
	ist, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get(context.Background(), istName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return isName, ist.Image.Name
}

var _ = g.Describe("[sig-imageregistry][Feature:ImageMirror][Slow] Image mirror", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("image-mirror", admissionapi.LevelBaseline)

	g.It("mirror image from integrated registry to external registry [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		g.By("get user credentials")
		user, token := getCreds(oc)

		g.By("fetch the URL of integrated registry server")
		registryHost, err := GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set up and fetch the URL of external registry server")
		var testPod1 = fmt.Sprintf(
			`kind: Pod
id: oc-image-mirror-test-1
apiVersion: v1
metadata:
  name: oc-image-mirror-test-1
  labels:
    name: oc-image-mirror-test-1
spec:
  containers:
  - name: registry-1
    image: %[1]s
    env:
    - name: REGISTRY_HTTP_ADDR
      value: :5001
    - name: REGISTRY_HTTP_NET
      value: tcp
    - name: REGISTRY_LOGLEVEL
      value: debug
    - name: REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY
      value: /tmp
  - name: shell
    image: %[2]s
    command:
    - /bin/sleep
    - infinity
`,
			image.LocationFor("docker.io/library/registry:2.8.0-beta.1"),
			image.ShellImage(),
		)
		pod := NewTestPod(oc, "oc-image-mirror-test-1", testPod1).Run()

		g.By("get the protocol of integrated registry server")
		schema, err := getRegistrySchema(pod, registryHost)
		o.Expect(pod.NotErr(err)).NotTo(o.HaveOccurred())

		framework.Logf("the protocol is %s://%s", schema, registryHost)

		g.By("generate config.json for integrated registry server")
		genDockerConfig(pod, registryHost, user, token)

		g.By("calling oc new-build with Dockerfile")
		isName, imgName := testNewBuild(oc)

		repoName := oc.Namespace() + "/" + isName

		g.By("Check that we have it in the integrated registry server")
		requestHasStatusCode(pod, fmt.Sprintf("%s://%s/v2/%s/manifests/%s", schema, registryHost, repoName, imgName), token, "200")

		g.By("Check that we do not have it in the external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5001/v2/%s/manifests/%s", repoName, imgName), "", "404")

		g.By("Mirror image from the integrated registry server to the external registry server")
		command := fmt.Sprintf("cd /tmp; oc image mirror %s/%s:latest %s/%s:stable --insecure=true --registry-config config.json",
			registryHost, repoName, "127.0.0.1:5001", repoName,
		)
		err = pod.ShellExec([]string{"bash", "-c", command}...).Execute()
		o.Expect(pod.NotErr(err)).NotTo(o.HaveOccurred())

		g.By("Check that we have it in the external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5001/v2/%s/manifests/%s", repoName, imgName), "", "200")

		g.By("remove external registry pods")
		err = exutil.RemovePodsWithPrefixes(oc, pod.name)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("mirror image from integrated registry into few external registries [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		g.By("get user credentials")
		user, token := getCreds(oc)

		g.By("fetch the URL of integrated registry server")
		registryHost, err := GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set up and fetch the URL of external registry server")
		var testPod2 = fmt.Sprintf(
			`kind: Pod
id: oc-image-mirror-test-2
apiVersion: v1
metadata:
  name: oc-image-mirror-test-2
  labels:
    name: oc-image-mirror-test-2
spec:
  containers:
  - name: registry-1
    image: %[1]s
    env:
    - name: REGISTRY_HTTP_ADDR
      value: :5002
    - name: REGISTRY_HTTP_NET
      value: tcp
    - name: REGISTRY_LOGLEVEL
      value: debug
    - name: REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY
      value: /tmp
  - name: registry-2
    image: %[1]s
    env:
    - name: REGISTRY_HTTP_ADDR
      value: :5003
    - name: REGISTRY_HTTP_NET
      value: tcp
    - name: REGISTRY_LOGLEVEL
      value: debug
    - name: REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY
      value: /tmp
  - name: shell
    image: %[2]s
    command:
    - /bin/sleep
    - infinity
`,
			image.LocationFor("docker.io/library/registry:2.8.0-beta.1"),
			image.ShellImage(),
		)
		pod := NewTestPod(oc, "oc-image-mirror-test-2", testPod2).Run()

		g.By("get the protocol of integrated registry server")
		schema, err := getRegistrySchema(pod, registryHost)
		o.Expect(pod.NotErr(err)).NotTo(o.HaveOccurred())

		framework.Logf("the protocol is %s://%s", schema, registryHost)

		g.By("generate config.json for integrated registry server")
		genDockerConfig(pod, registryHost, user, token)

		g.By("calling oc new-build with Dockerfile")
		isName, imgName := testNewBuild(oc)

		repoName := oc.Namespace() + "/" + isName

		g.By("Check that we have it in the integrated registry server")
		requestHasStatusCode(pod, fmt.Sprintf("%s://%s/v2/%s/manifests/%s", schema, registryHost, repoName, imgName), token, "200")

		g.By("Check that we do not have it in the first external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5002/v2/%s/manifests/%s", repoName, imgName), "", "404")

		g.By("Check that we do not have it in the second external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5003/v2/%s/manifests/%s", repoName, imgName), "", "404")

		g.By("Mirror image from the integrated registry server to the external registry server")
		command := fmt.Sprintf("cd /tmp; oc image mirror %s/%s:latest %s/%s:stable %s/%s:prod --insecure=true --registry-config config.json",
			registryHost, repoName,
			"127.0.0.1:5002", repoName,
			"127.0.0.1:5003", repoName,
		)
		err = pod.ShellExec([]string{"bash", "-c", command}...).Execute()
		o.Expect(pod.NotErr(err)).NotTo(o.HaveOccurred())

		g.By("Check that we have it in the first external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5002/v2/%s/manifests/%s", repoName, imgName), "", "200")

		g.By("Check that we have it in the second external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5003/v2/%s/manifests/%s", repoName, imgName), "", "200")

		g.By("remove external registry pods")
		err = exutil.RemovePodsWithPrefixes(oc, pod.name)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
