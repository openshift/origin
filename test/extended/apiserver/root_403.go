package apiserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("apiserver", exutil.KubeConfigPath())

	g.It("anonymous browsers should get a 403 from /", func() {
		transport, err := anonymousHttpTransport(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		// For more info, refer to release notes of https://bugzilla.redhat.com/show_bug.cgi?id=1821771
		for _, history := range cv.Status.History {
			if strings.HasPrefix(history.Version, "4.1.") {
				g.Skip("the test is not expected to work with clusters upgraded from 4.1.x")
			}
		}

		req, err := http.NewRequest("GET", oc.AdminConfig().Host, nil)
		req.Header.Set("Accept", "*/*")
		resp, err := transport.RoundTrip(req)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(resp.StatusCode).Should(o.Equal(http.StatusForbidden))
	})

	g.It("authenticated browser should get a 200 from /", func() {
		transport, err := rest.TransportFor(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		req, err := http.NewRequest("GET", oc.AdminConfig().Host, nil)
		req.Header.Set("Accept", "*/*")
		resp, err := transport.RoundTrip(req)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(resp.StatusCode).Should(o.Equal(http.StatusOK))

		o.Expect(resp.Header.Get("Content-Type")).Should(o.Equal("application/json"))
		type result struct {
			Paths []string
		}
		body, err := ioutil.ReadAll(resp.Body)
		o.Expect(err).NotTo(o.HaveOccurred())

		var got result
		err = json.Unmarshal(body, &got)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func anonymousHttpTransport(restConfig *rest.Config) (*http.Transport, error) {
	if len(restConfig.TLSClientConfig.CAData) == 0 {
		return &http.Transport{}, nil
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(restConfig.TLSClientConfig.CAData); !ok {
		return nil, errors.New("failed to add server CA certificates to client pool")
	}
	return net.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{
			// only use RootCAs from client config, especially no client certs
			RootCAs: pool,
		},
	}), nil
}
