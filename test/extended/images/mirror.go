package images

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	knet "k8s.io/apimachinery/pkg/util/net"

	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/handlers"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"

	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

func createTestRegistryServer(ctx context.Context) *httptest.Server {
	remoteRegistryApp := handlers.NewApp(ctx, &configuration.Configuration{
		Loglevel: "debug",
		Storage: configuration.Storage{
			"inmemory": configuration.Parameters{},
		},
	})
	return httptest.NewServer(remoteRegistryApp)
}

func getRandName() string {
	c := 20
	b := make([]byte, c)

	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return fmt.Sprintf("%02x", b)
}

func httpGet(registryURL, repository, manifestDigest string) (content []byte, statusCode int, reqErr error) {
	c := http.Client{
		Transport: knet.SetTransportDefaults(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}),
	}

	for _, schema := range []string{"https", "http"} {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s/v2/%s/manifests/%s", schema, registryURL, repository, manifestDigest), nil)
		if err != nil {
			reqErr = fmt.Errorf("failed to make request: %v", err)
			continue
		}

		resp, err := c.Do(req)
		if err != nil {
			reqErr = fmt.Errorf("failed to do the request: %v", err)
			continue
		}
		defer resp.Body.Close()

		content, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			reqErr = fmt.Errorf("failed to read body: %v", err)
			continue
		}

		statusCode = resp.StatusCode
		reqErr = nil
		return
	}
	return
}

func writeConfig(oc *exutil.CLI, registryURL, destdir string) error {
	user, err := oc.Run("whoami").Args().Output()
	if err != nil {
		return fmt.Errorf("unable to get current username: %v", err)
	}

	token, err := oc.Run("whoami").Args("-t").Output()
	if err != nil {
		return fmt.Errorf("unable to get token: %v", err)
	}

	config := fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, registryURL, base64.StdEncoding.EncodeToString([]byte(user+":"+token)))

	return ioutil.WriteFile(filepath.Join(destdir, "config.json"), []byte(config), 0644)
}

var _ = g.Describe("[Feature:ImageMirror][registry] Image mirror", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("prune-images", exutil.KubeConfigPath())

	g.It("mirror image from integrated registry to external registry", func() {
		g.By("Fetch the URL of integrated registry server")
		registryURL, err := GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Generate config.json for integrated registry server")
		tempdir, err := ioutil.TempDir("", "image-mirror-")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempdir)

		err = writeConfig(oc, registryURL, tempdir)
		o.Expect(err).NotTo(o.HaveOccurred())

		cwd, err := os.Getwd()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = os.Chdir(tempdir)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Chdir(cwd)

		g.By("Set up and fetch the URL of external registry server")
		externalRegistry := createTestRegistryServer(context.Background())
		defer externalRegistry.Close()

		externalRegistryURL, err := url.Parse(externalRegistry.URL)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Build and push random image into the integrated registry server")
		isName := "mirror-" + getRandName()
		repoName := oc.Namespace() + "/" + isName

		outSink := g.GinkgoWriter

		dClient, err := testutil.NewDockerClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		imgName, _, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, isName, "latest", testImageSize, 2, outSink, true, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check that we have it in the integrated registry server")
		_, _, err = httpGet(registryURL, repoName, imgName)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check that we do not have it in the external registry server")
		_, statusCode, err := httpGet(externalRegistryURL.Host, repoName, imgName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statusCode).To(o.Equal(404))

		g.By("Mirror image from the integrated registry server to the external registry server")
		err = oc.Run("image").Args("mirror",
			fmt.Sprintf("%s/%s:latest", registryURL, repoName),
			fmt.Sprintf("%s/%s:stable", externalRegistryURL.Host, repoName),
			"--insecure=true",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check that we have it in the external registry server")
		_, statusCode, err = httpGet(externalRegistryURL.Host, repoName, imgName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(statusCode).To(o.Equal(200))
	})

	g.It("mirror image from integrated registry into few external registries", func() {
		g.By("Fetch the URL of integrated registry server")
		registryURL, err := GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Generate config.json for integrated registry server")
		tempdir, err := ioutil.TempDir("", "image-mirror-")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempdir)

		err = writeConfig(oc, registryURL, tempdir)
		o.Expect(err).NotTo(o.HaveOccurred())

		cwd, err := os.Getwd()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = os.Chdir(tempdir)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Chdir(cwd)

		g.By("Set up and fetch the URL of first external registry server")
		externalRegistry0 := createTestRegistryServer(context.Background())
		defer externalRegistry0.Close()

		externalRegistry0URL, err := url.Parse(externalRegistry0.URL)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Set up and fetch the URL of second external registry server")
		externalRegistry1 := createTestRegistryServer(context.Background())
		defer externalRegistry1.Close()

		externalRegistry1URL, err := url.Parse(externalRegistry1.URL)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Build and push random image into the integrated registry server")
		isName := "mirror-" + getRandName()
		repoName := oc.Namespace() + "/" + isName

		outSink := g.GinkgoWriter

		dClient, err := testutil.NewDockerClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		imgName, _, err := BuildAndPushImageOfSizeWithDocker(oc, dClient, isName, "latest", testImageSize, 2, outSink, true, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check that we have it in the integrated registry server")
		_, _, err = httpGet(registryURL, repoName, imgName)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check that we do not have it in the external registry server")
		for _, host := range []string{externalRegistry0URL.Host, externalRegistry1URL.Host} {
			_, statusCode, err := httpGet(host, repoName, imgName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(statusCode).To(o.Equal(404))
		}

		g.By("Mirror image from the integrated registry server to the external registry server")
		err = oc.Run("image").Args("mirror",
			fmt.Sprintf("%s/%s:latest=%s/%s:prod", registryURL, repoName, externalRegistry0URL.Host, repoName),
			fmt.Sprintf("%s/%s:latest=%s/%s:stable", registryURL, repoName, externalRegistry1URL.Host, repoName),
			"--insecure=true",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Check that we have it in the external registry server")
		for _, host := range []string{externalRegistry0URL.Host, externalRegistry1URL.Host} {
			_, statusCode, err := httpGet(host, repoName, imgName)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(statusCode).To(o.Equal(200))
		}
	})
})
