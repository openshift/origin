package networking

import (
	"context"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	utilnet "k8s.io/utils/net"
)

var _ = g.Describe("[sig-network][endpoints] admission [apigroup:config.openshift.io]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("endpoint-admission")

	var clusterAdminKubeClient, projectAdminClient kubernetes.Interface
	var clusterIP, serviceIP string

	g.BeforeEach(func() {
		clusterAdminKubeClient = oc.AdminKubeClient()
		projectAdminClient = oc.KubeClient()

		networkConfig, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, clusterCIDR, err := net.ParseCIDR(networkConfig.Status.ClusterNetwork[0].CIDR)
		o.Expect(err).NotTo(o.HaveOccurred())
		ip, err := cidr.Host(clusterCIDR, 3)
		o.Expect(err).NotTo(o.HaveOccurred())
		clusterIP = ip.String()

		_, serviceCIDR, err := net.ParseCIDR(networkConfig.Status.ServiceNetwork[0])
		o.Expect(err).NotTo(o.HaveOccurred())
		ip, err = cidr.Host(serviceCIDR, 3)
		o.Expect(err).NotTo(o.HaveOccurred())
		serviceIP = ip.String()
	})

	g.It("blocks manual creation of Endpoints pointing to the cluster or service network", g.Label("Size:S"), func() {
		serviceAccountClient, _, err := getClientForServiceAccount(clusterAdminKubeClient, rest.AnonymousClientConfig(oc.AdminConfig()), "kube-system", "endpoint-controller")
		o.Expect(err).NotTo(o.HaveOccurred(), "error getting endpoint controller service account")

		// Cluster admin
		testOneEndpoint(oc, clusterAdminKubeClient, "cluster", clusterIP, true)
		testOneEndpoint(oc, clusterAdminKubeClient, "service", serviceIP, true)
		testOneEndpoint(oc, clusterAdminKubeClient, "external", "1.2.3.4", true)

		// Endpoint controller service account
		testOneEndpoint(oc, serviceAccountClient, "cluster", clusterIP, true)
		testOneEndpoint(oc, serviceAccountClient, "service", serviceIP, true)
		testOneEndpoint(oc, serviceAccountClient, "external", "1.2.3.4", true)

		// Project admin
		testOneEndpoint(oc, projectAdminClient, "cluster", clusterIP, false)
		testOneEndpoint(oc, projectAdminClient, "service", serviceIP, false)
		// FIXME: This was allowed by default up to OCP 4.20 but will be flipped
		// to deny-by-default once the k8s 1.34 rebase is finished, at which
		// point this (and the other chunk below) should be uncommented.
		// testOneEndpoint(oc, projectAdminClient, "external", "1.2.3.4", false)

		// Now try giving the project admin endpoints create/edit permission...
		out, err := oc.AsAdmin().Run("create", "role", "--namespace", oc.Namespace(), "endpoints-edit", "--verb=create,update,patch", "--resource=endpoints").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "error adding endpoints-edit role: %s", out)
		out, err = oc.AsAdmin().Run("create", "rolebinding", "--namespace", oc.Namespace(), "user-endpoints-edit", "--role=endpoints-edit", "--user", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "error adding user-endpoints-edit rolebinding: ", out)

		// Project admin + endpoints edit; restricted IPs are still blocked, but
		// the external IP will work now
		testOneEndpoint(oc, projectAdminClient, "cluster", clusterIP, false)
		testOneEndpoint(oc, projectAdminClient, "service", serviceIP, false)
		testOneEndpoint(oc, projectAdminClient, "external", "1.2.3.4", true)

		// User with endpoints edit permission but without endpoints/restricted
		// permission can't modify IPs but can still do other modifications
		ep := testOneEndpoint(oc, clusterAdminKubeClient, "cluster", clusterIP, true)
		ep.Annotations = map[string]string{"foo": "bar"}
		ep, err = projectAdminClient.CoreV1().Endpoints(oc.Namespace()).Update(context.Background(), ep, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "unexpected error updating endpoint annotation")

		// FIXME: as above, uncomment after 1.34 rebase
		// ep.Subsets[0].Addresses[0].IP = serviceIP
		// ep, err = projectAdminClient.CoreV1().Endpoints(oc.Namespace()).Update(context.Background(), ep, metav1.UpdateOptions{})
		// o.Expect(err).To(o.HaveOccurred(), "unexpected success modifying endpoint")
	})

	g.It("blocks manual creation of EndpointSlices pointing to the cluster or service network", g.Label("Size:S"), func() {
		serviceAccountClient, _, err := getClientForServiceAccount(clusterAdminKubeClient, rest.AnonymousClientConfig(oc.AdminConfig()), "kube-system", "endpointslice-controller")
		o.Expect(err).NotTo(o.HaveOccurred(), "error getting endpoint controller service account")

		// Cluster admin
		testOneEndpointSlice(oc, clusterAdminKubeClient, "cluster", clusterIP, true)
		testOneEndpointSlice(oc, clusterAdminKubeClient, "service", serviceIP, true)
		testOneEndpointSlice(oc, clusterAdminKubeClient, "external", "1.2.3.4", true)

		// Endpoint controller service account
		testOneEndpointSlice(oc, serviceAccountClient, "cluster", clusterIP, true)
		testOneEndpointSlice(oc, serviceAccountClient, "service", serviceIP, true)
		testOneEndpointSlice(oc, serviceAccountClient, "external", "1.2.3.4", true)

		// Project admin
		testOneEndpointSlice(oc, projectAdminClient, "cluster", clusterIP, false)
		testOneEndpointSlice(oc, projectAdminClient, "service", serviceIP, false)
		testOneEndpointSlice(oc, projectAdminClient, "external", "1.2.3.4", false)

		// Now try giving the project admin endpointslice create/edit permission...
		out, err := oc.AsAdmin().Run("create", "role", "--namespace", oc.Namespace(), "endpointslice-edit", "--verb=create,update,patch", "--resource=endpointslices.discovery.k8s.io").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "error adding endpointslice-edit role: %s", out)
		out, err = oc.AsAdmin().Run("create", "rolebinding", "--namespace", oc.Namespace(), "user-endpointslice-edit", "--role=endpointslice-edit", "--user", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "error adding user-endpointslice-edit rolebinding: ", out)

		// Project admin + endpointslice edit; restricted IPs are still blocked, but
		// the external IP will work now
		testOneEndpointSlice(oc, projectAdminClient, "cluster", clusterIP, false)
		testOneEndpointSlice(oc, projectAdminClient, "service", serviceIP, false)
		testOneEndpointSlice(oc, projectAdminClient, "external", "1.2.3.4", true)

		// User with endpointslice edit permission but without endpointslices/restricted
		// permission can't modify IPs but can still do other modifications
		slice := testOneEndpointSlice(oc, clusterAdminKubeClient, "cluster", clusterIP, true)
		slice.Annotations = map[string]string{"foo": "bar"}
		slice, err = projectAdminClient.DiscoveryV1().EndpointSlices(oc.Namespace()).Update(context.Background(), slice, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "unexpected error updating endpoint annotation")

		slice.Endpoints[0].Addresses[0] = serviceIP
		slice, err = projectAdminClient.DiscoveryV1().EndpointSlices(oc.Namespace()).Update(context.Background(), slice, metav1.UpdateOptions{})
		o.Expect(err).To(o.HaveOccurred(), "unexpected success modifying endpoint")
	})
})

func testOneEndpoint(oc *exutil.CLI, client kubernetes.Interface, addrType, addr string, success bool) *corev1.Endpoints {
	testEndpoint := &corev1.Endpoints{}
	testEndpoint.GenerateName = "test"
	testEndpoint.Subsets = []corev1.EndpointSubset{
		{
			Addresses: []corev1.EndpointAddress{
				{
					IP: addr,
				},
			},
			Ports: []corev1.EndpointPort{
				{
					Port:     9999,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	ep, err := client.CoreV1().Endpoints(oc.Namespace()).Create(context.Background(), testEndpoint, metav1.CreateOptions{})
	if success {
		o.Expect(err).NotTo(o.HaveOccurred(), "unexpected error creating %s endpoint", addrType)
	} else {
		o.Expect(err).To(o.HaveOccurred(), "unexpected success creating %s endpoint", addrType)
	}
	return ep
}

func testOneEndpointSlice(oc *exutil.CLI, client kubernetes.Interface, addrType, addr string, success bool) *discoveryv1.EndpointSlice {
	addressType := discoveryv1.AddressTypeIPv4
	if utilnet.IsIPv6String(addr) {
		addressType = discoveryv1.AddressTypeIPv6
	}
	port := int32(9999)
	protocol := corev1.ProtocolTCP

	testSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test",
		},
		AddressType: addressType,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{
					addr,
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Port:     &port,
				Protocol: &protocol,
			},
		},
	}

	slice, err := client.DiscoveryV1().EndpointSlices(oc.Namespace()).Create(context.Background(), testSlice, metav1.CreateOptions{})
	if success {
		o.Expect(err).NotTo(o.HaveOccurred(), "unexpected error creating %s endpointslice", addrType)
	} else {
		o.Expect(err).To(o.HaveOccurred(), "unexpected success creating %s endpointslice", addrType)
	}
	return slice
}

func getClientForServiceAccount(adminClient kubernetes.Interface, clientConfig *rest.Config, namespace, name string) (*kubernetes.Clientset, *rest.Config, error) {
	_, err := adminClient.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, nil, err
		}
		_, err = adminClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
		if err != nil {
			return nil, nil, err
		}
	}

	_, err = adminClient.CoreV1().ServiceAccounts(namespace).Create(context.Background(), &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name}}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, nil, err
	}

	tokenRequest, err := adminClient.CoreV1().ServiceAccounts(namespace).CreateToken(context.Background(), name, &authenticationv1.TokenRequest{}, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, err
	}

	saClientConfig := rest.AnonymousClientConfig(clientConfig)
	saClientConfig.BearerToken = tokenRequest.Status.Token

	kubeClientset, err := kubernetes.NewForConfig(saClientConfig)
	if err != nil {
		return nil, nil, err
	}

	return kubeClientset, saClientConfig, nil
}
