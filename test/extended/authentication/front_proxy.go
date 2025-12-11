package authentication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/test/extended/scheme"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:Authentication] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api")

	g.Describe("TestFrontProxy", func() {
		g.It(fmt.Sprintf("should succeed"), g.Label("Size:S"), func() {
			controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("External clusters do not have an aggregator-client secret in the cluster. Because the control plane lives outside the cluster, the aggregator-client secret is not needed in the cluster.")
			}

			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				e2eskipper.Skipf("MicroShift team promised to create this test separately because the aggregator client cert is accessed differently there. If this text is still here, please reachout to MicroShift team about where the test lives.")
			}

			frontProxySecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-kube-apiserver").Get(context.Background(), "aggregator-client", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			frontProxyConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			frontProxyConfig.TLSClientConfig.CertData = frontProxySecret.Data["tls.crt"]
			frontProxyConfig.TLSClientConfig.KeyData = frontProxySecret.Data["tls.key"]
			frontProxyConfig.GroupVersion = &schema.GroupVersion{Version: "v1"}
			frontProxyConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}

			restClient, err := rest.RESTClientFor(frontProxyConfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			content, err := restClient.Get().SetHeader("X-Remote-User", oc.Username()).SetHeader("X-Remote-Group", "system:authenticated").AbsPath("/apis/user.openshift.io/v1/users/~").DoRaw(context.Background())
			o.Expect(err).NotTo(o.HaveOccurred())

			user := &userv1.User{}
			err = json.NewDecoder(bytes.NewBuffer(content)).Decode(user)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(user.Name).To(o.Equal(oc.Username()))
		})
	})
})
