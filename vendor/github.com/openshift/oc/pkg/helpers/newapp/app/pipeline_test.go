package app

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "github.com/openshift/api/apps/v1"
	imagev1 "github.com/openshift/api/image/v1"
)

type portDesc struct {
	port     int
	protocol string
}

type containerDesc struct {
	name  string
	ports []portDesc
}

func fakeDeploymentConfig(name string, containers ...containerDesc) *appsv1.DeploymentConfig {
	specContainers := []corev1.Container{}
	for _, c := range containers {
		container := corev1.Container{
			Name: c.name,
		}

		container.Ports = []corev1.ContainerPort{}
		for _, p := range c.ports {
			container.Ports = append(container.Ports, corev1.ContainerPort{
				Name:          fmt.Sprintf("port-%d-%s", p.port, p.protocol),
				ContainerPort: int32(p.port),
				Protocol:      corev1.Protocol(p.protocol),
			})
		}

		specContainers = append(specContainers, container)
	}
	return &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.DeploymentConfigSpec{
			Replicas: 1,
			Selector: map[string]string{"name": "test"},
			Template: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: specContainers,
				},
			},
		},
	}
}

func expectedService(name string, ports ...portDesc) *corev1.Service {
	servicePorts := []corev1.ServicePort{}
	for _, p := range ports {
		servicePorts = append(servicePorts, corev1.ServicePort{
			// Name is derived purely from the port and protocol, ignoring the container port name
			Name:       fmt.Sprintf("%d-%s", p.port, p.protocol),
			Port:       int32(p.port),
			Protocol:   corev1.Protocol(p.protocol),
			TargetPort: intstr.FromInt(p.port),
		})
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"name": "test"},
			Ports:    servicePorts,
		},
	}
}

func getServices(objects Objects) Objects {
	result := Objects{}
	for _, obj := range objects {
		if _, isSvc := obj.(*corev1.Service); isSvc {
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
	is := func(name, ns string) *imagev1.ImageStream {
		obj := &imagev1.ImageStream{}
		obj.Name = name
		obj.Namespace = ns
		return obj
	}
	dc := func(name, ns string) *appsv1.DeploymentConfig {
		obj := &appsv1.DeploymentConfig{}
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
		au := NewAcceptUnique()
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
				expectedService("multidc2", portDesc{200, "udp"}, portDesc{300, "tcp"}),
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
			t.Errorf("%s: did not get expected output: %s",
				test.name, diff.ObjectGoPrintDiff(test.expectedServices, services))
		}
	}
}
