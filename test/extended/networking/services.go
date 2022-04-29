package networking

import (
	admissionapi "k8s.io/pod-security-admission/api"

	"context"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("[sig-network] services", func() {
	Context("basic functionality", func() {
		f1 := e2e.NewDefaultFramework("net-services1")
		// TODO(sur): verify if privileged is really necessary in a follow-up
		f1.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

		It("should allow connections to another pod on the same node via a service IP", func() {
			Expect(checkServiceConnectivity(f1, f1, SAME_NODE)).To(Succeed())
		})

		It("should allow connections to another pod on a different node via a service IP", func() {
			Expect(checkServiceConnectivity(f1, f1, DIFFERENT_NODE)).To(Succeed())
		})
	})

	InNonIsolatingContext(func() {
		f1 := e2e.NewDefaultFramework("net-services1")
		// TODO(sur): verify if privileged is really necessary in a follow-up
		f1.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
		f2 := e2e.NewDefaultFramework("net-services2")
		// TODO(sur): verify if privileged is really necessary in a follow-up
		f2.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

		It("should allow connections to pods in different namespaces on the same node via service IPs", func() {
			Expect(checkServiceConnectivity(f1, f2, SAME_NODE)).To(Succeed())
		})

		It("should allow connections to pods in different namespaces on different nodes via service IPs", func() {
			Expect(checkServiceConnectivity(f1, f2, DIFFERENT_NODE)).To(Succeed())
		})
	})

	oc := exutil.NewCLI("ns-global")

	InIsolatingContext(func() {
		f1 := e2e.NewDefaultFramework("net-services1")
		// TODO(sur): verify if privileged is really necessary in a follow-up
		f1.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
		f2 := e2e.NewDefaultFramework("net-services2")
		// TODO(sur): verify if privileged is really necessary in a follow-up
		f2.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

		It("should prevent connections to pods in different namespaces on the same node via service IPs", func() {
			Expect(checkServiceConnectivity(f1, f2, SAME_NODE)).NotTo(Succeed())
		})

		It("should prevent connections to pods in different namespaces on different nodes via service IPs", func() {
			Expect(checkServiceConnectivity(f1, f2, DIFFERENT_NODE)).NotTo(Succeed())
		})

		It("should allow connections to services in the default namespace from a pod in another namespace on the same node", func() {
			makeNamespaceGlobal(oc, f1.Namespace)
			Expect(checkServiceConnectivity(f1, f2, SAME_NODE)).To(Succeed())
		})
		It("should allow connections to services in the default namespace from a pod in another namespace on a different node", func() {
			makeNamespaceGlobal(oc, f1.Namespace)
			Expect(checkServiceConnectivity(f1, f2, DIFFERENT_NODE)).To(Succeed())
		})
		It("should allow connections from pods in the default namespace to a service in another namespace on the same node", func() {
			makeNamespaceGlobal(oc, f2.Namespace)
			Expect(checkServiceConnectivity(f1, f2, SAME_NODE)).To(Succeed())
		})
		It("should allow connections from pods in the default namespace to a service in another namespace on a different node", func() {
			makeNamespaceGlobal(oc, f2.Namespace)
			Expect(checkServiceConnectivity(f1, f2, DIFFERENT_NODE)).To(Succeed())
		})
	})

	var retryInterval = 1 * time.Minute

	Context("external ip", func() {
		It("ensures policy is configured correctly on the cluster [Serial]", func() {
			namespace := oc.Namespace()
			adminConfigClient := oc.AdminConfigClient()
			k8sClient := oc.KubeClient()
			serviceClient := k8sClient.CoreV1().Services(namespace)
			// Test a load balancer service with default cluster networks config
			// In this case service creation must throw an error for non admin user
			By("create service of type load balancer with default cluster networks config")
			serviceName := "svc-without-ext-ip"
			By("check load balance service creation fails")
			err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
			deleteService(serviceClient, serviceName)
			Expect(kapierrs.IsForbidden(err)).Should(Equal(true))

			// Test external ip policy configured with allowedCIDRs. Make sure service
			// is created if that is within allowedCIDRs range and service creation
			// fails if its outside the allowed range.
			By("update network config with allowed cidr for external ip")
			modifyNetworkConfig(adminConfigClient, nil, []string{"192.168.132.10/32"}, nil)
			serviceName = "svc-with-ext-ip"
			By("check service is within external ip within allowed range")
			for {
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
				deleteService(serviceClient, serviceName)
				if err != nil && kapierrs.IsForbidden(err) {
					time.Sleep(retryInterval)
					continue
				}
				expectNoError(err)
				break
			}
			By("check service creation fails when external ip outside the allowed range")
			err = createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.20"}, nil)
			deleteService(serviceClient, serviceName)
			Expect(kapierrs.IsForbidden(err)).Should(Equal(true))

			// Revert cluster networks config into default settings and make sure
			// service creation must fail with an error for non admin user.
			By("update network config without external ip")
			modifyNetworkConfig(adminConfigClient, []string{}, []string{}, []string{})
			serviceName = "svc-without-ext-ip-2"
			By("check load balance service creation fails")
			for {
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
				deleteService(serviceClient, serviceName)
				if err == nil {
					e2e.Logf("error not occurred while creating %s/%s service", namespace, serviceName)
					time.Sleep(retryInterval)
					continue
				}
				e2e.Logf("error occurred while creating %s/%s service: %v", namespace, serviceName, err)
				Expect(kapierrs.IsForbidden(err)).Should(Equal(true))
				break
			}
		})
	})

	InBareMetalClusterContext(oc, func() {
		It("ensures external auto assign cidr is configured correctly on the cluster [Serial]", func() {
			namespace := oc.Namespace()
			adminConfigClient := oc.AdminConfigClient()
			k8sClient := oc.KubeClient()
			serviceClient := k8sClient.CoreV1().Services(namespace)
			// Test a load balancer service with default cluster networks config
			// In this case service creation must throw an error for non admin user.
			By("create service of type load balancer with default cluster networks config")
			serviceName := "svc-without-ext-ip-3"
			By("check load balance service creation fails")
			err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
			Expect(kapierrs.IsForbidden(err)).Should(Equal(true))

			// Test external ip policy configured with both policy and auto assign cidr. Make sure service
			// is assigned with an ip address from auto assign cidr.
			By("update network config with auto assign cidr")
			modifyNetworkConfig(adminConfigClient, []string{"192.168.132.254/29"}, []string{"192.168.132.0/29"}, []string{"192.168.132.8/29"})
			serviceName = "svc-ext-ip-auto-assign"
			By("check load balancer service having desired ingress ip prefix")
			for {
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{}, nil)
				if err != nil && kapierrs.IsForbidden(err) {
					deleteService(serviceClient, serviceName)
					time.Sleep(retryInterval)
					continue
				}
				expectNoError(err)
				service, err := serviceClient.Get(context.Background(), serviceName, metav1.GetOptions{})
				expectNoError(err)
				var ingressIP string
				if len(service.Status.LoadBalancer.Ingress) > 0 {
					ingressIP = service.Status.LoadBalancer.Ingress[0].IP
				}
				deleteService(serviceClient, serviceName)
				if !strings.HasPrefix(ingressIP, "192.168.132") {
					time.Sleep(retryInterval)
					continue
				}
				break
			}

			// Revert cluster networks config into default settings and make sure
			// service creation must fail with an error for non admin user.
			By("update network config without external ip")
			modifyNetworkConfig(adminConfigClient, []string{}, []string{}, []string{})
			serviceName = "svc-without-ext-ip-4"
			By("check load balance service creation fails")
			for {
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
				deleteService(serviceClient, serviceName)
				if err == nil {
					time.Sleep(retryInterval)
					continue
				}
				Expect(kapierrs.IsForbidden(err)).Should(Equal(true))
				break
			}
		})
	})
})
