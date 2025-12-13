package apiserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver")

	g.It("anonymous browsers should get a 403 from /", g.Label("Size:S"), func() {
		transport, err := anonymousHttpTransport(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		req, err := http.NewRequest("GET", oc.AdminConfig().Host, nil)
		req.Header.Set("Accept", "*/*")
		resp, err := transport.RoundTrip(req)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(resp.StatusCode).Should(o.Equal(http.StatusForbidden))
	})

	g.It("authenticated browser should get a 200 from /", g.Label("Size:S"), func() {
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
