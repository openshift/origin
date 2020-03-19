package authentication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"

	userv1 "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/test/extended/scheme"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:Authentication] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api", exutil.KubeConfigPath())

	g.Describe("TestFrontProxy", func() {
		g.It(fmt.Sprintf("should succeed"), func() {
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
