package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/golang/protobuf/proto"
	openapi_v3 "github.com/google/gnostic-models/openapiv3"

	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/handler3"
	"k8s.io/kube-openapi/pkg/spec3"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver-openapi")

	g.It("should serve openapi v3 discovery", g.Label("Size:S"), func() {
		transport, err := rest.TransportFor(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		req, err := http.NewRequest("GET", oc.AdminConfig().Host+"/openapi/v3", nil)
		req.Header.Set("Accept", "*/*")
		resp, err := transport.RoundTrip(req)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(resp.StatusCode).Should(o.Equal(http.StatusOK))

		body, err := io.ReadAll(resp.Body)
		o.Expect(err).NotTo(o.HaveOccurred())

		var openapiDiscovery handler3.OpenAPIV3Discovery
		err = json.Unmarshal(body, &openapiDiscovery)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("all relative URLs should have a hash")
		for _, gvPath := range openapiDiscovery.Paths {
			url, err := url.Parse(gvPath.ServerRelativeURL)
			o.Expect(err).NotTo(o.HaveOccurred())
			hash := url.Query().Get("hash")
			o.Expect(hash).ShouldNot(o.HaveLen(0))
		}
	})

	g.It("should serve openapi v3", g.Label("Size:M"), func() {
		transport, err := rest.TransportFor(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		req, err := http.NewRequest("GET", oc.AdminConfig().Host+"/openapi/v3", nil)
		req.Header.Set("Accept", "*/*")
		resp, err := transport.RoundTrip(req)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(resp.StatusCode).Should(o.Equal(http.StatusOK))

		body, err := io.ReadAll(resp.Body)
		o.Expect(err).NotTo(o.HaveOccurred())

		var openapiDiscovery handler3.OpenAPIV3Discovery
		err = json.Unmarshal(body, &openapiDiscovery)
		o.Expect(err).NotTo(o.HaveOccurred())

		authorizationOpenshiftExpectedPaths := []string{
			"/apis/authorization.openshift.io/v1/",
			"/apis/authorization.openshift.io/v1/clusterrolebindings",
			"/apis/authorization.openshift.io/v1/clusterrolebindings/{name}",
			"/apis/authorization.openshift.io/v1/clusterroles",
			"/apis/authorization.openshift.io/v1/clusterroles/{name}",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/localresourceaccessreviews",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/localsubjectaccessreviews",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/rolebindingrestrictions",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/rolebindingrestrictions/{name}",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/rolebindings",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/rolebindings/{name}",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/roles",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/roles/{name}",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/selfsubjectrulesreviews",
			"/apis/authorization.openshift.io/v1/namespaces/{namespace}/subjectrulesreviews",
			"/apis/authorization.openshift.io/v1/resourceaccessreviews",
			"/apis/authorization.openshift.io/v1/rolebindingrestrictions",
			"/apis/authorization.openshift.io/v1/rolebindings",
			"/apis/authorization.openshift.io/v1/roles",
			"/apis/authorization.openshift.io/v1/subjectaccessreviews",
			"/apis/authorization.openshift.io/v1/watch/namespaces/{namespace}/rolebindingrestrictions",
			"/apis/authorization.openshift.io/v1/watch/namespaces/{namespace}/rolebindingrestrictions/{name}",
			"/apis/authorization.openshift.io/v1/watch/rolebindingrestrictions",
		}

		authorizationKubernetestExpectedPaths := []string{
			"/apis/authorization.k8s.io/v1/",
			"/apis/authorization.k8s.io/v1/namespaces/{namespace}/localsubjectaccessreviews",
			"/apis/authorization.k8s.io/v1/selfsubjectaccessreviews",
			"/apis/authorization.k8s.io/v1/selfsubjectrulesreviews",
			"/apis/authorization.k8s.io/v1/subjectaccessreviews",
		}

		testCases := []struct {
			name         string
			accept       string
			useEtag      bool
			groupVersion string
			expectTitle  string
			expectPaths  []string
		}{
			// test authorization.openshift.io
			{
				name:         "authorization.openshift.io with json",
				groupVersion: "apis/authorization.openshift.io/v1",
				accept:       "application/json",
				expectTitle:  "OpenShift",
				expectPaths:  authorizationOpenshiftExpectedPaths,
			},
			{
				name:         "authorization.openshift.io with json+hash",
				groupVersion: "apis/authorization.openshift.io/v1",
				accept:       "application/json",
				useEtag:      true,
				expectTitle:  "OpenShift",
				expectPaths:  authorizationOpenshiftExpectedPaths,
			},
			{
				name:         "authorization.openshift.io with protobuf",
				groupVersion: "apis/authorization.openshift.io/v1",
				accept:       "application/com.github.proto-openapi.spec.v3.v1.0+protobuf",
				expectTitle:  "OpenShift",
				expectPaths:  authorizationOpenshiftExpectedPaths,
			},
			{
				name:         "authorization.openshift.io with protobuf+hash",
				groupVersion: "apis/authorization.openshift.io/v1",
				accept:       "application/com.github.proto-openapi.spec.v3.v1.0+protobuf",
				useEtag:      true,
				expectTitle:  "OpenShift",
				expectPaths:  authorizationOpenshiftExpectedPaths,
			},
			// vanilla types (authorization.k8s.io) should work as well
			{
				name:         "authorization.k8s.io/v1 with json",
				groupVersion: "apis/authorization.k8s.io/v1",
				accept:       "application/json",
				expectTitle:  "Kubernetes",
				expectPaths:  authorizationKubernetestExpectedPaths,
			},
			{
				name:         "authorization.k8s.io with json+hash",
				groupVersion: "apis/authorization.k8s.io/v1",
				accept:       "application/json",
				useEtag:      true,
				expectTitle:  "Kubernetes",
				expectPaths:  authorizationKubernetestExpectedPaths,
			},
			{
				name:         "authorization.k8s.io with protobuf",
				groupVersion: "apis/authorization.k8s.io/v1",
				accept:       "application/com.github.proto-openapi.spec.v3.v1.0+protobuf",
				expectTitle:  "Kubernetes",
				expectPaths:  authorizationKubernetestExpectedPaths,
			},
			{
				name:         "authorization.k8s.io with protobuf+hash",
				groupVersion: "apis/authorization.k8s.io/v1",
				accept:       "application/com.github.proto-openapi.spec.v3.v1.0+protobuf",
				useEtag:      true,
				expectTitle:  "Kubernetes",
				expectPaths:  authorizationKubernetestExpectedPaths,
			},
		}

		for _, tc := range testCases {
			g.By(fmt.Sprintf("should serve openapi spec for %v", tc.name), func() {
				gvPath, ok := openapiDiscovery.Paths[tc.groupVersion]
				o.Expect(ok).To(o.BeTrue())
				var gvResp *http.Response
				var hash string

				for retries := 0; (gvResp == nil || gvResp.StatusCode == http.StatusMovedPermanently) && retries < 5; retries++ {
					relativeURL := gvPath.ServerRelativeURL
					if gvResp != nil {
						relativeURL = gvResp.Header.Get("Location")
						fmt.Printf("redirected to %v \n", gvResp.Header.Get("Location"))
					}
					url, err := url.Parse(oc.AdminConfig().Host + relativeURL)
					o.Expect(err).NotTo(o.HaveOccurred())
					query := url.Query()
					if tc.useEtag {
						hash = query.Get("hash")
					} else {
						query.Del("hash")
						url.RawQuery = query.Encode()
					}
					gvReq, err := http.NewRequest("GET", url.String(), nil)
					gvReq.Header.Set("Accept", tc.accept)
					gvResp, err = transport.RoundTrip(gvReq)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				o.Expect(gvResp.StatusCode).Should(o.Equal(http.StatusOK))
				gvBody, err := io.ReadAll(gvResp.Body)
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(gvResp.Header.Get("Content-Type")).Should(o.Equal(tc.accept))
				if tc.useEtag {
					o.Expect(gvResp.Header.Get("Etag")).Should(o.Equal(strconv.Quote(hash)))
				} else {
					o.Expect(gvResp.Header.Get("Etag")).ShouldNot(o.BeEmpty())
				}
				o.Expect(gvResp.Header.Get("Vary")).Should(o.Equal("Accept"))
				lastModified, err := time.Parse(time.RFC1123, gvResp.Header.Get("Last-Modified"))
				o.Expect(err).NotTo(o.HaveOccurred())
				if lastModified.IsZero() || lastModified.After(time.Now()) {
					g.Fail(fmt.Sprintf("invalid lastModified: %v", lastModified))
				}

				if tc.useEtag {
					o.Expect(gvResp.Header.Get("Cache-Control")).Should(o.Equal("public, immutable"))
					o.Expect(gvResp.Header.Get("Expires")).ShouldNot(o.BeEmpty())
				} else {
					o.Expect(gvResp.Header.Get("Cache-Control")).Should(o.Equal("no-cache, private"))
				}

				var paths []string
				var title string

				if tc.accept == "application/json" {
					var spec spec3.OpenAPI
					err := json.Unmarshal(gvBody, &spec)
					o.Expect(err).NotTo(o.HaveOccurred())
					title = spec.Info.Title
					for path := range spec.Paths.Paths {
						paths = append(paths, path)
					}
				} else {
					var spec openapi_v3.Document
					err := proto.Unmarshal(gvBody, &spec)
					o.Expect(err).NotTo(o.HaveOccurred())
					title = spec.Info.Title
					for _, item := range spec.Paths.Path {
						paths = append(paths, item.Name)
					}
				}

				o.Expect(title).Should(o.HavePrefix(tc.expectTitle))
				o.Expect(paths).Should(o.ContainElements(tc.expectPaths))
			})
		}
	})
})
