package cmd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktc "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
)

func exposeData() []*kapi.Service {
	// Doesn't support TCP.
	udp := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo", Namespace: "test", ResourceVersion: "12",
		},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{"service": "test"},
			Ports: []kapi.ServicePort{
				{
					Protocol:   kapi.ProtocolUDP,
					Port:       80,
					TargetPort: util.NewIntOrStringFromInt(80),
				},
			},
		},
	}
	// Supports TCP, has numeric target port.
	numeric := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo", Namespace: "test", ResourceVersion: "12",
		},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{"service": "test"},
			Ports: []kapi.ServicePort{
				{
					Protocol:   kapi.ProtocolTCP,
					Port:       80,
					TargetPort: util.NewIntOrStringFromInt(80),
				},
			},
		},
	}
	// Supports TCP, has named port.
	named := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo", Namespace: "test", ResourceVersion: "12",
		},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{"service": "test"},
			Ports: []kapi.ServicePort{
				{
					Protocol:   kapi.ProtocolTCP,
					Port:       80,
					Name:       "http",
					TargetPort: util.NewIntOrStringFromString("http"),
				},
			},
		},
	}
	// Supports TCP, has multiple ports.
	multipleTCPPorts := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo", Namespace: "test", ResourceVersion: "12",
		},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{"service": "test"},
			Ports: []kapi.ServicePort{
				{
					Protocol:   kapi.ProtocolTCP,
					Port:       80,
					Name:       "http",
					TargetPort: util.NewIntOrStringFromString("http"),
				},
				{
					Protocol:   kapi.ProtocolTCP,
					Port:       443,
					Name:       "https",
					TargetPort: util.NewIntOrStringFromString("443"),
				},
			},
		},
	}
	// Doesn't support TCP, has multiple ports.
	multipleUDPPorts := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo", Namespace: "test", ResourceVersion: "12",
		},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{"service": "test"},
			Ports: []kapi.ServicePort{
				{
					Protocol:   kapi.ProtocolUDP,
					Port:       7,
					Name:       "echo",
					TargetPort: util.NewIntOrStringFromString("echo"),
				},
				{
					Protocol:   kapi.ProtocolUDP,
					Port:       23,
					Name:       "telnet",
					TargetPort: util.NewIntOrStringFromInt(23),
				},
			},
		},
	}

	return []*kapi.Service{udp, numeric, named, multipleTCPPorts, multipleUDPPorts}
}

func TestValidateService(t *testing.T) {
	services := exposeData()
	tests := []struct {
		name         string
		service      *kapi.Service
		portFlagUsed string
		expectedPort string
		expectedErr  error
	}{
		{
			name:         "udp service",
			service:      services[0],
			expectedPort: "",
			expectedErr:  noTCPErr,
		},
		{
			name:         "another udp service",
			service:      services[4],
			portFlagUsed: "echo",
			expectedPort: "",
			expectedErr:  noTCPErr,
		},
		{
			name:         "round robin tcp service",
			service:      services[1],
			expectedPort: "",
		},
		{
			name:         "tcp multiport service",
			service:      services[3],
			expectedPort: "http",
		},
		{
			name:         "numeric --target-port",
			service:      services[3],
			portFlagUsed: "443",
			expectedPort: "443",
		},
		{
			name:         "non-numeric --target-port",
			service:      services[2],
			portFlagUsed: "http",
			expectedPort: "http",
		},
		{
			name:         "invalid numeric --target-port",
			service:      services[3],
			portFlagUsed: "8080",
			expectedPort: "",
			expectedErr:  invalidPortErr,
		},
		{
			name:         "invalid non-numeric --target-port",
			service:      services[1],
			portFlagUsed: "udp",
			expectedPort: "",
			expectedErr:  invalidPortErr,
		},
	}

	for _, test := range tests {
		fake := &ktc.Fake{}
		fake.AddReactor("get", "services", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
			return true, test.service, nil
		})

		gotPort, gotErr := validateService(fake, "test", "someservice", test.portFlagUsed)
		if gotErr != test.expectedErr {
			t.Errorf("%s: error mismatch: got %v, expected %v", test.name, gotErr, test.expectedErr)
		}
		if gotPort != test.expectedPort {
			t.Errorf("%s: port mismatch: got %s, expected %s", test.name, gotPort, test.expectedPort)
		}
	}
}
