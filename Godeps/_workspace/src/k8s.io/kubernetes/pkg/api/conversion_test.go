/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package api_test

import (
	"io/ioutil"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/controller"
	"k8s.io/kubernetes/pkg/registry/daemonset"
	"k8s.io/kubernetes/pkg/registry/deployment"
	"k8s.io/kubernetes/pkg/registry/horizontalpodautoscaler"
	"k8s.io/kubernetes/pkg/registry/ingress"
	"k8s.io/kubernetes/pkg/registry/job"
	"k8s.io/kubernetes/pkg/registry/limitrange"
	"k8s.io/kubernetes/pkg/registry/namespace"
	"k8s.io/kubernetes/pkg/registry/node"
	"k8s.io/kubernetes/pkg/registry/persistentvolume"
	"k8s.io/kubernetes/pkg/registry/persistentvolumeclaim"
	"k8s.io/kubernetes/pkg/registry/pod"
	"k8s.io/kubernetes/pkg/registry/podtemplate"
	"k8s.io/kubernetes/pkg/registry/resourcequota"
	"k8s.io/kubernetes/pkg/registry/secret"
	"k8s.io/kubernetes/pkg/registry/serviceaccount"
	"k8s.io/kubernetes/pkg/registry/thirdpartyresource"
	"k8s.io/kubernetes/pkg/registry/thirdpartyresourcedata"
)

func BenchmarkPodConversion(b *testing.B) {
	data, err := ioutil.ReadFile("pod_example.json")
	if err != nil {
		b.Fatalf("Unexpected error while reading file: %v", err)
	}
	var pod api.Pod
	if err := api.Scheme.DecodeInto(data, &pod); err != nil {
		b.Fatalf("Unexpected error decoding pod: %v", err)
	}

	scheme := api.Scheme.Raw()
	var result *api.Pod
	for i := 0; i < b.N; i++ {
		versionedObj, err := scheme.ConvertToVersion(&pod, testapi.Default.Version())
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		obj, err := scheme.ConvertToVersion(versionedObj, scheme.InternalVersion)
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		result = obj.(*api.Pod)
	}
	if !api.Semantic.DeepDerivative(pod, *result) {
		b.Fatalf("Incorrect conversion: expected %v, got %v", pod, *result)
	}
}

func BenchmarkNodeConversion(b *testing.B) {
	data, err := ioutil.ReadFile("node_example.json")
	if err != nil {
		b.Fatalf("Unexpected error while reading file: %v", err)
	}
	var node api.Node
	if err := api.Scheme.DecodeInto(data, &node); err != nil {
		b.Fatalf("Unexpected error decoding node: %v", err)
	}

	scheme := api.Scheme.Raw()
	var result *api.Node
	for i := 0; i < b.N; i++ {
		versionedObj, err := scheme.ConvertToVersion(&node, testapi.Default.Version())
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		obj, err := scheme.ConvertToVersion(versionedObj, scheme.InternalVersion)
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		result = obj.(*api.Node)
	}
	if !api.Semantic.DeepDerivative(node, *result) {
		b.Fatalf("Incorrect conversion: expected %v, got %v", node, *result)
	}
}

func BenchmarkReplicationControllerConversion(b *testing.B) {
	data, err := ioutil.ReadFile("replication_controller_example.json")
	if err != nil {
		b.Fatalf("Unexpected error while reading file: %v", err)
	}
	var replicationController api.ReplicationController
	if err := api.Scheme.DecodeInto(data, &replicationController); err != nil {
		b.Fatalf("Unexpected error decoding node: %v", err)
	}

	scheme := api.Scheme.Raw()
	var result *api.ReplicationController
	for i := 0; i < b.N; i++ {
		versionedObj, err := scheme.ConvertToVersion(&replicationController, testapi.Default.Version())
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		obj, err := scheme.ConvertToVersion(versionedObj, scheme.InternalVersion)
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		result = obj.(*api.ReplicationController)
	}
	if !api.Semantic.DeepDerivative(replicationController, *result) {
		b.Fatalf("Incorrect conversion: expected %v, got %v", replicationController, *result)
	}
}

// TestSelectableFieldLabelConversions verifies that each resource have field
// label conversion defined for each its selectable field.
func TestSelectableFieldLabelConversions(t *testing.T) {
	kindFields := []struct {
		apiVersion      string
		kind            string
		namespaceScoped bool
		fields          labels.Set
		// labelMap maps deprecated labels to their canonical names
		labelMap map[string]string
	}{
		{testapi.Default.Version(), "LimitRange", true, labels.Set(limitrange.LimitRangeToSelectableFields(&api.LimitRange{})), nil},
		{testapi.Default.Version(), "Namespace", false, namespace.NamespaceToSelectableFields(&api.Namespace{}), map[string]string{"name": "metadata.name"}},
		{testapi.Default.Version(), "Node", false, labels.Set(node.NodeToSelectableFields(&api.Node{})), nil},
		{testapi.Default.Version(), "PersistentVolume", false, persistentvolume.PersistentVolumeToSelectableFields(&api.PersistentVolume{}), map[string]string{"name": "metadata.name"}},
		{testapi.Default.Version(), "PersistentVolumeClaim", true, persistentvolumeclaim.PersistentVolumeClaimToSelectableFields(&api.PersistentVolumeClaim{}), map[string]string{"name": "metadata.name"}},
		{testapi.Default.Version(), "Pod", true, labels.Set(pod.PodToSelectableFields(&api.Pod{})), nil},
		{testapi.Default.Version(), "PodTemplate", true, labels.Set(podtemplate.PodTemplateToSelectableFields(&api.PodTemplate{})), nil},
		{testapi.Default.Version(), "ReplicationController", true, labels.Set(controller.ControllerToSelectableFields(&api.ReplicationController{})), nil},
		{testapi.Default.Version(), "ResourceQuota", true, resourcequota.ResourceQuotaToSelectableFields(&api.ResourceQuota{}), nil},
		{testapi.Default.Version(), "Secret", true, secret.SelectableFields(&api.Secret{}), nil},
		{testapi.Default.Version(), "ServiceAccount", true, serviceaccount.SelectableFields(&api.ServiceAccount{}), nil},

		{testapi.Extensions.Version(), "Autoscaler", true, labels.Set(horizontalpodautoscaler.AutoscalerToSelectableFields(&extensions.HorizontalPodAutoscaler{})), nil},
		{testapi.Extensions.Version(), "DaemonSet", true, labels.Set(daemonset.DaemonSetToSelectableFields(&extensions.DaemonSet{})), nil},
		{testapi.Extensions.Version(), "Deployment", true, labels.Set(deployment.DeploymentToSelectableFields(&extensions.Deployment{})), nil},
		{testapi.Extensions.Version(), "Ingress", true, labels.Set(ingress.IngressToSelectableFields(&extensions.Ingress{})), nil},
		{testapi.Extensions.Version(), "Job", true, labels.Set(job.JobToSelectableFields(&extensions.Job{})), nil},
		{testapi.Extensions.Version(), "ThirdPartyResource", true, thirdpartyresource.SelectableFields(&extensions.ThirdPartyResource{}), nil},
		{testapi.Extensions.Version(), "ThirdPartyResourceData", true, thirdpartyresourcedata.SelectableFields(&extensions.ThirdPartyResourceData{}), nil},
	}

	badFieldLabels := []string{
		".name",
		"bad",
		"metadata",
		"foo.bar",
	}

	value := "value"

	for _, kfs := range kindFields {
		if len(kfs.fields) == 0 {
			t.Logf("no selectable fields for kind %q, skipping", kfs.kind)
		}
		for label := range kfs.fields {
			if !kfs.namespaceScoped && label == "metadata.namespace" {
				// FIXME: SelectableFields() shouldn't return "metadata.namespace" for cluster scoped resources
				continue
			}
			newLabel, newValue, err := api.Scheme.ConvertFieldLabel(kfs.apiVersion, kfs.kind, label, value)
			if err != nil {
				t.Errorf("%s kind=%s label=%s: got unexpected error: %v", kfs.apiVersion, kfs.kind, label, err)
			} else {
				expectedLabel := label
				if l, exists := kfs.labelMap[label]; exists {
					expectedLabel = l
				}
				if newLabel != expectedLabel {
					t.Errorf("%s kind=%s label=%s: got unexpected label name (%q != %q)", kfs.apiVersion, kfs.kind, label, newLabel, expectedLabel)
				}
				if newValue != value {
					t.Errorf("%s kind=%s label=%s: got unexpected new value (%q != %q)", kfs.apiVersion, kfs.kind, label, newValue, value)
				}
			}
		}

		for _, label := range badFieldLabels {
			_, _, err := api.Scheme.ConvertFieldLabel(kfs.apiVersion, kfs.kind, label, "value")
			if err == nil {
				t.Errorf("%s kind=%s label=%s: got unexpected non-error", kfs.apiVersion, kfs.kind, label)
			}
		}
	}
}
