package apiserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/onsi/ginkgo/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = ginkgo.Describe("[sig-api-machinery] API health endpoints", func() {
	f := framework.NewDefaultFramework("api-health-endpoints")
	f.SkipNamespaceCreation = true

	oc := exutil.NewCLIWithoutNamespace("api-health-endpoints").AsAdmin()

	ginkgo.It("should contain the required checks for the openshift-apiserver APIs", ginkgo.Label("Size:S"), func() {
		ctx := context.Background()
		supported, msg := isSupportedPlatform(ctx, "openshift-apiserver", oc)
		if !supported {
			ginkgo.Skip(msg)
		}

		requiredReadyzChecks := sets.New[string](
			"[+]ping ok",
			"[+]log ok",
			"[+]etcd ok",
			"[+]etcd-readiness ok",
			"[+]informer-sync ok",
			"[+]poststarthook/generic-apiserver-start-informers ok",
			"[+]poststarthook/max-in-flight-filter ok",
			"[+]poststarthook/storage-object-count-tracker-hook ok",
			"[+]poststarthook/image.openshift.io-apiserver-caches ok",
			"[+]poststarthook/authorization.openshift.io-bootstrapclusterroles ok",
			"[+]poststarthook/authorization.openshift.io-ensurenodebootstrap-sa ok",
			"[+]poststarthook/project.openshift.io-projectcache ok",
			"[+]poststarthook/project.openshift.io-projectauthorizationcache ok",
			"[+]poststarthook/openshift.io-startinformers ok",
			"[+]poststarthook/openshift.io-restmapperupdater ok",
			"[+]poststarthook/quota.openshift.io-clusterquotamapping ok",
			"[+]shutdown ok",
		)
		requiredLivezChecks := sets.New[string](
			"[+]ping ok",
			"[+]log ok",
			"[+]etcd ok",
			"[+]poststarthook/generic-apiserver-start-informers ok",
			"[+]poststarthook/max-in-flight-filter ok",
			"[+]poststarthook/storage-object-count-tracker-hook ok",
			"[+]poststarthook/image.openshift.io-apiserver-caches ok",
			"[+]poststarthook/authorization.openshift.io-bootstrapclusterroles ok",
			"[+]poststarthook/authorization.openshift.io-ensurenodebootstrap-sa ok",
			"[+]poststarthook/project.openshift.io-projectcache ok",
			"[+]poststarthook/project.openshift.io-projectauthorizationcache ok",
			"[+]poststarthook/openshift.io-startinformers ok",
			"[+]poststarthook/openshift.io-restmapperupdater ok",
			"[+]poststarthook/quota.openshift.io-clusterquotamapping ok",
		)

		// ensure the service exists and hit the well-known endpoint
		_, err := f.ClientSet.CoreV1().Services("openshift-apiserver").Get(ctx, "api", metav1.GetOptions{})
		framework.ExpectNoError(err)

		transport, err := restclient.TransportFor(oc.AdminConfig())
		framework.ExpectNoError(err)
		basePath := oc.AdminConfig().Host + "/api/v1/namespaces/openshift-apiserver/services/https:api:443/proxy/"

		readyzPath := basePath + "readyz?verbose=true"
		ginkgo.By(readyzPath)
		err = testPath(transport, readyzPath, requiredReadyzChecks)
		framework.ExpectNoError(err)

		livezPath := basePath + "livez?verbose=true"
		ginkgo.By(livezPath)
		err = testPath(transport, livezPath, requiredLivezChecks)
		framework.ExpectNoError(err)
		return
	})

	ginkgo.It("should contain the required checks for the oauth-apiserver APIs", ginkgo.Label("Size:S"), func() {
		ctx := context.Background()
		supported, msg := isSupportedPlatform(ctx, "oauth-apiserver", oc)
		if !supported {
			ginkgo.Skip(msg)
		}

		requiredReadyzChecks := sets.New[string](
			"[+]ping ok",
			"[+]log ok",
			"[+]etcd ok",
			"[+]etcd-readiness ok",
			"[+]informer-sync ok",
			"[+]poststarthook/generic-apiserver-start-informers ok",
			"[+]poststarthook/max-in-flight-filter ok",
			"[+]poststarthook/storage-object-count-tracker-hook ok",
			"[+]poststarthook/openshift.io-StartUserInformer ok",
			"[+]poststarthook/openshift.io-StartOAuthInformer ok",
			"[+]poststarthook/openshift.io-StartTokenTimeoutUpdater ok",
			"[+]shutdown ok",
		)
		requiredLivezChecks := sets.New[string](
			"[+]ping ok",
			"[+]log ok",
			"[+]etcd ok",
			"[+]poststarthook/generic-apiserver-start-informers ok",
			"[+]poststarthook/max-in-flight-filter ok",
			"[+]poststarthook/storage-object-count-tracker-hook ok",
			"[+]poststarthook/openshift.io-StartUserInformer ok",
			"[+]poststarthook/openshift.io-StartOAuthInformer ok",
			"[+]poststarthook/openshift.io-StartTokenTimeoutUpdater ok",
		)

		// ensure the service exists and hit the well-known endpoint
		_, err := f.ClientSet.CoreV1().Services("openshift-oauth-apiserver").Get(ctx, "api", metav1.GetOptions{})
		framework.ExpectNoError(err)

		transport, err := restclient.TransportFor(oc.AdminConfig())
		framework.ExpectNoError(err)
		basePath := oc.AdminConfig().Host + "/api/v1/namespaces/openshift-oauth-apiserver/services/https:api:443/proxy/"

		readyzPath := basePath + "readyz?verbose=true"
		ginkgo.By(readyzPath)
		err = testPath(transport, readyzPath, requiredReadyzChecks)
		framework.ExpectNoError(err)

		livezPath := basePath + "livez?verbose=true"
		ginkgo.By(livezPath)
		err = testPath(transport, livezPath, requiredLivezChecks)
		framework.ExpectNoError(err)
		return
	})
})

func testPath(transport http.RoundTripper, path string, requiredChecks sets.Set[string]) error {
	request, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return err
	}
	resp, err := transport.RoundTrip(request)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected response statusCode=%v", resp.StatusCode)
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	checks := sets.New[string](strings.Split(string(rawBody), "\n")...)
	if missing := requiredChecks.Difference(checks); missing.Len() > 0 {
		return fmt.Errorf("missing required checks: %s, for path: %s in: %s", missing, path, string(rawBody))
	}
	return nil
}

func isSupportedPlatform(ctx context.Context, name string, oc *exutil.CLI) (bool, string) {
	isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
	framework.ExpectNoError(err)
	if isMicroShift {
		return false, fmt.Sprintf("MicroShift does not have this component %s", name)
	}
	if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
		return false, fmt.Sprintf("HyperShift does not have this component %s in the same spot", name)
	}
	return true, ""
}
