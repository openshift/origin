package integration

import (
	"math/rand"
	"net"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/service/controller/ingressip"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const sentinelName = "sentinel"

// TestIngressIPAllocation validates that ingress ip allocation is
// performed correctly even when multiple controllers are running.
func TestIngressIPAllocation(t *testing.T) {
	testutil.RequireEtcd(t)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	masterConfig.NetworkConfig.ExternalIPNetworkCIDRs = []string{"172.16.0.0/24"}
	masterConfig.NetworkConfig.IngressIPNetworkCIDR = "172.16.1.0/24"
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterWithOptions(masterConfig, testserver.TestOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	kc, _, err := configapi.GetKubeClient(clusterAdminKubeConfig, &configapi.ClientConnectionOverrides{
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
	_, informerController := framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return kc.Services(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return kc.Services(kapi.NamespaceAll).Watch(options)
			},
		},
		&kapi.Service{},
		time.Minute*10,
		framework.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, cur interface{}) {
				service := cur.(*kapi.Service)
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
	_, ipNet, err := net.ParseCIDR(masterConfig.NetworkConfig.IngressIPNetworkCIDR)
	c := ingressip.NewIngressIPController(kc, ipNet, 10*time.Minute)
	go c.Run(stopChannel)

	t.Log("waiting for sentinel to be updated with external ip")
	select {
	case <-received:
	case <-time.After(time.Duration(90 * time.Second)):
		t.Fatal("took too long")
	}

	// Validate that all services of type load balancer have a unique
	// ingress ip and corresponding external ip.
	services, err := kc.Services(kapi.NamespaceDefault).List(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	ips := sets.NewString()
	for _, s := range services.Items {
		typeLoadBalancer := s.Spec.Type == kapi.ServiceTypeLoadBalancer
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

func generateServiceEvents(t *testing.T, kc kclient.Interface) {
	maxMillisecondInterval := 25
	minServiceCount := 10
	maxOperations := minServiceCount + 30
	var services []*kapi.Service
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
			s, err := kc.Services(kapi.NamespaceDefault).Get(name)
			if err != nil {
				continue
			}
			// Flip the service type
			if s.Spec.Type == kapi.ServiceTypeLoadBalancer {
				s.Spec.Type = kapi.ServiceTypeClusterIP
				s.Spec.Ports[0].NodePort = 0
			} else {
				s.Spec.Type = kapi.ServiceTypeLoadBalancer
			}
			s, err = kc.Services(kapi.NamespaceDefault).Update(s)
			if err != nil {
				continue
			}
			t.Logf("Updated service %s", name)
		case deleteOp:
			targetIndex := rand.Intn(len(services))
			name := services[targetIndex].Name
			err := kc.Services(kapi.NamespaceDefault).Delete(name)
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

func createService(kc kclient.Interface, name string, typeLoadBalancer bool) (*kapi.Service, error) {
	serviceType := kapi.ServiceTypeClusterIP
	if typeLoadBalancer {
		serviceType = kapi.ServiceTypeLoadBalancer
	}
	service := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: "service-",
			Name:         name,
		},
		Spec: kapi.ServiceSpec{
			Type: serviceType,
			Ports: []kapi.ServicePort{{
				Protocol: "TCP",
				Port:     8080,
			}},
		},
	}
	return kc.Services(kapi.NamespaceDefault).Create(service)
}
