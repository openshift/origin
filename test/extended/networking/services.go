package networking

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/origin/pkg/test"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("[sig-network] services", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("ns-global", admissionapi.LevelBaseline)
	var retryInterval = 1 * time.Minute

	InIPv4ClusterContext(oc, func() {
		It("ensures external ip policy is configured correctly on the cluster [apigroup:config.openshift.io] [Serial]", test.ExtendedDuration(), func() {
			// Check if the test can write to cluster/network.config.openshift.io
			hasAccess, err := hasNetworkConfigWriteAccess(oc)
			Expect(err).NotTo(HaveOccurred())
			if !hasAccess {
				skipper.Skipf("The test is not permitted to modify the cluster/network.config.openshift.io resource")
			}

			namespace := oc.Namespace()
			adminConfigClient := oc.AdminConfigClient()
			k8sClient := oc.KubeClient()
			// Test a load balancer service with default cluster networks config
			// In this case service creation must throw an error for non admin user
			By("create service of type load balancer with default cluster networks config")
			serviceName := names.SimpleNameGenerator.GenerateName("svc-without-ext-ip")
			By("check load balance service creation fails")
			err = createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
			Expect(kapierrs.IsForbidden(err)).Should(Equal(true))

			// Test external ip policy configured with allowedCIDRs. Make sure service
			// is created if that is within allowedCIDRs range and service creation
			// fails if its outside the allowed range.
			By("update network config with allowed cidr for external ip")
			modifyNetworkConfig(adminConfigClient, nil, []string{"192.168.132.10/32"}, nil)
			By("check service is within external ip within allowed range")
			for {
				serviceName = names.SimpleNameGenerator.GenerateName("svc-with-ext-ip")
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
				if err != nil {
					e2e.Logf("error occurred while creating %s/%s service: %v: reason: %v", namespace, serviceName, err, kapierrs.ReasonForError(err))
				}
				if err != nil && strings.Contains(err.Error(), "connection reset by peer") {
					// kube api server is still rolling out changed external ip configuration and somehow its closing client connection
					// while its in progress. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				if err != nil && kapierrs.IsForbidden(err) {
					// external ip configuration rollout is not complete. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				expectNoError(err)
				break
			}
			By("check service creation fails when external ip outside the allowed range")
			serviceName = names.SimpleNameGenerator.GenerateName("svc-with-ext-ip")
			err = createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.20"}, nil)
			Expect(kapierrs.IsForbidden(err)).Should(Equal(true))

			// Revert cluster networks config into default settings and make sure
			// service creation must fail with an error for non admin user.
			By("update network config without external ip")
			modifyNetworkConfig(adminConfigClient, []string{}, []string{}, []string{})
			By("check load balance service creation fails")
			for {
				serviceName = names.SimpleNameGenerator.GenerateName("svc-without-ext-ip-2")
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
				if err != nil {
					e2e.Logf("error occurred while creating %s/%s service: %v: reason: %v", namespace, serviceName, err, kapierrs.ReasonForError(err))
				}
				if err != nil && strings.Contains(err.Error(), "connection reset by peer") {
					// kube api server is still rolling out changed external ip configuration and somehow its closing client connection
					// while its in progress. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				if err == nil {
					e2e.Logf("error not occurred while creating %s/%s service", namespace, serviceName)
					// external ip configuration rollout is not complete. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				Expect(kapierrs.IsForbidden(err)).Should(Equal(true))
				break
			}
		})
	})

	InBareMetalIPv4ClusterContext(oc, func() {
		It("ensures external auto assign cidr is configured correctly on the cluster [apigroup:config.openshift.io] [Serial]", test.ExtendedDuration(), func() {
			// Check if the test can write to cluster/network.config.openshift.io
			hasAccess, err := hasNetworkConfigWriteAccess(oc)
			Expect(err).NotTo(HaveOccurred())
			if !hasAccess {
				skipper.Skipf("The test is not permitted to modify the cluster/network.config.openshift.io resource")
			}

			namespace := oc.Namespace()
			adminConfigClient := oc.AdminConfigClient()
			k8sClient := oc.KubeClient()
			serviceClient := k8sClient.CoreV1().Services(namespace)
			// Test a load balancer service with default cluster networks config
			// In this case service creation must throw an error for non admin user.
			By("create service of type load balancer with default cluster networks config")
			serviceName := names.SimpleNameGenerator.GenerateName("svc-without-ext-ip-3")
			By("check load balance service creation fails")
			err = createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
			Expect(kapierrs.IsForbidden(err)).Should(Equal(true))

			// Test external ip policy configured with both policy and auto assign cidr. Make sure service
			// is assigned with an ip address from auto assign cidr.
			By("update network config with auto assign cidr")
			modifyNetworkConfig(adminConfigClient, []string{"192.168.132.254/29"}, []string{"192.168.132.0/29"}, []string{"192.168.132.8/29"})
			By("check load balancer service having desired ingress ip prefix")
			for {
				serviceName = names.SimpleNameGenerator.GenerateName("svc-ext-ip-auto-assign")
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{}, nil)
				if err != nil {
					e2e.Logf("error occurred while creating %s/%s service: %v: reason: %v", namespace, serviceName, err, kapierrs.ReasonForError(err))
				}
				if err != nil && strings.Contains(err.Error(), "connection reset by peer") {
					// kube api server is still rolling out changed external ip configuration and somehow its closing client connection
					// while its in progress. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				if err != nil && kapierrs.IsForbidden(err) {
					// external ip configuration rollout is not complete. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
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
				if !strings.HasPrefix(ingressIP, "192.168.132") {
					// external ip configuration rollout is not complete. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				break
			}

			// Revert cluster networks config into default settings and make sure
			// service creation must fail with an error for non admin user.
			By("update network config without external ip")
			modifyNetworkConfig(adminConfigClient, []string{}, []string{}, []string{})
			By("check load balance service creation fails")
			for {
				serviceName = names.SimpleNameGenerator.GenerateName("svc-without-ext-ip-4")
				err := createWebserverLBService(k8sClient, namespace, serviceName, "", []string{"192.168.132.10"}, nil)
				if err != nil {
					e2e.Logf("error occurred while creating %s/%s service: %v: reason: %v", namespace, serviceName, err, kapierrs.ReasonForError(err))
				}
				if err != nil && strings.Contains(err.Error(), "connection reset by peer") {
					// kube api server is still rolling out changed external ip configuration and somehow its closing client connection
					// while its in progress. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				if err == nil {
					// external ip configuration rollout is not complete. so retry after retryInterval.
					e2e.Logf("external ip config rollout is in progress, retry creating service %s until it succeeds or exceeds test timeout 30 mins", serviceName)
					time.Sleep(retryInterval)
					continue
				}
				Expect(kapierrs.IsForbidden(err)).Should(Equal(true))
				break
			}
		})
	})
})
