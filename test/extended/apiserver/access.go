package apiserver

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

// These tests are duplicating check-endpoints and monitor test functionality. They are required
// for cert rotation suites where running monitor alongside test is impossible as we need to
// skew time to emulate the age of the cluster which is disruptive.
var _ = g.Describe("[Conformance][sig-api-machinery][Feature:APIServer] kube-apiserver should be accessible via", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("apiserver", admissionapi.LevelPrivileged)

	for description, apiPath := range map[string]string{
		"service network": "kubernetes.default.svc",
		"api-int":         "api-int",
		"api-ext":         "api-ext",
	} {
		g.It(fmt.Sprintf("%s endpoint", description), g.Label("Size:M"), func() {
			// skip on microshift
			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				g.Skip("Not supported on Microshift")
			}

			// external controlplane topology doesn't have master nodes
			controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				g.Skip("ExternalControlPlaneTopology doesn't have master node kubeconfigs")
			}
			// get external/internal URLs
			infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			if apiPath == "api-ext" {
				externalAPIUrl, err := url.Parse(infra.Status.APIServerURL)
				o.Expect(err).NotTo(o.HaveOccurred())
				apiPath = externalAPIUrl.Host
			}
			if apiPath == "api-int" {
				internalAPIUrl, err := url.Parse(infra.Status.APIServerInternalURL)
				o.Expect(err).NotTo(o.HaveOccurred())
				apiPath = internalAPIUrl.Host
			}
			err = retry.OnError(
				wait.Backoff{
					Duration: 2 * time.Second,
					Steps:    3,
					Factor:   5.0,
					Jitter:   0.1,
				},
				func(err error) bool {
					// retry error when kube-apiserver was temporarily unavailable, this matches oc error coming from:
					// https://github.com/kubernetes/kubernetes/blob/cbb5ea8210596ada1efce7e7a271ca4217ae598e/staging/src/k8s.io/kubectl/pkg/cmd/util/helpers.go#L237-L243
					matched, _ := regexp.MatchString("The connection to the server .+ was refused - did you specify the right host or port", err.Error())
					return !matched
				},
				func() error {
					pod, err := exutil.NewPodExecutor(oc, "kube-apiserver-access", image.ShellImage())
					o.Expect(err).NotTo(o.HaveOccurred())
					cmd := fmt.Sprintf("curl -kLs https://%s/readyz", apiPath)
					out, err := pod.Exec(cmd)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(out).To(o.ContainSubstring("ok"))
					return nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	}
})
