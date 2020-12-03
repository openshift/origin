package routes

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/test/e2e/framework"
)

var (
	httpPort  int32 = 8080
	httpsPort int32 = 8443
)

func generateSelfSignedCert(hosts ...string) ([]byte, []byte, error) {
	var hostnames []string
	hostnames = append(hostnames, hosts...)

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"openshift"},
			CommonName:   hosts[0],
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(30 * 24 * time.Hour),
		DNSNames:              hosts,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            10,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	certBuffer := bytes.NewBuffer([]byte{})
	err = pem.Encode(certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return nil, nil, err
	}

	certPEM := certBuffer.Bytes()
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return certPEM, keyPEM, nil
}

type Certificate struct {
	Cert, Key []byte
}

type CertGenerator map[string]*Certificate

func (cr CertGenerator) CertificateFor(hostnames ...string) (*Certificate, error) {
	var hash string
	for _, hostname := range hostnames {
		hash += "/" + hostname
	}

	c, found := cr[hash]
	if found {
		return c, nil
	}

	cert, key, err := generateSelfSignedCert(hostnames...)
	if err != nil {
		return nil, err
	}

	c = &Certificate{
		Cert: cert,
		Key:  key,
	}
	cr[hash] = c

	return c, nil
}

func describeTLSConfig(c *routev1.TLSConfig) string {
	if c == nil {
		return "http-only"
	}

	if len(c.InsecureEdgeTerminationPolicy) == 0 {
		return string(c.Termination)
	}

	return fmt.Sprintf("%s+%s", c.Termination, c.InsecureEdgeTerminationPolicy)
}

func int32ptr(i int32) *int32 {
	return &i
}

func labelsForApp(app string) map[string]string {
	return map[string]string{
		"app": app,
	}
}

func makeRS(name string, app string, responseSuffix string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: int32ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsForApp(app),
			},
			MinReadySeconds: 5,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForApp(app),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "http",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: httpPort,
								},
							},
							Image: "quay.io/openshift/origin-base",
							Command: []string{
								"socat",
								fmt.Sprintf("tcp-listen:%d,reuseaddr,fork", httpPort),
								fmt.Sprintf("SYSTEM:set -eEuxo pipefail; echo 'HTTP/1.1 200 OK' && echo && echo -n 'http: %s'", responseSuffix),
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: httpPort,
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
							},
						},
						{
							Name: "https",
							Ports: []corev1.ContainerPort{
								{
									Name:          "https",
									ContainerPort: httpsPort,
								},
							},
							Image: "quay.io/openshift/origin-base",
							Command: []string{
								"socat",
								fmt.Sprintf("openssl-listen:%d,reuseaddr,fork,cert=/certs/crt.pem,key=/certs/key.pem,cafile=/certs/crt.pem,verify=0", httpsPort),
								fmt.Sprintf("SYSTEM:set -eEuxo pipefail; echo 'HTTP/1.1 200 OK' && echo && echo -n 'https: %s'", responseSuffix),
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: httpsPort,
										},
										Scheme: corev1.URISchemeHTTPS,
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "certs",
									MountPath: "/certs",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: name + "-certs",
								},
							},
						},
					},
				},
			},
		},
	}
}

func makeService(name string, app string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(int(httpPort)),
				},
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstr.FromInt(int(httpsPort)),
				},
			},
			Selector: labelsForApp(app),
		},
	}
}

func makeRoute(routeName string, serviceName string, path string, tls *routev1.TLSConfig) *routev1.Route {
	targetPort := httpPort
	if tls != nil {
		switch tls.Termination {
		case routev1.TLSTerminationEdge:
			targetPort = httpPort
		case routev1.TLSTerminationReencrypt:
			fallthrough
		case routev1.TLSTerminationPassthrough:
			targetPort = httpsPort
		default:
			panic("unexpected termination type")
		}
	}
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name: routeName,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			Path: path,
			TLS:  tls,
			Port: &routev1.RoutePort{
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: targetPort,
				},
			},
		},
	}
}

func getRouteHosts(route *routev1.Route) []string {
	return []string{
		route.Spec.Host,
		fmt.Sprintf("%s.%s.svc", route.Name, route.Namespace),
	}
}

var _ = g.Describe("[sig-network][Conformance][Feature:Route] Path based routing", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI

	g.AfterEach(func() {
		if !g.CurrentGinkgoTestDescription().Failed {
			return
		}

		out, err := oc.Run("get").Args("pods,replicasets,services,endpoints,routes").Output()
		if err != nil {
			framework.Logf("Error gathering resources: %v", err)
			return
		}
		framework.Logf("\n%s\n", out)
	})

	oc = exutil.NewCLI("routes")

	tt := []struct {
		specificRoute *routev1.TLSConfig
		globalRoute   *routev1.TLSConfig
	}{
		{
			specificRoute: nil,
			globalRoute:   nil,
		},
		{
			specificRoute: nil,
			globalRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
			},
		},
		{
			specificRoute: nil,
			globalRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
			},
		},
		{
			specificRoute: nil,
			globalRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
		{
			specificRoute: nil,
			globalRoute: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
		},
		{
			specificRoute: nil,
			globalRoute: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationPassthrough,
			},
		},
		{
			specificRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
			},
			globalRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
			},
		},
		{
			specificRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
			},
			globalRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
			},
		},
		{
			specificRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
			},
			globalRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
		{
			specificRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
			},
			globalRoute: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
		},
		{
			specificRoute: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
			},
			globalRoute: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationPassthrough,
			},
		},
	}
	for _, tc := range tt {
		tc := tc
		testName := fmt.Sprintf(
			"should work for %q route and %q route",
			describeTLSConfig(tc.specificRoute),
			describeTLSConfig(tc.globalRoute),
		)
		g.It(testName, func() {
			ctx := context.Background()

			const (
				global   = "global"
				specific = "specific"
			)

			var err error

			defaultIngressCertCM, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "default-ingress-cert", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(defaultIngressCertCM.Data).NotTo(o.BeNil())
			defaultIngressCert := defaultIngressCertCM.Data["ca-bundle.crt"]
			o.Expect(defaultIngressCert).NotTo(o.BeEmpty())

			certGenerator := make(CertGenerator)

			globalRoute := makeRoute(global, global, "", tc.globalRoute)
			globalRoute, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(ctx, globalRoute, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			specificRoute := makeRoute(specific, specific, "/.well-known/acme-challenge/token", tc.specificRoute)
			specificRoute.Spec.Host = globalRoute.Spec.Host
			specificRoute, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(ctx, specificRoute, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			routes := []*routev1.Route{globalRoute, specificRoute}

			for _, route := range routes {
				cert, err := certGenerator.CertificateFor(getRouteHosts(route)...)
				o.Expect(err).NotTo(o.HaveOccurred())

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: route.Name + "-certs",
					},
					StringData: map[string]string{
						"crt.pem": string(cert.Cert),
						"key.pem": string(cert.Key),
					},
				}
				secret, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(ctx, secret, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Reencrypt routes require DestinationCA
				if route.Spec.TLS == nil || route.Spec.TLS.Termination != routev1.TLSTerminationReencrypt {
					continue
				}

				route, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Patch(ctx, route.Name, types.StrategicMergePatchType, []byte(fmt.Sprintf(`{"spec":{"tls":{"destinationCACertificate":%q}}}`, string(cert.Cert))), metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			var replicaSets []*appsv1.ReplicaSet
			for _, app := range []string{global, specific} {
				rs := makeRS(app, app, app)
				rs, err = oc.KubeClient().AppsV1().ReplicaSets(oc.Namespace()).Create(ctx, rs, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				replicaSets = append(replicaSets, rs)

				service := makeService(app, app)
				service, err = oc.KubeClient().CoreV1().Services(oc.Namespace()).Create(ctx, service, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			for _, route := range routes {
				framework.Logf("Waiting for Route %s/%s to be admitted.", route.Namespace, route.Name)

				route, err = exutil.WaitForRouteState(ctx, oc.RouteClient().RouteV1(), route.Namespace, route.Name, func(route *routev1.Route) (bool, error) {
					for _, i := range route.Status.Ingress {
						for _, c := range i.Conditions {
							if c.Type == routev1.RouteAdmitted && c.Status == corev1.ConditionTrue {
								return true, nil
							}
						}
					}

					return false, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("Route %s/%s is admitted.", route.Namespace, route.Name)
			}

			// Admitted Route doesn't mean it's exposed by the router yet but it's the best we got.
			// Router should reload before the pods became available.

			for _, rs := range replicaSets {
				framework.Logf("Waiting for ReplicaSet %s/%s to be fully available.", rs.Namespace, rs.Name)

				rs, err = exutil.WaitForRSState(ctx, oc.KubeClient().AppsV1(), rs.Namespace, rs.Name, func(rs *appsv1.ReplicaSet) (bool, error) {
					return rs.Status.AvailableReplicas == *rs.Spec.Replicas, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				framework.Logf("ReplicaSet %s/%s is fully available with %d replicas.", rs.Namespace, rs.Name, rs.Status.AvailableReplicas)
			}

			framework.Logf("Starting to test the endpoints.")
			type UrlExpectation struct {
				url            string
				statusCode     int
				expectedResult string
			}
			for _, route := range routes {
				var urlExpectations []UrlExpectation

				if route.Spec.TLS == nil {
					urlExpectations = append(urlExpectations, UrlExpectation{
						url: (&url.URL{
							Scheme: "http",
							Host:   route.Spec.Host,
							Path:   route.Spec.Path,
						}).String(),
						statusCode:     http.StatusOK,
						expectedResult: "http: " + route.Name,
					})
				} else {
					switch route.Spec.TLS.Termination {
					case routev1.TLSTerminationEdge:
						urlExpectations = append(urlExpectations, UrlExpectation{
							url: (&url.URL{
								Scheme: "https",
								Host:   route.Spec.Host,
								Path:   route.Spec.Path,
							}).String(),
							statusCode:     http.StatusOK,
							expectedResult: "http: " + route.Name,
						})

						switch route.Spec.TLS.InsecureEdgeTerminationPolicy {
						case "", routev1.InsecureEdgeTerminationPolicyNone:
							urlExpectations = append(urlExpectations, UrlExpectation{
								url: (&url.URL{
									Scheme: "http",
									Host:   route.Spec.Host,
									Path:   route.Spec.Path,
								}).String(),
								statusCode: http.StatusServiceUnavailable,
							})
						case routev1.InsecureEdgeTerminationPolicyAllow:
							urlExpectations = append(urlExpectations, UrlExpectation{
								url: (&url.URL{
									Scheme: "http",
									Host:   route.Spec.Host,
									Path:   route.Spec.Path,
								}).String(),
								statusCode:     http.StatusOK,
								expectedResult: "http: " + route.Name,
							})
						case routev1.InsecureEdgeTerminationPolicyRedirect:
							urlExpectations = append(urlExpectations, UrlExpectation{
								url: (&url.URL{
									Scheme: "http",
									Host:   route.Spec.Host,
									Path:   route.Spec.Path,
								}).String(),
								statusCode: http.StatusFound,
							})
						default:
							panic("unexpected edge termination policy type")
						}

					case routev1.TLSTerminationReencrypt:
						fallthrough
					case routev1.TLSTerminationPassthrough:
						urlExpectations = append(urlExpectations, UrlExpectation{
							url: (&url.URL{
								Scheme: "https",
								Host:   route.Spec.Host,
								Path:   route.Spec.Path,
							}).String(),
							statusCode:     http.StatusOK,
							expectedResult: "https: " + route.Name,
						})

					default:
						panic("unexpected termination policy type")
					}
				}

				for _, e := range urlExpectations {
					func() {
						framework.Logf("Testing url %s", e.url)

						cert, err := certGenerator.CertificateFor(getRouteHosts(route)...)
						o.Expect(err).NotTo(o.HaveOccurred())
						caCertPool := x509.NewCertPool()
						caCertPool.AppendCertsFromPEM(cert.Cert)
						caCertPool.AppendCertsFromPEM([]byte(defaultIngressCert))
						client := &http.Client{
							Transport: &http.Transport{
								Proxy: http.ProxyFromEnvironment,
								TLSClientConfig: &tls.Config{
									RootCAs:    caCertPool,
									ClientAuth: tls.NoClientCert,
								},
							},
							CheckRedirect: func(req *http.Request, via []*http.Request) error {
								return http.ErrUseLastResponse
							},
						}

						resp, err := client.Get(e.url)
						o.Expect(err).NotTo(o.HaveOccurred())
						defer resp.Body.Close()

						o.Expect(resp.StatusCode).To(o.BeEquivalentTo(e.statusCode))
						if resp.StatusCode != http.StatusOK {
							return
						}

						body, err := ioutil.ReadAll(resp.Body)
						o.Expect(err).NotTo(o.HaveOccurred())

						res := string(body)
						o.Expect(res).To(o.BeEquivalentTo(e.expectedResult))
					}()
				}
			}
		})
	}
})
