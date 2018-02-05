package integration

import (
	"math/rand"
	"net"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kinformers "k8s.io/client-go/informers"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/service/controller/ingressip"
	testserver "github.com/openshift/origin/test/util/server"
)

const sentinelName = "sentinel"

// TestIngressIPAllocation validates that ingress ip allocation is
// performed correctly even when multiple controllers are running.
func TestIngressIPAllocation(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	masterConfig.NetworkConfig.ExternalIPNetworkCIDRs = []string{"172.16.0.0/24"}
	masterConfig.NetworkConfig.IngressIPNetworkCIDR = "172.16.1.0/24"
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterWithOptions(masterConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	kc, _, err := configapi.GetExternalKubeClient(clusterAdminKubeConfig, &configapi.ClientConnectionOverrides{
		QPS:   20,
		Burst: 50,
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	stopChannel := make(chan struct{})
	defer close(stopChannel)
	received := make(chan bool)

	rand.Seed(time.Now().UTC().UnixNano())

	t.Log("start informer to watch for sentinel")
	_, informerController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return kc.Core().Services(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kc.Core().Services(metav1.NamespaceAll).Watch(options)
			},
		},
		&v1.Service{},
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, cur interface{}) {
				service := cur.(*v1.Service)
				if service.Name == sentinelName && len(service.Spec.ExternalIPs) > 0 {
					received <- true
				}
			},
		},
	)
	go informerController.Run(stopChannel)

	t.Log("start generating service events")
	go generateServiceEvents(t, kc)

	// Start a second controller that will be out of sync with the first
	kubeInformers := kinformers.NewSharedInformerFactory(kc, 0)
	_, ipNet, err := net.ParseCIDR(masterConfig.NetworkConfig.IngressIPNetworkCIDR)
	c := ingressip.NewIngressIPController(kubeInformers.Core().V1().Services().Informer(), kc, ipNet, 10*time.Minute)
	kubeInformers.Start(stopChannel)
	go c.Run(stopChannel)

	t.Log("waiting for sentinel to be updated with external ip")
	select {
	case <-received:
	case <-time.After(time.Duration(90 * time.Second)):
		t.Fatal("took too long")
	}

	// Validate that all services of type load balancer have a unique
	// ingress ip and corresponding external ip.
	services, err := kc.Core().Services(metav1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ips := sets.NewString()
	for _, s := range services.Items {
		typeLoadBalancer := s.Spec.Type == v1.ServiceTypeLoadBalancer
		hasAllocation := len(s.Status.LoadBalancer.Ingress) > 0
		switch {
		case !typeLoadBalancer && !hasAllocation:
			continue
		case !typeLoadBalancer && hasAllocation:
			t.Errorf("A service not of type load balancer has an ingress ip allocation")
			continue
		case typeLoadBalancer && !hasAllocation:
			t.Errorf("A service of type load balancer has not been allocated an ingress ip")
			continue
		}
		ingressIP := s.Status.LoadBalancer.Ingress[0].IP
		if ips.Has(ingressIP) {
			t.Errorf("One or more services have the same ingress ip")
			continue
		}
		ips.Insert(ingressIP)
		if len(s.Spec.ExternalIPs) == 0 || s.Spec.ExternalIPs[0] != ingressIP {
			t.Errorf("Service does not have the ingress ip as an external ip")
			continue
		}
	}
}

const (
	createOp = iota
	updateOp
	deleteOp
)

func generateServiceEvents(t *testing.T, kc kclientset.Interface) {
	maxMillisecondInterval := 25
	minServiceCount := 10
	maxOperations := minServiceCount + 30
	var services []*v1.Service
	for i := 0; i < maxOperations; {
		op := createOp
		if len(services) > minServiceCount {
			op = rand.Intn(deleteOp + 1)
		}
		switch op {
		case createOp:
			typeChoice := rand.Intn(2)
			typeLoadBalancer := false
			if typeChoice == 1 {
				typeLoadBalancer = true
			}
			s, err := createService(kc, "", typeLoadBalancer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			services = append(services, s)
			t.Logf("Added service %s", s.Name)
		case updateOp:
			targetIndex := rand.Intn(len(services))
			name := services[targetIndex].Name
			s, err := kc.Core().Services(metav1.NamespaceDefault).Get(name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			// Flip the service type
			if s.Spec.Type == v1.ServiceTypeLoadBalancer {
				s.Spec.Type = v1.ServiceTypeClusterIP
				s.Spec.Ports[0].NodePort = 0
			} else {
				s.Spec.Type = v1.ServiceTypeLoadBalancer
			}
			s, err = kc.Core().Services(metav1.NamespaceDefault).Update(s)
			if err != nil {
				continue
			}
			t.Logf("Updated service %s", name)
		case deleteOp:
			targetIndex := rand.Intn(len(services))
			name := services[targetIndex].Name
			err := kc.Core().Services(metav1.NamespaceDefault).Delete(name, nil)
			if err != nil {
				continue
			}
			services = append(services[:targetIndex], services[targetIndex+1:]...)
			t.Logf("Deleted service %s", name)
		}
		i++
		time.Sleep(time.Duration(rand.Intn(maxMillisecondInterval)) * time.Millisecond)
	}

	// Create one last service to serve as a sentinel. The service
	// will be created after a slight delay so that it can be assured
	// of being the last service a controller will see, and with a
	// known name so its processing can be detected.
	time.Sleep(time.Millisecond * 100)
	_, err := createService(kc, sentinelName, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func createService(kc kclientset.Interface, name string, typeLoadBalancer bool) (*v1.Service, error) {
	serviceType := v1.ServiceTypeClusterIP
	if typeLoadBalancer {
		serviceType = v1.ServiceTypeLoadBalancer
	}
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "service-",
			Name:         name,
		},
		Spec: v1.ServiceSpec{
			Type: serviceType,
			Ports: []v1.ServicePort{{
				Protocol: "TCP",
				Port:     8080,
			}},
		},
	}
	return kc.Core().Services(metav1.NamespaceDefault).Create(service)
}
