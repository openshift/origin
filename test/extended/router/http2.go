package router

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"golang.org/x/net/http2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/openshift/origin/test/extended/router/certgen"
	"github.com/openshift/origin/test/extended/router/shard"
)

const (
	// http2ClientGetTimeout specifies the time limit for requests
	// made by the HTTP Client.
	http2ClientTimeout = 1 * time.Minute
)

func makeHTTPClient(useHTTP2Transport bool, timeout time.Duration) *http.Client {
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}

	c := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}

	if useHTTP2Transport {
		c.Transport = &http2.Transport{
			TLSClientConfig: &tlsConfig,
		}
	}

	return c
}

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		http2ServiceConfigPath     = exutil.FixturePath("testdata", "router", "router-http2.yaml")
		http2RoutesConfigPath      = exutil.FixturePath("testdata", "router", "router-http2-routes.yaml")
		http2RouterShardConfigPath = exutil.FixturePath("testdata", "router", "router-shard.yaml")
		http2SourceBackendPath     = exutil.FixturePath("testdata", "router", "router-http2-server.backend")

		oc = exutil.NewCLI("router-http2")

		shardConfigPath string // computed
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			exutil.DumpPodLogsStartingWith("http2", oc)
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
		if len(shardConfigPath) > 0 {
			if err := oc.AsAdmin().Run("delete").Args("-n", "openshift-ingress-operator", "-f", shardConfigPath).Execute(); err != nil {
				e2e.Logf("deleting ingress controller failed: %v\n", err)
			}
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the http2 tests", func() {
			defaultDomain, err := getDefaultIngressClusterDomainName(oc, 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")

			srcTarGz, err := makeCompressedTarArchive([]string{http2SourceBackendPath})
			o.Expect(err).NotTo(o.HaveOccurred())
			base64SrcTarGz := strings.Join(split(base64.StdEncoding.EncodeToString(srcTarGz), 76), "\n")

			g.By(fmt.Sprintf("creating service from a config file %q", http2ServiceConfigPath))
			err = oc.Run("new-app").Args("-f", http2ServiceConfigPath, "-p", "BASE64_SRC_TGZ="+base64SrcTarGz).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.ExpectNoError(e2epod.WaitForPodRunningInNamespaceSlow(oc.KubeClient(), "http2", oc.Namespace()), "http2 backend server pod not running")

			// certificate start and end time are very
			// lenient to avoid any clock drift between
			// between the test machine and the cluster
			// under test.
			notBefore := time.Now().Add(-24 * time.Hour)
			notAfter := time.Now().Add(24 * time.Hour)

			// Generate crt/key for routes that need them.
			_, tlsCrtData, tlsPrivateKey, err := certgen.GenerateKeyPair(notBefore, notAfter)
			o.Expect(err).NotTo(o.HaveOccurred())

			derKey, err := certgen.MarshalPrivateKeyToDERFormat(tlsPrivateKey)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemCrt, err := certgen.MarshalCertToPEMString(tlsCrtData)
			o.Expect(err).NotTo(o.HaveOccurred())

			shardedDomain := oc.Namespace() + "." + defaultDomain

			g.By(fmt.Sprintf("creating routes from a config file %q", http2RoutesConfigPath))
			err = oc.Run("new-app").Args("-f", http2RoutesConfigPath,
				"-p", "DOMAIN="+shardedDomain,
				"-p", "TLS_CRT="+pemCrt,
				"-p", "TLS_KEY="+derKey,
				"-p", "TYPE="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("creating router shard %q from a config file %q", oc.Namespace(), http2RouterShardConfigPath))
			shardConfigPath, err = shard.DeployNewRouterShard(oc, 15*time.Minute, shard.Config{
				FixturePath: http2RouterShardConfigPath,
				Name:        oc.Namespace(),
				Domain:      shardedDomain,
				Type:        oc.Namespace(),
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "new ingresscontroller did not rollout")

			testCases := []struct {
				route             string
				frontendProto     string
				backendProto      string
				statusCode        int
				useHTTP2Transport bool
				expectedGetError  string
			}{{
				route:             "http2-custom-cert-edge",
				frontendProto:     "HTTP/2.0",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: true,
			}, {
				route:             "http2-custom-cert-reencrypt",
				frontendProto:     "HTTP/2.0",
				backendProto:      "HTTP/2.0",
				statusCode:        http.StatusOK,
				useHTTP2Transport: true,
			}, {
				route:             "http2-passthrough",
				frontendProto:     "HTTP/2.0",
				backendProto:      "HTTP/2.0",
				statusCode:        http.StatusOK,
				useHTTP2Transport: true,
			}, {
				route:             "http2-default-cert-edge",
				useHTTP2Transport: true,
				expectedGetError:  `http2: unexpected ALPN protocol ""; want "h2"`,
			}, {
				route:             "http2-default-cert-reencrypt",
				useHTTP2Transport: true,
				expectedGetError:  `http2: unexpected ALPN protocol ""; want "h2"`,
			}, {
				route:             "http2-custom-cert-edge",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-custom-cert-reencrypt",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/2.0", // reencrypt always has backend ALPN enabled
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-passthrough",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-default-cert-edge",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-default-cert-reencrypt",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/2.0", // reencrypt always has backend ALPN enabled
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}}

			// If we cannot resolve then we're not going
			// to make a connection, so assert that lookup
			// succeeds for each route. DNS can take a
			// long time to rollout for the new subdomain.
			for _, tc := range testCases {
				err := wait.PollImmediate(3*time.Second, 20*time.Minute, func() (bool, error) {
					host := tc.route + "." + shardedDomain
					addrs, err := net.LookupHost(host)
					if err != nil {
						e2e.Logf("host lookup error: %v, retrying...", err)
						return false, nil
					}
					e2e.Logf("host %q now resolves as %+v", host, addrs)
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// Shard is using a namespace selector so
			// label the test namespace to match.
			err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			for i, tc := range testCases {
				host := tc.route + "." + shardedDomain
				e2e.Logf("[test #%d/%d]: GET route: %s", i+1, len(testCases), host)

				var resp *http.Response

				o.Expect(wait.Poll(3*time.Second, 15*time.Minute, func() (bool, error) {
					host := tc.route + "." + shardedDomain
					client := makeHTTPClient(tc.useHTTP2Transport, http2ClientTimeout)
					resp, err = client.Get("https://" + host)
					if err != nil && len(tc.expectedGetError) != 0 {
						errMatch := strings.Contains(err.Error(), tc.expectedGetError)
						if !errMatch {
							// We can hit the "default" ingress controller
							// so wait for the new one to be fully
							// registered and serving.
							e2e.Logf("[test #%d/%d]: route: %s, GET error: %v, retrying...", i+1, len(testCases), host, err)
						}
						return errMatch, nil
					}
					if err != nil {
						e2e.Logf("[test #%d/%d]: route: %s, GET error: %v, retrying...", i+1, len(testCases), host, err)
						return false, nil // could be 503 if service not ready
					}
					if tc.statusCode == 0 {
						return false, nil
					}
					if resp.StatusCode != tc.statusCode {
						e2e.Logf("[test #%d/%d]: route: %s, expected status: %v, actual status: %v, retrying...", i+1, len(testCases), host, tc.statusCode, resp.StatusCode)
					}
					return resp.StatusCode == tc.statusCode, nil
				})).NotTo(o.HaveOccurred())

				if tc.expectedGetError != "" {
					continue
				}

				o.Expect(resp).ToNot(o.BeNil(), "response was nil")
				o.Expect(resp.StatusCode).To(o.Equal(tc.statusCode), "HTTP response code not matched")
				o.Expect(resp.Proto).To(o.Equal(tc.frontendProto), "protocol not matched")
				body, err := ioutil.ReadAll(resp.Body)
				o.Expect(err).NotTo(o.HaveOccurred(), "failed to read the response body")
				o.Expect(string(body)).To(o.Equal(tc.backendProto), fmt.Sprintf("response body content not matched: got %q, want %q", string(body), tc.backendProto))
				o.Expect(resp.Body.Close()).NotTo(o.HaveOccurred(), "failed to close response body")
			}
		})
	})
})

func getDefaultIngressClusterDomainName(oc *exutil.CLI, timeout time.Duration) (string, error) {
	var domain string

	if err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		ingress, err := oc.AdminConfigClient().ConfigV1().Ingresses().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Get ingresses.config/cluster failed: %v, retrying...", err)
			return false, nil
		}
		domain = ingress.Spec.Domain
		return true, nil
	}); err != nil {
		return "", err
	}

	return domain, nil
}

// split string into chunks limited in length by size.
// Note: assumes 1:1 mapping between bytes/chars (i.e., non-UTF).
func split(s string, size int) []string {
	var chunks []string

	for len(s) > 0 {
		if len(s) < size {
			size = len(s)
		}
		chunks, s = append(chunks, s[:size]), s[size:]
	}

	return chunks
}

// makeCompressedTarArchive creates a compressed tar archive from the
// contents of all filenames.
func makeCompressedTarArchive(filenames []string) ([]byte, error) {
	buf := new(bytes.Buffer)

	gz, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("Error: gzip.NewWriterLevel(): %v", err)
	}

	tw := tar.NewWriter(gz)

	for _, filename := range filenames {
		file, err := os.Stat(filename)
		if err != nil {
			return nil, fmt.Errorf("Error: failed to stat %q: %v", filename, err)
		}

		hdr, err := tar.FileInfoHeader(file, "")
		if err != nil {
			return nil, fmt.Errorf("Error: failed to create tar header for %q: %v", filename, err)
		}

		// Avoid untar logging "implausibly old timestamp".
		hdr.ModTime = time.Now()

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}

		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read %q: %v", filename, err)
		}

		if len(data) == 0 {
			return nil, fmt.Errorf("%q has no data", filename)
		}

		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
