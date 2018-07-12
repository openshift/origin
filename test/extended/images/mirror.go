package images

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const podStartupTimeout = 3 * time.Minute
const testPod1 = `kind: Pod
id: oc-image-mirror-test-1
apiVersion: v1
metadata:
  name: oc-image-mirror-test-1
  labels:
    name: oc-image-mirror-test-1
spec:
  containers:
  - name: registry-1
    image: docker.io/library/registry
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
    image: openshift/origin:latest
    command:
    - /bin/sleep
    - infinity
`

const testPod2 = `kind: Pod
id: oc-image-mirror-test-2
apiVersion: v1
metadata:
  name: oc-image-mirror-test-2
  labels:
    name: oc-image-mirror-test-2
spec:
  containers:
  - name: registry-1
    image: docker.io/library/registry
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
    image: docker.io/library/registry
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
    image: openshift/origin:latest
    command:
    - /bin/sleep
    - infinity
`

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
			podList, err := framework.WaitForPodsWithLabel(c, ns, label)
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

func (pod *testPod) NotErr(err error) {
	if err != nil {
		exutil.DumpPodLogsStartingWithInNamespace(pod.name, pod.oc.Namespace(), pod.oc)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *testPod) Run() *testPod {
	g.By("Set up and fetch the URL of external registry server")

	err := pod.oc.Run("create").Args("-f", "-").InputString(pod.spec).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	pod.NotErr(pod.syncRunning(pod.oc.AdminKubeClient(), pod.oc.Namespace(), podStartupTimeout))
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

	m := regexp.MustCompile(fmt.Sprintf(`(?m)^< HTTP/1\.1 %s `, statusCode)).FindString(out)
	if len(m) == 0 {
		err = fmt.Errorf("unexpected status code (expected %s): %s", statusCode, out)
	}
	pod.NotErr(err)
}

func testNewBuild(oc *exutil.CLI) (string, string) {
	isName := "mirror-" + getRandName()
	istName := isName + ":latest"

	testDockerfile := fmt.Sprintf(`FROM busybox:latest
RUN echo %s > /1
RUN echo %s > /2
RUN echo %s > /3
`,
		getRandName(),
		getRandName(),
		getRandName(),
	)

	err := oc.Run("new-build").Args("-D", "-", "--to", istName).InputString(testDockerfile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("starting a test build")
	bc, err := oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get(isName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(*bc.Spec.Source.Dockerfile).To(o.Equal(testDockerfile))

	g.By("expecting the Dockerfile build is in Complete phase")
	err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), isName+"-1", nil, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("checking for the imported tag: %s", istName))
	ist, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get(istName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return isName, ist.Image.Name
}

var _ = g.Describe("[Feature:ImageMirror][registry][Slow] Image mirror", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("image-mirror", exutil.KubeConfigPath())

	g.It("mirror image from integrated registry to external registry", func() {
		g.By("get user credentials")
		user, token := getCreds(oc)

		g.By("fetch the URL of integrated registry server")
		registryHost, err := GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set up and fetch the URL of external registry server")
		pod := NewTestPod(oc, "oc-image-mirror-test-1", testPod1).Run()

		g.By("get the protocol of integrated registry server")
		schema, err := getRegistrySchema(pod, registryHost)
		pod.NotErr(err)

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
		command := fmt.Sprintf("cd /tmp; oc --loglevel=8 image mirror %s/%s:latest %s/%s:stable --insecure=true",
			registryHost, repoName, "127.0.0.1:5001", repoName,
		)
		err = pod.ShellExec([]string{"bash", "-c", command}...).Execute()
		pod.NotErr(err)

		g.By("Check that we have it in the external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5001/v2/%s/manifests/%s", repoName, imgName), "", "200")

		g.By("remove external registry pods")
		err = exutil.RemovePodsWithPrefixes(oc, pod.name)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("mirror image from integrated registry into few external registries", func() {
		g.By("get user credentials")
		user, token := getCreds(oc)

		g.By("fetch the URL of integrated registry server")
		registryHost, err := GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set up and fetch the URL of external registry server")
		pod := NewTestPod(oc, "oc-image-mirror-test-2", testPod2).Run()

		g.By("get the protocol of integrated registry server")
		schema, err := getRegistrySchema(pod, registryHost)
		pod.NotErr(err)

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
		command := fmt.Sprintf("cd /tmp; oc image mirror %s/%s:latest %s/%s:stable %s/%s:prod --insecure=true",
			registryHost, repoName,
			"127.0.0.1:5002", repoName,
			"127.0.0.1:5003", repoName,
		)
		err = pod.ShellExec([]string{"bash", "-c", command}...).Execute()
		pod.NotErr(err)

		g.By("Check that we have it in the first external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5002/v2/%s/manifests/%s", repoName, imgName), "", "200")

		g.By("Check that we have it in the second external registry server")
		requestHasStatusCode(pod, fmt.Sprintf("http://127.0.0.1:5003/v2/%s/manifests/%s", repoName, imgName), "", "200")

		g.By("remove external registry pods")
		err = exutil.RemovePodsWithPrefixes(oc, pod.name)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
