package images

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	configv1 "github.com/openshift/api/config/v1"
	imageregistryv1 "github.com/openshift/api/imageregistry/v1"
	routev1 "github.com/openshift/api/route/v1"
	config "github.com/openshift/client-go/config/clientset/versioned"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/imageregistryutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/test/e2e/framework"
)

func storageSupportsRedirects(storage imageregistryv1.ImageRegistryConfigStorage, authentication *configv1.Authentication) bool {
	// GCP only supports redirects with permanent credentials. Short lived credentials (GCP workload identity) are being used
	// when the cluster authentication config contains a ServiceAccountIssuer that points to an OIDC endpoint rather than
	// empty string.
	return storage.S3 != nil || (storage.GCS != nil && authentication.Spec.ServiceAccountIssuer == "") || storage.Azure != nil || storage.Swift != nil || storage.OSS != nil
}

var _ = g.Describe("[sig-imageregistry] Image registry [apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()

	routeNamespace := "openshift-image-registry"
	routeName := "test-image-redirect"
	var oc *exutil.CLI
	var ns string
	var route *routev1.Route

	g.BeforeEach(func() {
		var err error
		oc.AdminRouteClient().RouteV1().Routes(routeNamespace).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		route, err = imageregistryutil.ExposeImageRegistry(context.TODO(), oc.AdminRouteClient(), routeName)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func() {
		oc.AdminRouteClient().RouteV1().Routes(routeNamespace).Delete(context.TODO(), routeName, metav1.DeleteOptions{})
		if g.CurrentSpecReport().Failed() && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLI("image-redirect")

	g.It("should redirect on blob pull [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		ctx := context.TODO()
		imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if imageRegistryConfig.Spec.DisableRedirect {
			g.Skip("only run when using redirect")
		}

		configClient, err := config.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		authenticationConfig, err := configClient.ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if !storageSupportsRedirects(imageRegistryConfig.Spec.Storage, authenticationConfig) {
			g.Skip("only run when configured image registry storage supports redirects")
		}

		ns = oc.Namespace()

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
				// Use the HTTP proxy configured in the environment variables.
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // this token will be gone shortly and if someone can intercept the router in a CI cluster, they can intercept a lot.
				},
			},
			// this prevents the client from automatically following redirects
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		request, err := http.NewRequest(
			"POST",
			fmt.Sprintf(
				"https://%s/v2/%s/%s/blobs/uploads/",
				route.Status.Ingress[0].Host,
				oc.Namespace(),
				"repo",
			),
			nil,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		request.Header.Set("Authorization", "Bearer "+imageRegistryToken)
		response, err := httpClient.Do(request)
		o.Expect(err).NotTo(o.HaveOccurred())

		bodyDump, err := httputil.DumpResponse(response, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("response from %s %s: %v", request.Method, request.URL, string(bodyDump))

		o.Expect(response.StatusCode).To(
			o.Equal(http.StatusAccepted),
			fmt.Sprintf("%s from %s must be %v, got %v", request.Method, request.URL, http.StatusAccepted, response.StatusCode),
		)

		content := "Hello, world!"
		digest := digest.FromString(content)

		location := response.Header.Get("Location")
		if strings.Contains(location, "?") {
			location += "&"
		} else {
			location += "?"
		}
		location += "digest=" + digest.String()

		request, err = http.NewRequest(
			"PUT",
			location,
			strings.NewReader(content),
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		request.Header.Set("Authorization", "Bearer "+imageRegistryToken)
		response, err = httpClient.Do(request)
		o.Expect(err).NotTo(o.HaveOccurred())

		bodyDump, err = httputil.DumpResponse(response, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("response from %s %s: %v", request.Method, request.URL, string(bodyDump))

		o.Expect(response.StatusCode).To(
			o.Equal(http.StatusCreated),
			fmt.Sprintf("%s from %s must be %v, got %v", request.Method, request.URL, http.StatusCreated, response.StatusCode),
		)

		request, err = http.NewRequest(
			"GET",
			fmt.Sprintf(
				"https://%s/v2/%s/%s/blobs/%s",
				route.Status.Ingress[0].Host,
				oc.Namespace(),
				"repo",
				digest,
			),
			nil,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		request.Header.Set("Authorization", "Bearer "+imageRegistryToken)
		response, err = httpClient.Do(request)
		o.Expect(err).NotTo(o.HaveOccurred())

		bodyDump, err = httputil.DumpResponse(response, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("response from %s %s: %v", request.Method, request.URL, string(bodyDump))

		o.Expect(response.StatusCode).To(
			o.Equal(http.StatusTemporaryRedirect),
			fmt.Sprintf("%s from %s must be %v for performance reasons; got %v", request.Method, request.URL, http.StatusTemporaryRedirect, response.StatusCode),
		)

		if imageRegistryConfig.Spec.Storage.GCS != nil {
			redirectLocation := response.Header["Location"][0]
			if !strings.Contains(redirectLocation, "storage.googleapis.com") {
				g.Fail(fmt.Sprintf("expected redirect to google storage, but got %v", response.Header["Location"]))
			}
		}
	})
})
