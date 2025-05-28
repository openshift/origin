package router

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	routev1 "github.com/openshift/api/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc            = exutil.NewCLIWithoutNamespace("router-external-certificate")
		helloPodPath  = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "hello-openshift", "hello-pod.json")
		helloPodName  = "hello-openshift"
		helloPodSvc   = "hello-openshift"
		defaultDomain string
		err           error
	)

	const (
		numNamespace         = 10
		numRoutePerNamespace = 50
		repeat               = 2
	)

	g.BeforeEach(func() {
		_, err := exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		defaultDomain, err = getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")
	})

	g.Context("stress testing", func() {

		g.It("should work", func() {
			// Repeat Loop
			for rep := 0; rep < repeat; rep++ {
				var (
					namespaces []string
					secrets    []*corev1.Secret
					routes     []*routev1.Route
				)

				for i := 0; i < numNamespace; i++ {
					nsName := fmt.Sprintf("ns-ext-%d", i)
					createNamespace(oc, nsName)
					oc = oc.SetNamespace(nsName).AsAdmin()
					namespaces = append(namespaces, nsName)

					// Create hello-openshift resources in each namespace.
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

					// Create routes in each namespace
					for j := 0; j < numRoutePerNamespace; j++ {
						secretName := fmt.Sprintf("%s-secret-%d", nsName, j)
						routeName := fmt.Sprintf("%s-route-%d", nsName, j)
						host := fmt.Sprintf("host-%d-%s.%s", j, oc.Namespace(), defaultDomain)

						// Create the secret
						g.By(fmt.Sprintf("Creating secret %s in %s", secretName, oc.Namespace()))
						secret, rootDerBytes, err := generateTLSCertSecret(oc.Namespace(), secretName, corev1.SecretTypeTLS, host)
						o.Expect(err).NotTo(o.HaveOccurred())
						_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), secret, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())
						secrets = append(secrets, secret)

						// Grant the permission
						g.By(fmt.Sprintf("Granting permissions to router service account in %s", oc.Namespace()))
						err = createOrPatchRole(oc, secretName)
						o.Expect(err).NotTo(o.HaveOccurred())
						createOrPatchRoleBinding(oc)
						o.Expect(err).NotTo(o.HaveOccurred())

						// Create the route
						g.By(fmt.Sprintf("Creating route %s in %s", routeName, oc.Namespace()))
						route := generateRouteWithExternalCertificate(oc.Namespace(), routeName, secretName, helloPodSvc, host, routev1.TLSTerminationEdge)
						_, err = oc.RouteClient().RouteV1().Routes(oc.Namespace()).Create(context.Background(), route, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred())
						routes = append(routes, route)

						// Send traffic
						g.By("Sending https request")
						hostName, err := getHostnameForRoute(oc, route.Name)
						o.Expect(err).NotTo(o.HaveOccurred())
						resp, err := httpsGetCall(oc, hostName, rootDerBytes)
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(resp).Should(o.ContainSubstring(helloOpenShiftResponse))
					}
				}
				// Clean up routes
				for _, route := range routes {
					e2e.Logf("Removing route %s", route.Name)
					err = oc.RouteClient().RouteV1().Routes(route.Namespace).Delete(context.Background(), route.Name, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				// Clean up secrets
				for _, secret := range secrets {
					e2e.Logf("Removing secret %s", secret.Name)
					err = oc.KubeClient().CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				// Clean up namespaces
				for _, ns := range namespaces {
					e2e.Logf("Removing namespace %s", ns)
					err = oc.KubeClient().CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					// Wait for namespace to be fully deleted
					err = waitForNamespaceDeletion(oc, ns)
					o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("timed out waiting for namespace %s to delete", ns))
				}
			}
		})
	})
})

func createOrPatchRole(oc *exutil.CLI, secretName string) error {
	// Check if the Role exists
	// Create/Patch role
	_, err := oc.KubeClient().RbacV1().Roles(oc.Namespace()).Get(context.Background(), secretReaderRole, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = oc.KubeClient().RbacV1().Roles(oc.Namespace()).Create(context.Background(),
				generateSecretReaderRole(oc.Namespace(), secretName), metav1.CreateOptions{})
			return err
		}
		return err
	}
	e2e.Logf("patching role for secret %s", secretName)
	return patchRoleWithSecretAccess(oc, secretName)
}
func createOrPatchRoleBinding(oc *exutil.CLI) error {
	// Check if the RoleBinding exists
	_, err := oc.KubeClient().RbacV1().RoleBindings(oc.Namespace()).Get(context.Background(), secretReaderRoleBinding, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = oc.KubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(),
				generateRouterRoleBinding(oc.Namespace()), metav1.CreateOptions{})

			return err
		}
		return err
	}
	return nil
}

func waitForNamespaceDeletion(oc *exutil.CLI, name string) error {
	return wait.PollUntilContextTimeout(context.Background(), time.Second, changeTimeoutSeconds*time.Second, false, func(ctx context.Context) (done bool, err error) {
		_, err = oc.KubeClient().CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil // deleted
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}
