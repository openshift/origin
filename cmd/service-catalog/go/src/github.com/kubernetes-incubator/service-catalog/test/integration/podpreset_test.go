/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/features"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	settingsapi "github.com/kubernetes-incubator/service-catalog/pkg/apis/settings/v1alpha1"
	// our versioned client
	servicecatalogclient "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"

	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// TestPodPresetClient exercises the PodPreset client.
func TestPodPresetClient(t *testing.T) {
	// enable PodPreset APIs since they aren't enabled by default
	enablePodPresetFeature()
	defer disablePodPresetFeature()
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			const name = "test-podpreset"

			client, _, shutdown := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &settingsapi.PodPreset{}
			})
			defer shutdown()

			if err := testPodPresetClient(sType, client, name); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, sType := range []server.StorageType{server.StorageTypeEtcd} {
		if !t.Run(sType.String(), rootTestFunc(sType)) {
			t.Errorf("%s test failed", sType)
		}
	}
}

func enablePodPresetFeature() {
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", features.PodPreset))
}

func disablePodPresetFeature() {
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", features.PodPreset))
}

func testPodPresetClient(sType server.StorageType, client servicecatalogclient.Interface, name string) error {
	testNamespace := "test-namespace"
	podPresetName := "test-podpreset"

	cl := client.Settings().PodPresets(testNamespace)

	// table driven tests for podpresets so that we can expand the input dataset
	tests := []struct{ input *settingsapi.PodPreset }{
		{
			input: &settingsapi.PodPreset{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podPresetName,
					Namespace: testNamespace,
				},
				Spec: settingsapi.PodPresetSpec{
					Selector: metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "security",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"S2"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name:         "vol",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "vol", MountPath: "/foo"},
					},
					Env: []corev1.EnvVar{{Name: "abc", Value: "value"}, {Name: "ABC", Value: "value"}},
				},
			},
		},
	}

	for _, test := range tests {
		podpresets, err := cl.List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error listing podpresets: %v", err)
		}
		if n := len(podpresets.Items); n > 0 {
			return fmt.Errorf("podpresets should not exist on start, found %v podpresets", n)
		}

		in := test.input

		out, err := cl.Create(in)
		if err != nil {
			return fmt.Errorf("error creating podpreset :%v", err)
		}

		if in.Name != out.Name {
			return fmt.Errorf("name doesn't match: %v", err)
		}

		podpresets, err = cl.List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error listing podpreset :%v", err)
		}

		if n := len(podpresets.Items); n != 1 {
			return fmt.Errorf("expected list size to be 1 and got: %d", n)
		}

		got, err := cl.Get(podPresetName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error listing podpreset :%v", err)
		}

		if !reflect.DeepEqual(got, out) {
			return fmt.Errorf("objects do not match")
		}

		err = cl.Delete(podPresetName, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting podpreset : %v", err)
		}

		podpresets, err = cl.List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error listing podpreset : %v", err)
		}

		if n := len(podpresets.Items); n != 0 {
			return fmt.Errorf("expected no podpresets, but found: %d", n)
		}
	}

	return nil
}
