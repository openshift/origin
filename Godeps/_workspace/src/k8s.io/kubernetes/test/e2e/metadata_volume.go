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

package e2e

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metadata volume", func() {
	var c *client.Client
	var ns string

	BeforeEach(func() {
		var err error
		c, err = loadClient()
		Expect(err).NotTo(HaveOccurred())
		ns_, err := createTestingNS("metadata-volume", c)
		ns = ns_.Name
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Clean up the namespace if a non-default one was used
		if ns != api.NamespaceDefault {
			By("Cleaning up the namespace")
			err := c.Namespaces().Delete(ns)
			expectNoError(err)
		}
	})

	It("should provide labels and annotations files", func() {
		podName := "metadata-volume-" + string(util.NewUUID())
		pod := &api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name:        podName,
				Labels:      map[string]string{"cluster": "rack10"},
				Annotations: map[string]string{"builder": "john-doe"},
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					{
						Name:    "client-container",
						Image:   "gcr.io/google_containers/busybox",
						Command: []string{"sh", "-c", "cat /etc/labels /etc/annotations /etc/podname"},
						VolumeMounts: []api.VolumeMount{
							{
								Name:      "podinfo",
								MountPath: "/etc",
								ReadOnly:  false,
							},
						},
					},
				},
				Volumes: []api.Volume{
					{
						Name: "podinfo",
						VolumeSource: api.VolumeSource{
							Metadata: &api.MetadataVolumeSource{
								Items: []api.MetadataFile{
									{
										Name: "labels",
										FieldRef: api.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.labels",
										},
									},
									{
										Name: "annotations",
										FieldRef: api.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.annotations",
										},
									},
									{
										Name: "podname",
										FieldRef: api.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
							},
						},
					},
				},
				RestartPolicy: api.RestartPolicyNever,
			},
		}
		testContainerOutputInNamespace("metadata volume plugin", c, pod, 0, []string{
			fmt.Sprintf("cluster=rack10\n"),
			fmt.Sprintf("builder=john-doe\n"),
			fmt.Sprintf("%s\n", podName),
		}, ns)
	})
})
