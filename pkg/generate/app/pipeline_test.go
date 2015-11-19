package app

import (
	"fmt"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type portDesc struct {
	port     int
	protocol string
}

type containerDesc struct {
	name  string
	ports []portDesc
}

func fakeDeploymentConfig(name string, containers ...containerDesc) *deployapi.DeploymentConfig {
	specContainers := []kapi.Container{}
	for _, c := range containers {
		container := kapi.Container{
			Name: c.name,
		}

		container.Ports = []kapi.ContainerPort{}
		for _, p := range c.ports {
			container.Ports = append(container.Ports, kapi.ContainerPort{
				Name:          fmt.Sprintf("port-%d-%s", p.port, p.protocol),
				ContainerPort: p.port,
				Protocol:      kapi.Protocol(p.protocol),
			})
		}

		specContainers = append(specContainers, container)
	}
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Template: deployapi.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{"name": "test"},
				Template: &kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						Containers: specContainers,
					},
				},
			},
		},
	}
}

func expectedService(name string, ports ...portDesc) *kapi.Service {
	servicePorts := []kapi.ServicePort{}
	for _, p := range ports {
		servicePorts = append(servicePorts, kapi.ServicePort{
			// Name is derived purely from the port and protocol, ignoring the container port name
			Name:       fmt.Sprintf("%d-%s", p.port, p.protocol),
			Port:       p.port,
			Protocol:   kapi.Protocol(p.protocol),
			TargetPort: kutil.NewIntOrStringFromInt(p.port),
		})
	}

	return &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: kapi.ServiceSpec{
			Selector: map[string]string{"name": "test"},
			Ports:    servicePorts,
		},
	}
}

func getServices(objects Objects) Objects {
	result := Objects{}
	for _, obj := range objects {
		if _, isSvc := obj.(*kapi.Service); isSvc {
			result = append(result, obj)
		}
	}
	return result
}

func objsToString(objs Objects) string {
	result := "Objects{"
	for _, obj := range objs {
		result += fmt.Sprintf("\t%#v\n", obj)

	}
	result += "}"
	return result
}

func TestAcceptUnique(t *testing.T) {
	is := func(name, ns string) *imageapi.ImageStream {
		obj := &imageapi.ImageStream{}
		obj.Name = name
		obj.Namespace = ns
		return obj
	}
	dc := func(name, ns string) *deployapi.DeploymentConfig {
		obj := &deployapi.DeploymentConfig{}
		obj.Name = name
		obj.Namespace = ns
		return obj
	}
	objs := func(list ...runtime.Object) []runtime.Object {
		return list
	}
	tests := []struct {
		name   string
		objs   []runtime.Object
		expect int
	}{
		{
			name:   "same name, different kind, different ns",
			objs:   objs(is("aaa", "ns1"), is("aaa", "ns2"), dc("aaa", "ns1")),
			expect: 3,
		},
		{
			name:   "dup name, empty ns",
			objs:   objs(is("aaa", ""), is("aaa", "")),
			expect: 1,
		},
		{
			name:   "different name, empty ns",
			objs:   objs(is("aaa", ""), is("bbb", ""), dc("aaa", "")),
			expect: 3,
		},
	}
	for _, tc := range tests {
		au := NewAcceptUnique(kapi.Scheme)
		cnt := 0
		for _, obj := range tc.objs {
			if au.Accept(obj) {
				cnt++
			}
		}
		if cnt != tc.expect {
			t.Errorf("%s: did not get expected number of objects. Expected: %d, Got: %d", tc.name, tc.expect, cnt)
		}
	}
}

func TestAddServices(t *testing.T) {
	tests := []struct {
		name             string
		input            Objects
		firstOnly        bool
		expectedServices Objects
	}{
		{
			name: "single port",
			input: Objects{
				fakeDeploymentConfig("singleport", containerDesc{"test", []portDesc{{100, "tcp"}}}),
			},
			expectedServices: Objects{
				expectedService("singleport", portDesc{100, "tcp"}),
			},
		},
		{
			name: "multiple containers",
			input: Objects{
				fakeDeploymentConfig("multicontainer",
					containerDesc{"test1", []portDesc{{100, "tcp"}}},
					containerDesc{"test2", []portDesc{{200, "udp"}}},
				),
			},
			expectedServices: Objects{
				expectedService("multicontainer", portDesc{100, "tcp"}, portDesc{200, "udp"}),
			},
		},
		{
			name: "duplicate ports",
			input: Objects{
				fakeDeploymentConfig("dupports",
					containerDesc{"test1", []portDesc{{80, "tcp"}, {25, "tcp"}}},
					containerDesc{"test2", []portDesc{{80, "tcp"}}},
				),
			},
			expectedServices: Objects{
				expectedService("dupports", portDesc{25, "tcp"}, portDesc{80, "tcp"}),
			},
		},
		{
			name: "multiple deployment configs",
			input: Objects{
				fakeDeploymentConfig("multidc1",
					containerDesc{"test1", []portDesc{{100, "tcp"}, {200, "udp"}}},
					containerDesc{"test2", []portDesc{{300, "tcp"}}},
				),
				fakeDeploymentConfig("multidc2",
					containerDesc{"dc2_test1", []portDesc{{300, "tcp"}}},
					containerDesc{"dc2_test2", []portDesc{{200, "udp"}, {300, "tcp"}}},
				),
			},
			expectedServices: Objects{
				expectedService("multidc1", portDesc{100, "tcp"}, portDesc{200, "udp"}, portDesc{300, "tcp"}),
				expectedService("multidc2", portDesc{300, "tcp"}, portDesc{200, "udp"}),
			},
		},
		{
			name: "first only",
			input: Objects{
				fakeDeploymentConfig("firstonly",
					containerDesc{"test1", []portDesc{{80, "tcp"}, {25, "tcp"}, {100, "udp"}}},
				),
			},
			expectedServices: Objects{
				expectedService("firstonly", portDesc{25, "tcp"}),
			},
			firstOnly: true,
		},
	}

	for _, test := range tests {
		output := AddServices(test.input, test.firstOnly)
		services := getServices(output)
		if !reflect.DeepEqual(services, test.expectedServices) {
			t.Errorf("%s: did not get expected output.\nExpected:\n%s.\nGot:\n%s.",
				test.name, objsToString(test.expectedServices), objsToString(services))
		}
	}
}

func TestNewBuildPipeline(t *testing.T) {
	// If we cannot infer a name from user input, NewBuildPipeline should return
	// ErrNameRequired.
	_, err := NewBuildPipeline("test", nil, false, nil, nil, nil)
	if err != ErrNameRequired {
		t.Errorf("err = %#v; want %#v", err, ErrNameRequired)
	}
}
