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

package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/test/e2e/feature"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epodoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/nodefeature"
	imageutils "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var (
	// non-root UID used in tests.
	nonRootTestUserID = int64(1000)
)

var _ = SIGDescribe("Security Context", func() {
	f := framework.NewDefaultFramework("security-context-test")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	var podClient *e2epod.PodClient
	ginkgo.BeforeEach(func() {
		podClient = e2epod.NewPodClient(f)
	})

	ginkgo.Context("When creating a pod with HostUsers", func() {
		containerName := "userns-test"
		makePod := func(hostUsers bool) *v1.Pod {
			return &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "userns-" + string(uuid.NewUUID()),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    containerName,
							Image:   imageutils.GetE2EImage(imageutils.BusyBox),
							Command: []string{"cat", "/proc/self/uid_map"},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
					HostUsers:     &hostUsers,
				},
			}
		}

		f.It("must create the user namespace if set to false [LinuxOnly]", feature.UserNamespacesSupport, func(ctx context.Context) {
			// with hostUsers=false the pod must use a new user namespace
			podClient := e2epod.PodClientNS(f, f.Namespace.Name)

			createdPod1 := podClient.Create(ctx, makePod(false))
			createdPod2 := podClient.Create(ctx, makePod(false))
			ginkgo.DeferCleanup(func(ctx context.Context) {
				ginkgo.By("delete the pods")
				podClient.DeleteSync(ctx, createdPod1.Name, metav1.DeleteOptions{}, e2epod.DefaultPodDeletionTimeout)
				podClient.DeleteSync(ctx, createdPod2.Name, metav1.DeleteOptions{}, e2epod.DefaultPodDeletionTimeout)
			})
			getLogs := func(pod *v1.Pod) (string, error) {
				err := e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, f.ClientSet, createdPod1.Name, f.Namespace.Name, f.Timeouts.PodStart)
				if err != nil {
					return "", err
				}
				podStatus, err := podClient.Get(ctx, pod.Name, metav1.GetOptions{})
				if err != nil {
					return "", err
				}
				return e2epod.GetPodLogs(ctx, f.ClientSet, f.Namespace.Name, podStatus.Name, containerName)
			}

			logs1, err := getLogs(createdPod1)
			framework.ExpectNoError(err)
			logs2, err := getLogs(createdPod2)
			framework.ExpectNoError(err)

			// 65536 is the size used for a user namespace.  Verify that the value is present
			// in the /proc/self/uid_map file.
			if !strings.Contains(logs1, "65536") || !strings.Contains(logs2, "65536") {
				framework.Failf("user namespace not created")
			}
			if logs1 == logs2 {
				framework.Failf("two different pods are running with the same user namespace configuration")
			}
		})

		f.It("must not create the user namespace if set to true [LinuxOnly]", feature.UserNamespacesSupport, func(ctx context.Context) {
			// with hostUsers=true the pod must use the host user namespace
			pod := makePod(true)
			// When running in the host's user namespace, the /proc/self/uid_map file content looks like:
			// 0          0 4294967295
			// Verify the value 4294967295 is present in the output.
			e2epodoutput.TestContainerOutput(ctx, f, "read namespace", pod, 0, []string{
				"4294967295",
			})
		})

		f.It("should mount all volumes with proper permissions with hostUsers=false [LinuxOnly]", feature.UserNamespacesSupport, func(ctx context.Context) {
			// Create all volume types supported: configmap, secret, downwardAPI, projected.

			// Create configmap.
			name := "userns-volumes-test-" + string(uuid.NewUUID())
			configMap := newConfigMap(f, name)
			ginkgo.By(fmt.Sprintf("Creating configMap %v/%v", f.Namespace.Name, configMap.Name))
			var err error
			if configMap, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
				framework.Failf("unable to create test configMap %s: %v", configMap.Name, err)
			}

			// Create secret.
			secret := secretForTest(f.Namespace.Name, name)
			ginkgo.By(fmt.Sprintf("Creating secret with name %s", secret.Name))
			if secret, err = f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
				framework.Failf("unable to create test secret %s: %v", secret.Name, err)
			}

			// downwardAPI definition.
			downwardVolSource := &v1.DownwardAPIVolumeSource{
				Items: []v1.DownwardAPIVolumeFile{
					{
						Path: "name",
						FieldRef: &v1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.name",
						},
					},
				},
			}

			// Create a pod with all the volumes
			falseVar := false
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-userns-volumes-" + string(uuid.NewUUID()),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "userns-file-permissions",
							Image: imageutils.GetE2EImage(imageutils.BusyBox),
							// Print (numeric) GID of the files in /vol/.
							// We limit to "type f" as kubelet uses symlinks to those files, but we
							// don't care about the owner of the symlink itself, just the files.
							Command: []string{"sh", "-c", "stat -c='%g' $(find /vol/ -type f)"},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "cfg",
									MountPath: "/vol/cfg/",
								},
								{
									Name:      "secret",
									MountPath: "/vol/secret/",
								},
								{
									Name:      "downward",
									MountPath: "/vol/downward/",
								},
								{
									Name:      "projected",
									MountPath: "/vol/projected/",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "cfg",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: configMap.Name},
								},
							},
						},
						{
							Name: "secret",
							VolumeSource: v1.VolumeSource{
								Secret: &v1.SecretVolumeSource{
									SecretName: secret.Name,
								},
							},
						},
						{
							Name: "downward",
							VolumeSource: v1.VolumeSource{
								DownwardAPI: downwardVolSource,
							},
						},
						{
							Name: "projected",
							VolumeSource: v1.VolumeSource{
								Projected: &v1.ProjectedVolumeSource{
									Sources: []v1.VolumeProjection{
										{
											DownwardAPI: &v1.DownwardAPIProjection{
												Items: downwardVolSource.Items,
											},
										},
										{
											Secret: &v1.SecretProjection{
												LocalObjectReference: v1.LocalObjectReference{Name: secret.Name},
											},
										},
									},
								},
							},
						},
					},
					HostUsers:     &falseVar,
					RestartPolicy: v1.RestartPolicyNever,
				},
			}

			// Expect one line for each file on all the volumes.
			// Each line should be "=0" that means root inside the container is the owner of the file.
			downwardAPIVolFiles := 1
			projectedFiles := len(secret.Data) + downwardAPIVolFiles
			e2epodoutput.TestContainerOutput(ctx, f, "check file permissions", pod, 0, []string{
				strings.Repeat("=0\n", len(secret.Data)+len(configMap.Data)+downwardAPIVolFiles+projectedFiles),
			})
		})

		f.It("should set FSGroup to user inside the container with hostUsers=false [LinuxOnly]", feature.UserNamespacesSupport, func(ctx context.Context) {
			// Create configmap.
			name := "userns-volumes-test-" + string(uuid.NewUUID())
			configMap := newConfigMap(f, name)
			ginkgo.By(fmt.Sprintf("Creating configMap %v/%v", f.Namespace.Name, configMap.Name))
			var err error
			if configMap, err = f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
				framework.Failf("unable to create test configMap %s: %v", configMap.Name, err)
			}

			// Create a pod with hostUsers=false
			falseVar := false
			fsGroup := int64(200)
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-userns-fsgroup-" + string(uuid.NewUUID()),
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "userns-fsgroup",
							Image: imageutils.GetE2EImage(imageutils.BusyBox),
							// Print (numeric) GID of the files in /vol/.
							// We limit to "type f" as kubelet uses symlinks to those files, but we
							// don't care about the owner of the symlink itself, just the files.
							Command: []string{"sh", "-c", "stat -c='%g' $(find /vol/ -type f)"},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "cfg",
									MountPath: "/vol/cfg/",
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "cfg",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: configMap.Name},
								},
							},
						},
					},
					HostUsers:     &falseVar,
					RestartPolicy: v1.RestartPolicyNever,
					SecurityContext: &v1.PodSecurityContext{
						FSGroup: &fsGroup,
					},
				},
			}

			// Expect one line for each file on all the volumes.
			// Each line should be "=200" (fsGroup) that means it was mapped to the
			// right user inside the container.
			e2epodoutput.TestContainerOutput(ctx, f, "check FSGroup is mapped correctly", pod, 0, []string{
				strings.Repeat(fmt.Sprintf("=%v\n", fsGroup), len(configMap.Data)),
			})
		})
	})

	ginkgo.Context("When creating a container with runAsUser", func() {
		makeUserPod := func(podName, image string, command []string, userid int64) *v1.Pod {
			return &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: podName,
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Image:   image,
							Name:    podName,
							Command: command,
							SecurityContext: &v1.SecurityContext{
								RunAsUser: &userid,
							},
						},
					},
				},
			}
		}
		createAndWaitUserPod := func(ctx context.Context, userid int64) {
			podName := fmt.Sprintf("busybox-user-%d-%s", userid, uuid.NewUUID())
			podClient.Create(ctx, makeUserPod(podName,
				imageutils.GetE2EImage(imageutils.BusyBox),
				[]string{"sh", "-c", fmt.Sprintf("test $(id -u) -eq %d", userid)},
				userid,
			))

			podClient.WaitForSuccess(ctx, podName, framework.PodStartTimeout)
		}

		/*
			Release: v1.15
			Testname: Security Context, runAsUser=65534
			Description: Container is created with runAsUser option by passing uid 65534 to run as unpriviledged user. Pod MUST be in Succeeded phase.
			[LinuxOnly]: This test is marked as LinuxOnly since Windows does not support running as UID / GID.
		*/
		framework.ConformanceIt("should run the container with uid 65534 [LinuxOnly]", f.WithNodeConformance(), func(ctx context.Context) {
			createAndWaitUserPod(ctx, 65534)
		})

		/*
			Release: v1.15
			Testname: Security Context, runAsUser=0
			Description: Container is created with runAsUser option by passing uid 0 to run as root privileged user. Pod MUST be in Succeeded phase.
			This e2e can not be promoted to Conformance because a Conformant platform may not allow to run containers with 'uid 0' or running privileged operations.
			[LinuxOnly]: This test is marked as LinuxOnly since Windows does not support running as UID / GID.
		*/
		f.It("should run the container with uid 0 [LinuxOnly]", f.WithNodeConformance(), func(ctx context.Context) {
			createAndWaitUserPod(ctx, 0)
		})
	})

	ginkgo.Context("When creating a container with runAsNonRoot", func() {
		rootImage := imageutils.GetE2EImage(imageutils.BusyBox)
		nonRootImage := imageutils.GetE2EImage(imageutils.NonRoot)
		makeNonRootPod := func(podName, image string, userid *int64) *v1.Pod {
			return &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: podName,
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Image:   image,
							Name:    podName,
							Command: []string{"id", "-u"}, // Print UID and exit
							SecurityContext: &v1.SecurityContext{
								RunAsNonRoot: pointer.BoolPtr(true),
								RunAsUser:    userid,
							},
						},
					},
				},
			}
		}

		ginkgo.It("should run with an explicit non-root user ID [LinuxOnly]", func(ctx context.Context) {
			// creates a pod with RunAsUser, which is not supported on Windows.
			e2eskipper.SkipIfNodeOSDistroIs("windows")
			name := "explicit-nonroot-uid"
			pod := makeNonRootPod(name, rootImage, pointer.Int64Ptr(nonRootTestUserID))
			podClient.Create(ctx, pod)

			podClient.WaitForSuccess(ctx, name, framework.PodStartTimeout)
			framework.ExpectNoError(podClient.MatchContainerOutput(ctx, name, name, "1000"))
		})
		ginkgo.It("should not run with an explicit root user ID [LinuxOnly]", func(ctx context.Context) {
			// creates a pod with RunAsUser, which is not supported on Windows.
			e2eskipper.SkipIfNodeOSDistroIs("windows")
			name := "explicit-root-uid"
			pod := makeNonRootPod(name, nonRootImage, pointer.Int64Ptr(0))
			pod = podClient.Create(ctx, pod)

			ev, err := podClient.WaitForErrorEventOrSuccess(ctx, pod)
			framework.ExpectNoError(err)
			gomega.Expect(ev).NotTo(gomega.BeNil())
			gomega.Expect(ev.Reason).To(gomega.Equal(events.FailedToCreateContainer))
		})
		ginkgo.It("should run with an image specified user ID", func(ctx context.Context) {
			name := "implicit-nonroot-uid"
			pod := makeNonRootPod(name, nonRootImage, nil)
			podClient.Create(ctx, pod)

			podClient.WaitForSuccess(ctx, name, framework.PodStartTimeout)
			framework.ExpectNoError(podClient.MatchContainerOutput(ctx, name, name, "1234"))
		})
		ginkgo.It("should not run without a specified user ID", func(ctx context.Context) {
			name := "implicit-root-uid"
			pod := makeNonRootPod(name, rootImage, nil)
			pod = podClient.Create(ctx, pod)

			ev, err := podClient.WaitForErrorEventOrSuccess(ctx, pod)
			framework.ExpectNoError(err)
			gomega.Expect(ev).NotTo(gomega.BeNil())
			gomega.Expect(ev.Reason).To(gomega.Equal(events.FailedToCreateContainer))
		})
	})

	ginkgo.Context("When creating a pod with readOnlyRootFilesystem", func() {
		makeUserPod := func(podName, image string, command []string, readOnlyRootFilesystem bool) *v1.Pod {
			return &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: podName,
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Image:   image,
							Name:    podName,
							Command: command,
							SecurityContext: &v1.SecurityContext{
								ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
							},
						},
					},
				},
			}
		}
		createAndWaitUserPod := func(ctx context.Context, readOnlyRootFilesystem bool) string {
			podName := fmt.Sprintf("busybox-readonly-%v-%s", readOnlyRootFilesystem, uuid.NewUUID())
			podClient.Create(ctx, makeUserPod(podName,
				imageutils.GetE2EImage(imageutils.BusyBox),
				[]string{"sh", "-c", "touch checkfile"},
				readOnlyRootFilesystem,
			))

			if readOnlyRootFilesystem {
				waitForFailure(ctx, f, podName, framework.PodStartTimeout)
			} else {
				podClient.WaitForSuccess(ctx, podName, framework.PodStartTimeout)
			}

			return podName
		}

		/*
			Release: v1.15
			Testname: Security Context, readOnlyRootFilesystem=true.
			Description: Container is configured to run with readOnlyRootFilesystem to true which will force containers to run with a read only root file system.
			Write operation MUST NOT be allowed and Pod MUST be in Failed state.
			At this moment we are not considering this test for Conformance due to use of SecurityContext.
			[LinuxOnly]: This test is marked as LinuxOnly since Windows does not support creating containers with read-only access.
		*/
		f.It("should run the container with readonly rootfs when readOnlyRootFilesystem=true [LinuxOnly]", f.WithNodeConformance(), func(ctx context.Context) {
			createAndWaitUserPod(ctx, true)
		})

		/*
			Release: v1.15
			Testname: Security Context, readOnlyRootFilesystem=false.
			Description: Container is configured to run with readOnlyRootFilesystem to false.
			Write operation MUST be allowed and Pod MUST be in Succeeded state.
		*/
		framework.ConformanceIt("should run the container with writable rootfs when readOnlyRootFilesystem=false", f.WithNodeConformance(), func(ctx context.Context) {
			createAndWaitUserPod(ctx, false)
		})
	})

	ginkgo.Context("When creating a pod with privileged", func() {
		makeUserPod := func(podName, image string, command []string, privileged bool) *v1.Pod {
			return &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: podName,
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Image:   image,
							Name:    podName,
							Command: command,
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
				},
			}
		}
		createAndWaitUserPod := func(ctx context.Context, privileged bool) string {
			podName := fmt.Sprintf("busybox-privileged-%v-%s", privileged, uuid.NewUUID())
			podClient.Create(ctx, makeUserPod(podName,
				imageutils.GetE2EImage(imageutils.BusyBox),
				[]string{"sh", "-c", "ip link add dummy0 type dummy || true"},
				privileged,
			))
			podClient.WaitForSuccess(ctx, podName, framework.PodStartTimeout)
			return podName
		}
		/*
			Release: v1.15
			Testname: Security Context, privileged=false.
			Description: Create a container to run in unprivileged mode by setting pod's SecurityContext Privileged option as false. Pod MUST be in Succeeded phase.
			[LinuxOnly]: This test is marked as LinuxOnly since it runs a Linux-specific command.
		*/
		framework.ConformanceIt("should run the container as unprivileged when false [LinuxOnly]", f.WithNodeConformance(), func(ctx context.Context) {
			podName := createAndWaitUserPod(ctx, false)
			logs, err := e2epod.GetPodLogs(ctx, f.ClientSet, f.Namespace.Name, podName, podName)
			if err != nil {
				framework.Failf("GetPodLogs for pod %q failed: %v", podName, err)
			}

			framework.Logf("Got logs for pod %q: %q", podName, logs)
			if !strings.Contains(logs, "Operation not permitted") {
				framework.Failf("unprivileged container shouldn't be able to create dummy device")
			}
		})

		f.It("should run the container as privileged when true [LinuxOnly]", nodefeature.HostAccess, func(ctx context.Context) {
			podName := createAndWaitUserPod(ctx, true)
			logs, err := e2epod.GetPodLogs(ctx, f.ClientSet, f.Namespace.Name, podName, podName)
			if err != nil {
				framework.Failf("GetPodLogs for pod %q failed: %v", podName, err)
			}

			framework.Logf("Got logs for pod %q: %q", podName, logs)
			if strings.Contains(logs, "Operation not permitted") {
				framework.Failf("privileged container should be able to create dummy device")
			}
		})
	})

	ginkgo.Context("when creating containers with AllowPrivilegeEscalation", func() {
		makeAllowPrivilegeEscalationPod := func(podName string, allowPrivilegeEscalation *bool, uid int64) *v1.Pod {
			return &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: podName,
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Image: imageutils.GetE2EImage(imageutils.Nonewprivs),
							Name:  podName,
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: allowPrivilegeEscalation,
								RunAsUser:                &uid,
							},
						},
					},
				},
			}
		}
		createAndMatchOutput := func(ctx context.Context, podName, output string, allowPrivilegeEscalation *bool, uid int64) error {
			podClient.Create(ctx, makeAllowPrivilegeEscalationPod(podName,
				allowPrivilegeEscalation,
				uid,
			))
			podClient.WaitForSuccess(ctx, podName, framework.PodStartTimeout)
			return podClient.MatchContainerOutput(ctx, podName, podName, output)
		}

		/*
			Release: v1.15
			Testname: Security Context, allowPrivilegeEscalation unset, uid != 0.
			Description: Configuring the allowPrivilegeEscalation unset, allows the privilege escalation operation.
			A container is configured with allowPrivilegeEscalation not specified (nil) and a given uid which is not 0.
			When the container is run, container's output MUST match with expected output verifying container ran with uid=0.
			This e2e Can not be promoted to Conformance as it is Container Runtime dependent and not all conformant platforms will require this behavior.
			[LinuxOnly]: This test is marked LinuxOnly since Windows does not support running as UID / GID, or privilege escalation.
		*/
		f.It("should allow privilege escalation when not explicitly set and uid != 0 [LinuxOnly]", f.WithNodeConformance(), func(ctx context.Context) {
			podName := "alpine-nnp-nil-" + string(uuid.NewUUID())
			if err := createAndMatchOutput(ctx, podName, "Effective uid: 0", nil, nonRootTestUserID); err != nil {
				framework.Failf("Match output for pod %q failed: %v", podName, err)
			}
		})

		/*
			Release: v1.15
			Testname: Security Context, allowPrivilegeEscalation=false.
			Description: Configuring the allowPrivilegeEscalation to false, does not allow the privilege escalation operation.
			A container is configured with allowPrivilegeEscalation=false and a given uid (1000) which is not 0.
			When the container is run, container's output MUST match with expected output verifying container ran with given uid i.e. uid=1000.
			[LinuxOnly]: This test is marked LinuxOnly since Windows does not support running as UID / GID, or privilege escalation.
		*/
		framework.ConformanceIt("should not allow privilege escalation when false [LinuxOnly]", f.WithNodeConformance(), func(ctx context.Context) {
			podName := "alpine-nnp-false-" + string(uuid.NewUUID())
			apeFalse := false
			if err := createAndMatchOutput(ctx, podName, fmt.Sprintf("Effective uid: %d", nonRootTestUserID), &apeFalse, nonRootTestUserID); err != nil {
				framework.Failf("Match output for pod %q failed: %v", podName, err)
			}
		})

		/*
			Release: v1.15
			Testname: Security Context, allowPrivilegeEscalation=true.
			Description: Configuring the allowPrivilegeEscalation to true, allows the privilege escalation operation.
			A container is configured with allowPrivilegeEscalation=true and a given uid (1000) which is not 0.
			When the container is run, container's output MUST match with expected output verifying container ran with uid=0 (making use of the privilege escalation).
			This e2e Can not be promoted to Conformance as it is Container Runtime dependent and runtime may not allow to run.
			[LinuxOnly]: This test is marked LinuxOnly since Windows does not support running as UID / GID.
		*/
		f.It("should allow privilege escalation when true [LinuxOnly]", f.WithNodeConformance(), func(ctx context.Context) {
			podName := "alpine-nnp-true-" + string(uuid.NewUUID())
			apeTrue := true
			if err := createAndMatchOutput(ctx, podName, "Effective uid: 0", &apeTrue, nonRootTestUserID); err != nil {
				framework.Failf("Match output for pod %q failed: %v", podName, err)
			}
		})
	})
})

var _ = SIGDescribe("User Namespaces for Pod Security Standards [LinuxOnly]", func() {
	f := framework.NewDefaultFramework("user-namespaces-pss-test")
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	ginkgo.Context("with UserNamespacesSupport and UserNamespacesPodSecurityStandards enabled", func() {
		f.It("should allow pod", feature.UserNamespacesPodSecurityStandards, func(ctx context.Context) {
			name := "pod-user-namespaces-pss-" + string(uuid.NewUUID())
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: v1.PodSpec{
					RestartPolicy:   v1.RestartPolicyNever,
					HostUsers:       ptr.To(false),
					SecurityContext: &v1.PodSecurityContext{},
					Containers: []v1.Container{
						{
							Name:    name,
							Image:   imageutils.GetE2EImage(imageutils.BusyBox),
							Command: []string{"whoami"},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities:             &v1.Capabilities{Drop: []v1.Capability{"ALL"}},
								SeccompProfile:           &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
							},
						},
					},
				},
			}

			e2epodoutput.TestContainerOutput(ctx, f, "RunAsUser-RunAsNonRoot", pod, 0, []string{"root"})
		})
	})
})

// waitForFailure waits for pod to fail.
func waitForFailure(ctx context.Context, f *framework.Framework, name string, timeout time.Duration) {
	gomega.Expect(e2epod.WaitForPodCondition(ctx, f.ClientSet, f.Namespace.Name, name, fmt.Sprintf("%s or %s", v1.PodSucceeded, v1.PodFailed), timeout,
		func(pod *v1.Pod) (bool, error) {
			switch pod.Status.Phase {
			case v1.PodFailed:
				return true, nil
			case v1.PodSucceeded:
				return true, fmt.Errorf("pod %q succeeded with reason: %q, message: %q", name, pod.Status.Reason, pod.Status.Message)
			default:
				return false, nil
			}
		},
	)).To(gomega.Succeed(), "wait for pod %q to fail", name)
}
