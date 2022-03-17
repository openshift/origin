package images

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	imageapi "github.com/openshift/api/image/v1"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/imageregistryutil"
	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

var _ = g.Describe("[sig-imageregistry] Image redirect", func() {
	defer g.GinkgoRecover()

	routeNamespace := "openshift-image-registry"
	routeName := "test-image-redirect"
	var oc *exutil.CLI
	var ns string

	g.BeforeEach(func() {
		oc.AdminRouteClient().RouteV1().Routes(routeNamespace).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		_, err := imageregistryutil.ExposeImageRegistry(context.TODO(), oc.AdminRouteClient(), routeName)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func() {
		oc.AdminRouteClient().RouteV1().Routes(routeNamespace).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		if g.CurrentGinkgoTestDescription().Failed && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLI("image-redirect")

	g.It("should redirect an image pull on GCP", func() {
		ctx := context.TODO()
		infrastructure, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infrastructure.Status.PlatformStatus == nil {
			g.Fail("missing platform")
		}
		if infrastructure.Status.PlatformStatus.Type != configv1.GCPPlatformType {
			g.Skip("only run on GCP")
		}
		imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if imageRegistryConfig.Spec.DisableRedirect {
			g.Skip("only run when using redirect")
		}
		if imageRegistryConfig.Spec.Storage.GCS == nil {
			g.Skip("only run when using GCS")
		}

		ns = oc.Namespace()
		client := oc.ImageClient().ImageV1()

		// import tools:latest into this namespace - working around a pull through bug with referenced docker images
		// https://bugzilla.redhat.com/show_bug.cgi?id=1843253
		_, err = client.ImageStreamTags(ns).Create(context.Background(), &imageapi.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{Name: "1:tools"},
			Tag: &imageapi.TagReference{
				From: &kapi.ObjectReference{Kind: "ImageStreamTag", Namespace: "openshift", Name: "tools:latest"},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		isi, err := client.ImageStreamImports(ns).Create(context.Background(), &imageapi.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Name: "1",
			},
			Spec: imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: strings.Split(k8simage.GetE2EImage(k8simage.Agnhost), "/")[0]},
					ReferencePolicy: imageapi.TagReferencePolicy{
						Type: imageapi.LocalTagReferencePolicy,
					},
				},
				Images: []imageapi.ImageImportSpec{
					{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: k8simage.GetE2EImage(k8simage.Agnhost)},
						To:   &kapi.LocalObjectReference{Name: "mysql"},
					},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		for i, img := range isi.Status.Images {
			o.Expect(img.Status.Status).To(o.Equal("Success"), fmt.Sprintf("imagestreamimport status for spec.image[%d] (message: %s)", i, img.Status.Message))
		}
		imageLayer := ""
		for _, img := range isi.Status.Images {
			if img.Image != nil {
				if len(img.Image.DockerImageLayers) > 0 {
					imageLayer = img.Image.DockerImageLayers[len(img.Image.DockerImageLayers)-1].Name
				}
			}
		}

		// I've now got an imported image, I need to do something equivalent to
		// curl -v -H "Authorization: Basic $BUILD02"   https://registry.build02.ci.openshift.org/v2/openshift/tests/blobs/sha256:b46897b86ca27977cf8e71c558d5893166db6b9b944298510265660dc17b3e44
		route, err := oc.AdminRouteClient().RouteV1().Routes(routeNamespace).Get(context.TODO(), routeName, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		url := fmt.Sprintf("https://%s/v2/%s/1/blobs/%s",
			route.Status.Ingress[0].Host,
			oc.Namespace(),
			imageLayer,
		)

		// get the token to use to pull the data from the registry.
		imageRegistryToken := ""
		sa, err := oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()).Get(ctx, "builder", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sa.ImagePullSecrets).NotTo(o.BeEmpty())
		pullSecretName := sa.ImagePullSecrets[0].Name
		pullSecret, err := oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Get(ctx, pullSecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		dockerCfg := credentialprovider.DockerConfig{}
		err = json.Unmarshal(pullSecret.Data[".dockercfg"], &dockerCfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, auth := range dockerCfg {
			imageRegistryToken = auth.Password // these should all have the same value
			break
		}
		framework.Logf("using token: %q", imageRegistryToken)

		// create a custom http client so we can reliably detect the redirects
		httpClient := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // this token will be gone shortly and if someone can intercept the router in a CI cluster, they can intercept a lot.
				},
			},
			// this prevents the client from automatically following redirects
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		request, err := http.NewRequest("GET", url, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		request.Header.Set("Authorization", "Bearer "+imageRegistryToken)
		request.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

		response, err := httpClient.Do(request)
		o.Expect(err).NotTo(o.HaveOccurred())

		// only dump the body if we didn't get a 200-series.  If we got a 200 series, chances are that it will be quite large.
		dumpBody := false
		if response.StatusCode > 299 || response.StatusCode < 200 {
			dumpBody = true
		}
		bodyDump, err := httputil.DumpResponse(response, dumpBody)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("response: %v", string(bodyDump))
		o.Expect(response.StatusCode).To(
			o.Equal(http.StatusTemporaryRedirect),
			fmt.Sprintf("GET from %s must be %v for performance reasons; got %v", url, http.StatusTemporaryRedirect, response.StatusCode),
		)

		redirectLocation := response.Header["Location"][0]
		if !strings.Contains(redirectLocation, "storage.googleapis.com") {
			g.Fail(fmt.Sprintf("expected redirect to google storage, but got %v", response.Header["Location"]))
		}
	})
})
