package authentication

import (
	"bytes"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	userv1 "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/test/extended/scheme"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

var _ = g.Describe("[Feature:Authentication] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api", exutil.KubeConfigPath())

	g.Describe("TestFrontProxy", func() {
		g.It(fmt.Sprintf("should succeed"), func() {
			if e2e.TestContext.Provider == ibmcloud.ProviderName {
				e2e.Skipf("IBM ROKS clusters do not have an aggregator-client secret in the cluster. Because the control plane lives outside the cluster, the aggregator-client secret is not needed in the cluster.")
			}

			frontProxySecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-kube-apiserver").Get("aggregator-client", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			frontProxyConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			frontProxyConfig.TLSClientConfig.CertData = frontProxySecret.Data["tls.crt"]
			frontProxyConfig.TLSClientConfig.KeyData = frontProxySecret.Data["tls.key"]
			frontProxyConfig.GroupVersion = &schema.GroupVersion{Version: "v1"}
			frontProxyConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}

			restClient, err := rest.RESTClientFor(frontProxyConfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			content, err := restClient.Get().SetHeader("X-Remote-User", oc.Username()).SetHeader("X-Remote-Group", "system:authenticated").AbsPath("/apis/user.openshift.io/v1/users/~").DoRaw()
			o.Expect(err).NotTo(o.HaveOccurred())

			user := &userv1.User{}
			err = json.NewDecoder(bytes.NewBuffer(content)).Decode(user)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(user.Name).To(o.Equal(oc.Username()))
		})
	})
})
