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
	"k8s.io/kubernetes/pkg/util"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("[Skipped] Security Context", func() {
	framework := NewFramework("security-context")

	It("should support pod.Spec.SecurityContext.RunAsUser", func() {
		podName := "security-context-" + string(util.NewUUID())
		pod := &api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name:   podName,
				Labels: map[string]string{"name": podName},
			},
			Spec: api.PodSpec{
				SecurityContext: &api.PodSecurityContext{},
				Containers: []api.Container{
					{
						Name:    "test-container",
						Image:   "gcr.io/google_containers/busybox",
						Command: []string{"sh", "-c", "id -u"},
					},
				},
				RestartPolicy: api.RestartPolicyNever,
			},
		}

		var uid int64 = 1001
		pod.Spec.SecurityContext.RunAsUser = &uid

		framework.TestContainerOutput("pod.Spec.SecurityContext.RunAsUser", pod, 0, []string{
			fmt.Sprintf("%v", uid),
		})
	})

	It("should support container.SecurityContext.RunAsUser", func() {
		podName := "security-context-" + string(util.NewUUID())
		pod := &api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name:   podName,
				Labels: map[string]string{"name": podName},
			},
			Spec: api.PodSpec{
				SecurityContext: &api.PodSecurityContext{},
				Containers: []api.Container{
					{
						Name:            "test-container",
						Image:           "gcr.io/google_containers/busybox",
						Command:         []string{"sh", "-c", "id -u"},
						SecurityContext: &api.SecurityContext{},
					},
				},
				RestartPolicy: api.RestartPolicyNever,
			},
		}

		var uid int64 = 1001
		var overrideUid int64 = 1002
		pod.Spec.SecurityContext.RunAsUser = &uid
		pod.Spec.Containers[0].SecurityContext.RunAsUser = &overrideUid

		framework.TestContainerOutput("pod.Spec.SecurityContext.RunAsUser", pod, 0, []string{
			fmt.Sprintf("%v", overrideUid),
		})
	})
})
