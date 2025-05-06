package router

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	admissionapi "k8s.io/pod-security-admission/api"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/test/extended/router/certgen"
	exutil "github.com/openshift/origin/test/extended/util"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
)

const (
	// secretReaderRole is the name of the Role allowing access to the secret.
	secretReaderRole = "secret-reader-role"
	// secretReaderRoleBinding is the name of the RoleBinding associating the Role with the router service account.
	secretReaderRoleBinding = "secret-reader-role-binding"
	// helloOpenShiftResponse is the HTTP response from hello-openshift example pod.
	// https://github.com/kubernetes/kubernetes/blob/88dfcb225d41326113990e87b11137641c121a32/test/images/agnhost/netexec/netexec.go#L266-L269
	helloOpenShiftResponse = "NOW:"
	rootCertIssuerCN       = "RouteExternalCertificate Root CA"
)

var _ = g.Describe("[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc            = exutil.NewCLIWithPodSecurityLevel("router-external-certificate", admissionapi.LevelBaseline)
		helloPodPath  = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "hello-openshift", "hello-pod.json")
		helloPodName  = "hello-openshift"
		helloPodSvc   = "hello-openshift"
		defaultDomain string
		routerURL     string
		err           error
	)

	g.BeforeEach(func() {
		ip, err := exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		routerURL = fmt.Sprintf("https://%s", exutil.IPUrl(ip))

		defaultDomain, err = getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")

		g.By("creating pod")
		err = oc.Run("create").Args("-f", helloPodPath, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the pod to be running")
		err = pod.WaitForPodNameRunningInNamespace(context.TODO(), oc.KubeClient(), helloPodName, oc.Namespace())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating service")
		err = oc.Run("expose").Args("pod", helloPodName, "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the service to become available")
		err = exutil.WaitForEndpoint(oc.KubeClient(), oc.Namespace(), helloPodSvc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Context("with invalid setup", func() {
		var host string
		g.BeforeEach(func() {
			host = oc.Namespace() + "." + defaultDomain
		})

		g.Describe("the router", func() {
			g.It("should not support external certificate without proper permissions", func() {
				g.By("Creating a TLS certificate secret")
				secret, _, err := generateTLSCertSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeTLS, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Creating a route")
				route := generateRouteWithExternalCertificate(oc.Namespace(), "route", secret.Name, helloPodSvc, host, routev1.TLSTerminationEdge)
				_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), route, metav1.CreateOptions{})
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(err.Error()).To(o.And(
					o.ContainSubstring("Forbidden: router serviceaccount does not have permission to get this secret"),
					o.ContainSubstring("Forbidden: router serviceaccount does not have permission to watch this secret"),
					o.ContainSubstring("Forbidden: router serviceaccount does not have permission to list this secret")),
				)
			})

			g.It("should not support external certificate if the secret is in a different namespace", func() {
				g.By("Creating a new namespace")
				differentNamespace := fmt.Sprintf("%s-%s", "router-external-certificate", rand.String(5))
				err := createNamespace(oc, differentNamespace)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Creating a TLS certificate secret in another namespace")
				secret, _, err := generateTLSCertSecret(differentNamespace, "my-tls-secret", corev1.SecretTypeTLS, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.AdminKubeClient().CoreV1().Secrets(differentNamespace).Create(context.Background(), secret, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Creating a route in other namespace")
				route := generateRouteWithExternalCertificate(oc.Namespace(), "route", secret.Name, helloPodSvc, host, routev1.TLSTerminationEdge)
				_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), route, metav1.CreateOptions{})
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(err.Error()).To(o.ContainSubstring(`Not found: "secrets \"my-tls-secret\" not found"`))
			})

			g.It("should not support external certificate if the secret is not of type kubernetes.io/tls", func() {
				g.By("Creating a secret with the WRONG type (Opaque)")
				secret, _, err := generateTLSCertSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeOpaque, host) // Incorrect type
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Creating a route")
				route := generateRouteWithExternalCertificate(oc.Namespace(), "route", secret.Name, helloPodSvc, host, routev1.TLSTerminationEdge)
				_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), route, metav1.CreateOptions{})
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(err.Error()).To(o.ContainSubstring(`Invalid value: "my-tls-secret": secret of type "kubernetes.io/tls" required`))
			})

			g.It("should not support external certificate if the route termination type is Passthrough", func() {
				g.By("Creating a TLS certificate secret")
				secret, _, err := generateTLSCertSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeTLS, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Creating a route with Passthrough termination")
				passthroughRoute := generateRouteWithExternalCertificate(oc.Namespace(), "passthrough-route", secret.Name, helloPodSvc, host, routev1.TLSTerminationPassthrough)
				_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), passthroughRoute, metav1.CreateOptions{})
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(err.Error()).To(o.ContainSubstring(`Invalid value: "my-tls-secret": passthrough termination does not support certificates`))
			})

			g.It("should not support external certificate if inline certificate is also present", func() {
				g.By("Creating a TLS certificate secret")
				secret, _, err := generateTLSCertSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeTLS, host)
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Creating a route")
				route := generateRouteWithExternalCertificate(oc.Namespace(), "route", secret.Name, helloPodSvc, host, routev1.TLSTerminationEdge)
				// Add inline certificate
				route.Spec.TLS.Certificate = "my-crt"
				_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), route, metav1.CreateOptions{})
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(err.Error()).To(o.ContainSubstring(`Invalid value: "my-tls-secret": cannot specify both tls.certificate and tls.externalCertificate`))
			})
		})
	})

	g.Context("with valid setup", func() {
		var (
			secret       *corev1.Secret
			routes       []*routev1.Route
			hosts        []string
			rootDerBytes []byte
		)

		g.BeforeEach(func() {
			// The number of routes here is deliberately set to be greater than 5
			// to test the OpenShift Router's contention tracker behaviour. (see: https://github.com/openshift/router/blob/b41f9d05467fb7b3f6c2dafa6ac4b5e25164c0b6/pkg/router/controller/contention.go#L86).
			// This tracker limits the frequency of route status updates.
			// https://github.com/openshift/router/pull/614 introduced to ignore contention (route status updates) done by this feature (ExternalCertificate).
			// To ensure proper handling of the contention tracker, we need to test scenarios where a single secret is shared by more than 5 routes.
			// These routes' statuses should be able to update frequently without being blocked by the contention tracker.
			const numRoutes = 6
			var routeNames []string

			for i := 0; i < numRoutes; i++ {
				hosts = append(hosts, fmt.Sprintf("host-%d-%s.%s", i, oc.Namespace(), defaultDomain))
				routeNames = append(routeNames, fmt.Sprintf("route-%d", i))
			}

			g.By("Creating a TLS certificate secret")
			secret, rootDerBytes, err = generateTLSCertSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeTLS, hosts...)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Providing router service account permissions to get,list,watch the secret")
			_, err = oc.KubeClient().RbacV1().Roles(oc.Namespace()).Create(context.Background(),
				generateSecretReaderRole(oc.Namespace(), "my-tls-secret"), metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.KubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(),
				generateRouterRoleBinding(oc.Namespace()), metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating multiple routes referencing same external certificate")
			for i := 0; i < numRoutes; i++ {
				route := generateRouteWithExternalCertificate(oc.Namespace(), routeNames[i], secret.Name, helloPodSvc, hosts[i], routev1.TLSTerminationEdge)
				_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), route, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				routes = append(routes, route)
			}
		})

		g.Describe("the router should support external certificate", func() {
			g.It("and routes are reachable", func() {
				g.By("Sending https request")
				for _, route := range routes {
					hostName, err := getHostnameForRoute(oc, route.Name)
					o.Expect(err).NotTo(o.HaveOccurred())
					resp, err := httpsGetCall(oc, hostName, rootDerBytes)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
				}
			})

			g.Context("and the secret is deleted", func() {
				g.BeforeEach(func() {
					g.By("Deleting the secret")
					err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				})

				g.It("then routes are not reachable", func() {
					g.By("Checking the route status")
					for _, route := range routes {
						checkRouteStatus(oc, route.Name, corev1.ConditionFalse, "ExternalCertificateValidationFailed")
					}
				})

				g.Context("and re-created again", func() {
					g.It("then routes are reachable", func() {
						g.By("Re-creating the deleted secret")
						_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Sending https request")
						for _, route := range routes {
							hostName, err := getHostnameForRoute(oc, route.Name)
							o.Expect(err).NotTo(o.HaveOccurred())
							resp, err := httpsGetCall(oc, hostName, rootDerBytes)
							o.Expect(err).NotTo(o.HaveOccurred())
							o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
						}
					})
				})

				g.Context("and re-created again but RBAC permissions are dropped", func() {
					g.It("then routes are not reachable", func() {
						g.By("Deleting RBAC permissions")
						err = oc.KubeClient().RbacV1().RoleBindings(oc.Namespace()).Delete(context.Background(), secretReaderRoleBinding, metav1.DeleteOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Re-creating the deleted secret")
						_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Checking the route status")
						for _, route := range routes {
							checkRouteStatus(oc, route.Name, corev1.ConditionFalse, "ExternalCertificateValidationFailed")
						}
					})
				})
			})

			g.Context("and the secret is updated", func() {
				g.It("then also routes are reachable", func() {
					g.By("Updating the existing secret")
					// build a new secret
					secret, rootDerBytes, err = generateTLSCertSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeTLS, hosts...)
					o.Expect(err).NotTo(o.HaveOccurred())
					// update the existing secret with the new secret
					_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Update(context.Background(), secret, metav1.UpdateOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Sending https request")
					for _, route := range routes {
						hostName, err := getHostnameForRoute(oc, route.Name)
						o.Expect(err).NotTo(o.HaveOccurred())
						resp, err := httpsGetCall(oc, hostName, rootDerBytes)
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
					}
				})
			})

			g.Context("and the secret is updated but RBAC permissions are dropped", func() {
				g.It("then routes are not reachable", func() {
					g.By("Deleting RBAC permissions")
					err = oc.KubeClient().RbacV1().RoleBindings(oc.Namespace()).Delete(context.Background(), secretReaderRoleBinding, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Updating the existing secret")
					// build a new secret
					secret, rootDerBytes, err = generateTLSCertSecret(oc.Namespace(), "my-tls-secret", corev1.SecretTypeTLS, hosts...)
					o.Expect(err).NotTo(o.HaveOccurred())
					// update the existing secret with the new secret
					_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Update(context.Background(), secret, metav1.UpdateOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Checking the route status")
					for _, route := range routes {
						checkRouteStatus(oc, route.Name, corev1.ConditionFalse, "ExternalCertificateValidationFailed")
					}
				})
			})

			g.Context("and the route is updated", func() {
				var (
					routeToTest   *routev1.Route
					newSecretName = "new-ext-crt"
				)

				g.BeforeEach(func() {
					// These tests do not *explicitly* need verification on multiple routes.
					// Hence taking only one route into account.
					routeToTest = routes[0]
				})

				g.Context("to use new external certificate", func() {
					g.It("then also the route is reachable", func() {
						g.By("Creating a new secret")
						secret, rootDerBytes, err = generateTLSCertSecret(oc.Namespace(), newSecretName, corev1.SecretTypeTLS, hosts...)
						o.Expect(err).NotTo(o.HaveOccurred())
						_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Updating the existing role to add RBAC permissions for the new secret")
						err := patchRoleWithSecretAccess(oc, newSecretName)
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Updating the route to use new external certificate")
						err = patchRouteWithExternalCertificate(oc, routeToTest.Name, newSecretName)
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Sending https request")
						hostName, err := getHostnameForRoute(oc, routeToTest.Name)
						o.Expect(err).NotTo(o.HaveOccurred())
						resp, err := httpsGetCall(oc, hostName, rootDerBytes)
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
					})
				})

				g.Context("to use new external certificate, but RBAC permissions are not added", func() {
					g.It("route update is rejected", func() {
						g.By("Creating a new secret")
						secret, _, err = generateTLSCertSecret(oc.Namespace(), newSecretName, corev1.SecretTypeTLS, hosts...)
						o.Expect(err).NotTo(o.HaveOccurred())
						_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Updating the route to use new external certificate")
						err := patchRouteWithExternalCertificate(oc, routeToTest.Name, newSecretName)
						o.Expect(err).To(o.HaveOccurred())
						o.Expect(err.Error()).To(o.And(
							o.ContainSubstring("Forbidden: router serviceaccount does not have permission to get this secret"),
							o.ContainSubstring("Forbidden: router serviceaccount does not have permission to watch this secret"),
							o.ContainSubstring("Forbidden: router serviceaccount does not have permission to list this secret")),
						)
					})
				})

				g.Context("to use new external certificate, but secret is not of type kubernetes.io/tls", func() {
					g.It("route update is rejected", func() {
						g.By("Creating a secret with the WRONG type (Opaque)")
						secret, _, err := generateTLSCertSecret(oc.Namespace(), newSecretName, corev1.SecretTypeOpaque, hosts...) // Incorrect type
						o.Expect(err).NotTo(o.HaveOccurred())
						_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Updating the route to use new external certificate")
						err = patchRouteWithExternalCertificate(oc, routeToTest.Name, newSecretName)
						o.Expect(err).To(o.HaveOccurred())
						o.Expect(err.Error()).To(o.ContainSubstring(fmt.Sprintf(`Invalid value: "%s": secret of type "kubernetes.io/tls" required`, newSecretName)))
					})

				})

				g.Context("to use new external certificate, but secret does not exist", func() {
					// do not create new secret
					g.It("route update is rejected", func() {
						g.By("Updating the route to use new external certificate")
						err := patchRouteWithExternalCertificate(oc, routeToTest.Name, newSecretName)
						o.Expect(err).To(o.HaveOccurred())
						o.Expect(err.Error()).To(o.ContainSubstring(fmt.Sprintf(`Not found: "secrets \"%s\" not found"`, newSecretName)))
					})
				})

				g.Context("to use same external certificate", func() {
					g.It("then also the route is reachable", func() {
						g.By("Adding some label to trigger route update")
						err := patchRouteWithLabel(oc, routeToTest.Name)
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Sending https request")
						hostName, err := getHostnameForRoute(oc, routeToTest.Name)
						o.Expect(err).NotTo(o.HaveOccurred())
						resp, err := httpsGetCall(oc, hostName, rootDerBytes)
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
					})

				})

				g.Context("to use same external certificate, but RBAC permissions are dropped", func() {
					g.It("route update is rejected", func() {
						g.By("Deleting RBAC permissions")
						err = oc.KubeClient().RbacV1().RoleBindings(oc.Namespace()).Delete(context.Background(), secretReaderRoleBinding, metav1.DeleteOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("Adding some label to trigger route update")
						err := patchRouteWithLabel(oc, routeToTest.Name)
						o.Expect(err).To(o.HaveOccurred())
						o.Expect(err.Error()).To(o.And(
							o.ContainSubstring("Forbidden: router serviceaccount does not have permission to get this secret"),
							o.ContainSubstring("Forbidden: router serviceaccount does not have permission to watch this secret"),
							o.ContainSubstring("Forbidden: router serviceaccount does not have permission to list this secret")),
						)
					})
				})

				g.Context("to remove the external certificate", func() {
					g.BeforeEach(func() {
						g.By("Updating the route to remove the external certificate reference")
						err := patchRouteToRemoveExternalCertificate(oc, routeToTest.Name)
						o.Expect(err).NotTo(o.HaveOccurred())
					})

					g.It("then also the route is reachable and serves the default certificate", func() {
						g.By("Sending in-secure https request")
						hostName, err := getHostnameForRoute(oc, routeToTest.Name)
						o.Expect(err).NotTo(o.HaveOccurred())

						// Check if the job is running on a metal platform
						metal, err := isMetalJob(oc)
						o.Expect(err).NotTo(o.HaveOccurred())

						if metal {
							execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), oc.Namespace(), "execpod")
							defer func() {
								oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
							}()
							// don't assume the router is available via external DNS, because of complexity
							err = waitForRouterOKResponseExec(oc.Namespace(), execPod.Name, routerURL, hostName, changeTimeoutSeconds)
							o.Expect(err).NotTo(o.HaveOccurred())
						} else {
							resp, err := verifyRouteServesDefaultCert(oc, hostName)
							o.Expect(err).NotTo(o.HaveOccurred())
							o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
						}
					})

					g.Context("and again re-add the same external certificate", func() {
						g.It("then also the route is reachable", func() {
							g.By("Updating the route to re-add the external certificate reference")
							err = patchRouteWithExternalCertificate(oc, routeToTest.Name, secret.Name)
							o.Expect(err).NotTo(o.HaveOccurred())

							g.By("Sending https request")
							hostName, err := getHostnameForRoute(oc, routeToTest.Name)
							o.Expect(err).NotTo(o.HaveOccurred())
							resp, err := httpsGetCall(oc, hostName, rootDerBytes)
							o.Expect(err).NotTo(o.HaveOccurred())
							o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
						})
					})
				})
			})
		})
	})
})

// httpsGetCall makes an HTTPS GET request to the specified hostname with retries.
// It uses the provided rootDerBytes as the trusted CA certificate.
// For metal platforms, it uses an exec pod to make the request.
func httpsGetCall(oc *exutil.CLI, hostname string, rootDerBytes []byte) (string, error) {
	url := fmt.Sprintf("https://%s", hostname)
	e2e.Logf("running https get for host %q", hostname)

	if len(rootDerBytes) == 0 {
		return "", fmt.Errorf("root CA is empty; certificate generation likely failed")
	}
	// convert DER to PEM
	rootCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rootDerBytes,
	})

	// Check if the job is running on a metal platform
	metal, err := isMetalJob(oc)
	if err != nil {
		return "", err
	}

	if metal {
		e2e.Logf("Running on a metal platform")
		return httpsGetCallWithExecPod(oc, url, rootCertPEM)
	}

	// add root CA to trust pool
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(rootCertPEM); !ok {
		return "", fmt.Errorf("failed to add root CA certificate to cert pool")
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}

	_, body, err := sendHttpRequestWithRetry(url, client)
	return body, err
}

// httpsGetCallWithExecPod makes HTTPS GET request using an exec pod.
// This function is used specifically for metal platforms where external DNS resolution
// might be problematic. It creates a ConfigMap using the given root CA certificate,
// mounts it to the exec pod, and then uses curl within the pod to make the HTTPS request with retries.
func httpsGetCallWithExecPod(oc *exutil.CLI, url string, rootCertPEM []byte) (string, error) {
	// Create a configMap to hold the root CA certificate.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", oc.Namespace(), "root-ca", rand.String(5)),
			Namespace: oc.Namespace(),
		},
		Data: map[string]string{
			"ca.crt": string(rootCertPEM),
		},
	}
	_, err := oc.KubeClient().CoreV1().ConfigMaps(oc.Namespace()).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	defer func() {
		oc.KubeClient().CoreV1().ConfigMaps(oc.Namespace()).Delete(context.Background(), cm.Name, metav1.DeleteOptions{})
	}()

	// Mount the root CA certificate to the exec pod
	rootCAMountTweak := func(pod *corev1.Pod) {
		volumeMount := corev1.VolumeMount{
			Name:      "root-ca",
			MountPath: "/var/run/secrets",
		}

		volume := corev1.Volume{
			Name: "root-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm.Name,
					},
					Items: []corev1.KeyToPath{{
						Key:  "ca.crt",
						Path: "ca.crt",
					}},
				},
			},
		}

		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volumeMount)
		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}

	// Create an exec pod
	execPodName := fmt.Sprintf("%s-%s-%s", oc.Namespace(), "execpod", rand.String(5))
	execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), oc.Namespace(), execPodName, rootCAMountTweak)
	defer func() {
		oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
	}()

	cmdForStatusCode := fmt.Sprintf(`curl -s -o /dev/null -w '%%{http_code}\n' --cacert /var/run/secrets/ca.crt %q`, url)
	cmdForResponse := fmt.Sprintf("curl --cacert /var/run/secrets/ca.crt %q", url)

	var respBody string
	err = wait.PollUntilContextTimeout(context.Background(), time.Second, changeTimeoutSeconds*time.Second, false, func(ctx context.Context) (bool, error) {
		e2e.Logf("Running exec command %q", cmdForStatusCode)

		output, err := e2eoutput.RunHostCmd(oc.Namespace(), execPod.Name, cmdForStatusCode)
		if err != nil {
			e2e.Logf("Error running exec command: %s, %v, retrying...", output, err)
			return false, nil
		}

		// check if the status code is 200 OK
		if !strings.Contains(output, "200") {
			e2e.Logf("Unexpected HTTP status code: %s, retrying...", output)
			return false, nil
		}

		// Run another exec command to store the response body
		e2e.Logf("Running exec command %q", cmdForResponse)

		output, err = e2eoutput.RunHostCmd(oc.Namespace(), execPod.Name, cmdForResponse)
		if err != nil {
			e2e.Logf("Error running exec command: %s, %v, retrying...", output, err)
			return false, nil
		}
		respBody = output
		return true, nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to make successful HTTPS request after retries: %w", err)
	}

	return respBody, nil
}

// verifyRouteServesDefaultCert checks that the given hostname serves the default certificate.
func verifyRouteServesDefaultCert(oc *exutil.CLI, hostname string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	url := fmt.Sprintf("https://%s", hostname)

	var body string
	err := wait.PollUntilContextTimeout(context.Background(), time.Second, changeTimeoutSeconds*time.Second, false, func(ctx context.Context) (bool, error) {
		var err error
		var resp *http.Response
		resp, body, err = sendHttpRequestWithRetry(url, client)
		if err != nil {
			return false, err
		}

		// check that the route is serving the default certificate and not the external certificate.
		for _, cert := range resp.TLS.PeerCertificates {
			if strings.Contains(cert.Issuer.CommonName, rootCertIssuerCN) {
				e2e.Logf("Still serving external certificate: found Issuer CN=%q (expected NOT to match with %q), retrying...", cert.Issuer.CommonName, rootCertIssuerCN)
				return false, nil
			}
		}
		e2e.Logf("None of the PeerCertificates have Issuer CN matching %q", rootCertIssuerCN)
		return true, nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to verify default certificate after retries: %w", err)
	}

	return body, nil
}

// sendHttpRequestWithRetry sends an HTTP request to the specified URL using the provided client with retries.
func sendHttpRequestWithRetry(url string, client *http.Client) (*http.Response, string, error) {
	var resp *http.Response
	var body []byte

	err := wait.PollUntilContextTimeout(context.Background(), time.Second, changeTimeoutSeconds*time.Second, false, func(ctx context.Context) (bool, error) {
		e2e.Logf("Sending request to %q", url)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return false, err
		}

		resp, err = client.Do(req)
		if err != nil {
			e2e.Logf("Error making HTTPS request: %s, %v, retrying...", url, err)
			return false, nil
		}
		defer resp.Body.Close()

		// check if the status code is 200 OK
		if resp.StatusCode != http.StatusOK {
			e2e.Logf("Unexpected HTTP status code: %v, retrying...", resp.StatusCode)
			return false, nil
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			e2e.Logf("Failed to read response body: %v, retrying...", err)
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return nil, "", fmt.Errorf("failed to make successful HTTPS request after retries: %w", err)
	}

	return resp, string(body), nil
}

// checkRouteStatus polls the route status and verifies the ingress condition.
func checkRouteStatus(oc *exutil.CLI, routeName string, ingressConditionStatus corev1.ConditionStatus, ingressConditionReason string) error {
	e2e.Logf("checking route status for %q", routeName)

	ns := oc.KubeFramework().Namespace.Name
	return wait.PollUntilContextTimeout(context.Background(), time.Second, changeTimeoutSeconds*time.Second, false, func(ctx context.Context) (bool, error) {
		route, err := oc.RouteClient().RouteV1().Routes(ns).Get(context.Background(), routeName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting route %q: %v", routeName, err)
			return false, err
		}
		for _, ingress := range route.Status.Ingress {
			if len(ingress.Conditions) == 0 {
				e2e.Logf("ingress condition is empty, retrying...")
				return false, nil
			}
			for _, condition := range ingress.Conditions {
				if condition.Reason != ingressConditionReason && condition.Status != ingressConditionStatus {
					e2e.Logf("unexpected ingres condition, expected: [%s,%v] but got: [%s,%v], retrying...", ingressConditionReason, ingressConditionStatus, condition.Reason, condition.Status)
					return false, nil
				} else {
					e2e.Logf("got the expected ingres condition, reason: %s, status: %v", condition.Reason, condition.Status)
				}
			}
		}
		return true, nil
	})
}

// generateTLSCertSecret generates a TLS secret containing a certificate and key.
// The certificate is valid for the provided hostnames.
func generateTLSCertSecret(namespace, secretName string, secretType corev1.SecretType, hosts ...string) (*corev1.Secret, []byte, error) {
	// certificate start and end time are very
	// lenient to avoid any clock drift between
	// the test machine and the cluster under
	// test.
	notBefore := time.Now().Add(-24 * time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)

	// Generate crt/key for secret
	rootDerBytes, tlsCrtData, tlsPrivateKey, err := certgen.GenerateKeyPair(rootCertIssuerCN, notBefore, notAfter, hosts...)
	if err != nil {
		return nil, nil, err
	}

	derKey, err := certgen.MarshalPrivateKeyToDERFormat(tlsPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	pemCrt, err := certgen.MarshalCertToPEMString(tlsCrtData)
	if err != nil {
		return nil, nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
		},
		StringData: map[string]string{
			"tls.crt": pemCrt,
			"tls.key": derKey,
		},
		Type: secretType,
	}, rootDerBytes, nil
}

// generateRouteWithExternalCertificate creates a route with external certificate configuration.
func generateRouteWithExternalCertificate(namespace, routeName, secretName, serviceName, host string, termination routev1.TLSTerminationType) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			Host: host,
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			TLS: &routev1.TLSConfig{
				Termination: termination,
				ExternalCertificate: &routev1.LocalObjectReference{
					Name: secretName,
				},
			},
		},
	}
}

// generateSecretReaderRole creates a role that grants permissions to get, list, and watch the specified secret.
func generateSecretReaderRole(namespace, secretName string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretReaderRole,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
				Verbs:         []string{"get", "list", "watch"},
			},
		},
	}
}

// generateRouterRoleBinding creates a roleBinding that binds the secret reader role to the router service account.
func generateRouterRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretReaderRoleBinding,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "router",
				Namespace: "openshift-ingress",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     secretReaderRole,
		},
	}
}

// createNamespace creates a new namespace with the given name.
func createNamespace(oc *exutil.CLI, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := oc.AdminKubeClient().CoreV1().
		Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})

	return err
}

// patchRoleWithSecretAccess updates the "secretReaderRole" to grant access to the specified secret.
func patchRoleWithSecretAccess(oc *exutil.CLI, secretName string) error {
	newRule := fmt.Sprintf(`{"apiGroups": [""],"resources": ["secrets"],"resourceNames":["%s"],"verbs": ["get", "list", "watch"]}`, secretName)
	rolePatch := fmt.Sprintf(`{"rules": [%s]}`, newRule)
	_, err := oc.KubeClient().RbacV1().Roles(oc.Namespace()).Patch(
		context.Background(), secretReaderRole, types.MergePatchType, []byte(rolePatch), metav1.PatchOptions{},
	)
	return err
}

// patchRouteWithExternalCertificate updates the given route to use the specified external certificate secret.
func patchRouteWithExternalCertificate(oc *exutil.CLI, routeName, secretName string) error {
	routePatch := fmt.Sprintf(`{"spec":{"tls":{"externalCertificate":{"name":"%s"}}}}`, secretName)
	_, err := oc.RouteClient().RouteV1().Routes(oc.Namespace()).Patch(
		context.Background(), routeName, types.MergePatchType, []byte(routePatch), metav1.PatchOptions{},
	)
	return err
}

// patchRouteWithLabel updates the given route to add some labels. This is primarily used
// to trigger route updates.
func patchRouteWithLabel(oc *exutil.CLI, routeName string) error {
	routePatch := `{"metadata":{"labels":{"app":"myapp","key":"value"}}}`
	_, err := oc.RouteClient().RouteV1().Routes(oc.Namespace()).Patch(
		context.Background(), routeName, types.MergePatchType, []byte(routePatch), metav1.PatchOptions{},
	)
	return err
}

// patchRouteToRemoveExternalCertificate updates the given route to remove
// the external certificate reference.
func patchRouteToRemoveExternalCertificate(oc *exutil.CLI, routeName string) error {
	routePatch := `[{"op": "remove", "path": "/spec/tls/externalCertificate"}]`
	_, err := oc.RouteClient().RouteV1().Routes(oc.Namespace()).Patch(
		context.Background(), routeName, types.JSONPatchType, []byte(routePatch), metav1.PatchOptions{},
	)
	return err
}

// isMetalJob checks if the current job is running on a metal platform.
func isMetalJob(oc *exutil.CLI) (bool, error) {
	jobType, err := platformidentification.GetJobType(context.TODO(), oc.AdminConfig())
	if err != nil {
		return false, err
	}
	return jobType.Platform == "metal", nil
}
