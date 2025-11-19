package dra

import (
	"context"
	"fmt"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	helper "github.com/openshift/origin/test/extended/dra/helper"
	nvidia "github.com/openshift/origin/test/extended/dra/nvidia"
)

// two whole gpus, one we label as the producer, and the other consumer
// the producer pod sees only the 'producer' gpu
// the consumer pod sees both the 'producer' and the 'consumer' gpus
// the consumer pod is passed the UUID of the gpu which it will use
// by calling cudaSetDevice
// no need to use 'CUDA_VISIBLE_DEVICES' or 'NVIDIA_VISIBLE_DEVICES'
type shareDataWithCUDAIPC struct {
	f      *framework.Framework
	class  string
	node   *corev1.Node
	driver *nvidia.NvidiaDRADriverGPU
	// the UUID of the gpu on which the producer and the consumer should run
	producer, consumer string
}

func (spec shareDataWithCUDAIPC) Test(ctx context.Context, t testing.TB) {
	namespace := spec.f.Namespace.Name
	clientset := spec.f.ClientSet

	newRequest := func(name, gpuUUID string) resourceapi.DeviceRequest {
		return resourceapi.DeviceRequest{
			Name:            name,
			DeviceClassName: spec.class,
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: fmt.Sprintf("device.attributes['%s'].uuid == \"%s\"", spec.class, gpuUUID),
					},
				},
			},
		}
	}
	// a claim that allocates two whole gpus, named as 'producer' and 'consumer'
	// producer and consumer should match the uuids respectively
	claim := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "two-gpus",
		},
	}
	claim.Spec.Devices.Requests = []resourceapi.DeviceRequest{
		newRequest("producer", spec.producer),
		newRequest("consumer", spec.consumer),
	}

	// producer pod
	producer := helper.NewPod(namespace, "producer")
	producer.Spec.Containers = []corev1.Container{producerCtr()}
	// producer pod has access to the 'producer' gpu only
	producer.Spec.Containers[0].Resources.Claims = []corev1.ResourceClaim{{Name: "gpu", Request: "producer"}}
	producer.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:              "gpu",
			ResourceClaimName: ptr.To(claim.Name),
		},
	}
	shared(producer)

	// consumer pod
	consumer := helper.NewPod(namespace, "consumer")
	// nvidia device driver publishes a uuid of a gpu in this format 'GPU-{uuid}'
	// this is what we see in nvidia-smi, and ResourceSlices, when a CUDA
	// application enumerates the gpu devices using cudaGetDeviceProperties
	// it sees the uuid only.
	uuid, _ := strings.CutPrefix("GPU-", spec.consumer)
	consumer.Spec.Containers = []corev1.Container{consumerCtr(uuid)}
	// TODO: although the consumer pod runs its workload on the consumer gpu, it
	// needs access to the producer gpu for IPC, otherwsie we see the
	// following error from the consumer when it tries to open the
	// handle using cudaIpcOpenMemHandle
	//   ERROR opening IPC handle: invalid argument
	//
	// leaving Request empty ensures that the consumer has access
	// to all allocated devices in the claim.
	consumer.Spec.Containers[0].Resources.Claims = []corev1.ResourceClaim{{Name: "gpu", Request: ""}}
	consumer.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:              "gpu",
			ResourceClaimName: ptr.To(claim.Name),
		},
	}
	shared(consumer)

	t.Logf("producer uuid: %s consumer uuid: %s", spec.producer, spec.consumer)

	o.Expect(helper.EnsureNamespaceLabel(ctx, clientset, namespace, "pod-security.kubernetes.io/enforce", "privileged")).To(o.BeNil())

	g.By("creating external claim and pod(s)")
	t.Logf("creating resource claim: \n%s\n", framework.PrettyPrintJSON(claim))
	_, err := clientset.ResourceV1beta1().ResourceClaims(namespace).Create(ctx, claim, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	t.Logf("creating producer pod: \n%s\n", framework.PrettyPrintJSON(producer))
	producer, err = clientset.CoreV1().Pods(namespace).Create(ctx, producer, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	t.Logf("creating consumer pod: \n%s\n", framework.PrettyPrintJSON(consumer))
	consumer, err = clientset.CoreV1().Pods(namespace).Create(ctx, consumer, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.DeferCleanup(func(ctx context.Context) {
		g.By(fmt.Sprintf("listing resources in namespace: %s", namespace))
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		o.Expect(err).Should(o.BeNil())
		t.Logf("pods in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(pods))

		claims, err := clientset.ResourceV1beta1().ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
		o.Expect(err).Should(o.BeNil())
		t.Logf("resource claim in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(claims))
	})

	g.By(fmt.Sprintf("waiting for the producer pod %s/%s to be running", producer.Namespace, producer.Name))
	err = e2epodutil.WaitForPodRunningInNamespace(ctx, clientset, producer)
	o.Expect(err).To(o.BeNil())

	g.By(fmt.Sprintf("waiting for the consumer pod %s/%s to be running", consumer.Namespace, consumer.Name))
	err = e2epodutil.WaitForPodRunningInNamespace(ctx, clientset, consumer)
	o.Expect(err).To(o.BeNil())

	// verify that the producer and the consumer should see the gpu as expected
	for _, v := range []struct {
		pod  *corev1.Pod
		gpus []string
	}{
		{pod: producer, gpus: []string{spec.producer}},
		{pod: consumer, gpus: []string{spec.producer, spec.consumer}},
	} {
		ctr := v.pod.Spec.Containers[0]
		g.By(fmt.Sprintf("running nvidia-smi command into the container %s/%s container: %s", v.pod.Namespace, v.pod.Name, ctr.Name))
		gpus, err := nvidia.QueryGPUUsedByContainer(ctx, t, spec.f, v.pod.Name, v.pod.Namespace, ctr.Name)
		o.Expect(err).To(o.BeNil())
		o.Expect(gpus.UUIDs()).To(o.ConsistOf(v.gpus))
	}

	g.By("retrieving pod logs")
	for _, pod := range []*corev1.Pod{producer, consumer} {
		for _, ctr := range pod.Spec.Containers {
			logs, err := helper.GetLogs(ctx, clientset, pod.Namespace, pod.Name, ctr.Name)
			o.Expect(err).To(o.BeNil())
			t.Logf("logs from pod: %s, container: %s\n%s\n", pod.Name, ctr.Name, logs)
		}
	}
}

func shared(pod *corev1.Pod) {
	pod.Spec.HostIPC = true
	pod.Spec.HostPID = true
	pod.Spec.Tolerations = []corev1.Toleration{
		{
			Key:      "nvidia.com/gpu",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: "shared-volume",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/tmp/cuda-ipc-shared",
					Type: ptr.To(corev1.HostPathDirectoryOrCreate),
				},
			},
		},
	}

	ctr := &pod.Spec.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts, corev1.VolumeMount{Name: "shared-volume", MountPath: "/shared"})
	ctr.SecurityContext = &corev1.SecurityContext{
		Privileged: ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Add: []corev1.Capability{"IPC_LOCK"},
		},
	}

	// both pods should create the file '/tmp/ready' when they have successfully
	// handled CUDA IPC, and before they go into a permanent sleep.
	ctr.StartupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"cat", "/tmp/ready"},
			},
		},
		InitialDelaySeconds: 5,
		FailureThreshold:    12,
		PeriodSeconds:       10,
	}
}

func producerCtr() corev1.Container {
	return corev1.Container{
		Name:    "producer",
		Image:   "nvidia/cuda:12.4.1-devel-ubuntu22.04",
		Command: []string{"/bin/bash", "-c"},
		Env:     []corev1.EnvVar{
			// ideally we shouldn't have to set this env var, usually nvidia
			// dra driver or ctk will inject this if necessary.
			// {Name: "CUDA_VISIBLE_DEVICES", Value: "0"},
		},
		Args: []string{`
      echo "Producer: Listing available GPUs with nvidia-smi..."
      nvidia-smi -L
      echo "Producer: GPU UUIDs:"
      nvidia-smi --query-gpu=uuid --format=csv,noheader
      echo "Producer: GPU information complete."
      echo ""

      cat > /tmp/producer.cu << 'EOF'
      #include <cuda_runtime.h>
      #include <stdio.h>
      #include <unistd.h>
      #include <stdlib.h>

      int main() {
          void* devPtr;
          cudaIpcMemHandle_t handle;
          int deviceCount;

          printf("Producer: Initializing CUDA...\n");
          fflush(stdout);

          // Check available GPUs
          cudaError_t err = cudaGetDeviceCount(&deviceCount);
          if (err != cudaSuccess) {
              printf("ERROR getting device count: %s\n", cudaGetErrorString(err));
              return 1;
          }
          printf("Producer: Found %d GPU(s)\n", deviceCount);

          // List all available GPUs
          for (int i = 0; i < deviceCount; i++) {
              cudaDeviceProp prop;
              cudaGetDeviceProperties(&prop, i);
              printf("Producer: GPU %d: %s\n", i, prop.name);
          }
          fflush(stdout);

          err = cudaSetDevice(0);
          if (err != cudaSuccess) {
              printf("ERROR setting CUDA device: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Producer: Allocating GPU memory on device 0...\n");
          fflush(stdout);
          err = cudaMalloc(&devPtr, 1024 * 1024); // 1MB
          if (err != cudaSuccess) {
              printf("ERROR allocating GPU memory: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Producer: Writing test data to GPU memory...\n");
          fflush(stdout);
          int* hostData = (int*)malloc(1024 * 1024);
          for (int i = 0; i < 256 * 1024; i++) {
              hostData[i] = i + 42; // Simple pattern: index + 42
          }
          err = cudaMemcpy(devPtr, hostData, 1024 * 1024, cudaMemcpyHostToDevice);
          if (err != cudaSuccess) {
              printf("ERROR copying data to GPU: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Producer: Creating IPC handle...\n");
          fflush(stdout);
          err = cudaIpcGetMemHandle(&handle, devPtr);
          if (err != cudaSuccess) {
              printf("ERROR creating IPC handle: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Producer: Writing handle to shared volume...\n");
          fflush(stdout);
          FILE* f = fopen("/shared/cuda_ipc_handle.dat", "wb");
          if (!f) {
              printf("ERROR: Could not open handle file for writing\n");
              return 1;
          }
          fwrite(&handle, sizeof(handle), 1, f);
          fclose(f);

          printf("Producer: Success! Memory contains values 42, 43, 44, 45, 46...\n");
          printf("Producer: Hanging infinitely to keep GPU memory alive...\n");
          fflush(stdout);

          // signal that the producer has created the IPC handle
          FILE* ready = fopen("/tmp/ready", "w");
          if (!ready) {
              printf("ERROR: Could not open liveness file for writing\n");
              return 1;
          }
          fclose(ready);

          // Hang forever to keep the GPU memory allocated
          while (1) {
              sleep(3600);
          }

          return 0;
      }
EOF

      echo "Compiling producer..."
      nvcc /tmp/producer.cu -o /tmp/producer
      echo "Starting producer..."
      /tmp/producer

`},
	}
}

func consumerCtr(uuid string) corev1.Container {
	return corev1.Container{
		Name:    "consumer",
		Image:   "nvidia/cuda:12.4.1-devel-ubuntu22.04",
		Command: []string{"/bin/bash", "-c"},
		// this dictates to the consumer which gpu it should set using cudaSetDevice
		// since it has access to the producer gpu as well
		Env: []corev1.EnvVar{
			{Name: "GPU_UUID", Value: uuid},
		},
		Args: []string{`
      echo "Consumer: Waiting for producer to create handle..."

      # Wait for the handle file to be created
      while [ ! -f /shared/cuda_ipc_handle.dat ]; do
          echo "Consumer: Waiting for handle file..."
          sleep 2
      done

      echo "Consumer: Handle file found!"
      echo "Consumer: Listing available GPUs with nvidia-smi..."
      nvidia-smi -L
      echo "Consumer: GPU UUIDs:"
      nvidia-smi --query-gpu=uuid --format=csv,noheader
      echo "Consumer: GPU information complete."
      echo ""
      sleep 2

      cat > /tmp/consumer.cu << 'EOF'
      #include <cuda_runtime.h>
      #include <stdio.h>
      #include <unistd.h>
      #include <stdlib.h>

      // Helper function to print a cudaUUID_t for verification
      void print_uuid(const cudaUUID_t uuid) {
        for (int i = 0; i < 16; i++) {
          printf("%02x", (unsigned char)uuid.bytes[i]);
            if (i == 3 || i == 5 || i == 7 || i == 9) {
              printf("-");
            }
        }
        printf("\n");
      }

      int convert_to_uuid(const char* src, unsigned char* dest) {
          // Use sscanf to parse the UUID string with hyphens
          int bytes_read;
          bytes_read = sscanf(src,
            "%02hhx%02hhx%02hhx%02hhx-"
            "%02hhx%02hhx-"
            "%02hhx%02hhx-"
            "%02hhx%02hhx-"
            "%02hhx%02hhx%02hhx%02hhx%02hhx%02hhx",
            &dest[0], &dest[1], &dest[2], &dest[3],
            &dest[4], &dest[5],
            &dest[6], &dest[7],
            &dest[8], &dest[9],
            &dest[10], &dest[11], &dest[12], &dest[13], &dest[14], &dest[15]);
          return bytes_read;
      }

      // Helper function to compare two cudaUUID_t structures
      bool uuid_equal(cudaUUID_t a, cudaUUID_t b) {
        for (int i = 0; i < sizeof(a.bytes); i++) {
          if (a.bytes[i] != b.bytes[i]) {
            return false;
          }
        }
        return true;
      }

      int main() {
          const char* uuid_env_var = getenv("GPU_UUID");
          if (uuid_env_var == NULL) {
            printf("Error: Environment variable 'GPU_UUID' not set.\n");
            return 1;
          }
          printf("Read GPU UUID from env: %s\n", uuid_env_var);

          cudaUUID_t uuid;
          // A pointer to the byte array inside the cudaUUID_t struct
          unsigned char* uuid_bytes = (unsigned char*)uuid.bytes;
          int bytes_read;
          bytes_read = convert_to_uuid(uuid_env_var, uuid_bytes);
          if (bytes_read != 16) {
            printf("Error: Failed to parse UUID string. Expected 16 bytes, but read %d.\n", bytes_read);
            return 1;
          }

          printf("Converted to cudaUUID_t: ");
          print_uuid(uuid);
          printf("Consumer: Initializing CUDA...\n");
          fflush(stdout);

          // find the matching GPU
          int deviceCount;
          cudaError_t err = cudaGetDeviceCount(&deviceCount);
          if (err != cudaSuccess) {
              printf("ERROR getting device count: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Consumer: Found %d GPU(s), finding the matching device index\n", deviceCount);
          int device_index = -1;
          for (int i = 0; i < deviceCount; i++) {
              cudaDeviceProp prop;
              cudaGetDeviceProperties(&prop, i);
              int match = memcmp(prop.uuid.bytes, uuid.bytes, sizeof(uuid.bytes));
              printf("Consumer: GPU index: %d, name: %s, match: %d, uuid: ", i, prop.name, match == 0);
              print_uuid(prop.uuid);
              if (match == 0) {
                device_index = i;
                break;
              }
          }
          if (device_index == -1) {
            printf("Could not find a CUDA device matching the UUID.\n");
            return 1;
          }

          err = cudaSetDevice(device_index);
          if (err != cudaSuccess) {
              printf("ERROR setting CUDA device: %s\n", cudaGetErrorString(err));
              return 1;
          }

          void* devPtr;
          cudaIpcMemHandle_t handle;

          printf("Consumer: Reading IPC handle from shared volume...\n");
          fflush(stdout);
          FILE* f = fopen("/shared/cuda_ipc_handle.dat", "rb");
          if (!f) {
              printf("ERROR: Handle file not found\n");
              return 1;
          }
          fread(&handle, sizeof(handle), 1, f);
          fclose(f);

          printf("Consumer: Opening IPC memory handle...\n");
          fflush(stdout);
          err = cudaIpcOpenMemHandle(&devPtr, handle, cudaIpcMemLazyEnablePeerAccess);
          if (err != cudaSuccess) {
              printf("ERROR opening IPC handle: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Consumer: Successfully opened shared GPU memory!\n");
          fflush(stdout);

          // Read the data once
          int* hostData = (int*)malloc(1024 * sizeof(int));
          err = cudaMemcpy(hostData, devPtr, 1024 * sizeof(int), cudaMemcpyDeviceToHost);
          if (err != cudaSuccess) {
              printf("ERROR reading GPU memory: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Consumer: First 10 values from shared memory: ");
          for (int i = 0; i < 10; i++) {
              printf("%d ", hostData[i]);
          }
          printf("\n");
          fflush(stdout);

          // Verify expected pattern (index + 42)
          bool correct = true;
          for (int i = 0; i < 1024; i++) {
              if (hostData[i] != i + 42) {
                  correct = false;
                  break;
              }
          }

          if (correct) {
              printf("Consumer: ✓ Data verification PASSED!\n");
          } else {
              printf("Consumer: ✗ Data verification FAILED!\n");
          }

          printf("Consumer: Success! Hanging infinitely...\n");
          fflush(stdout);

          // signal that the consumer has read the data through the IPC handle
          FILE* ready = fopen("/tmp/ready", "w");
          if (!ready) {
              printf("ERROR: Could not open liveness file for writing\n");
              return 1;
          }
          fclose(ready);

          // Hang forever
          while (1) {
              sleep(3600);
          }

          return 0;
      }
EOF

      echo "Compiling consumer..."
      nvcc /tmp/consumer.cu -o /tmp/consumer
      echo "Starting consumer..."
      /tmp/consumer

`},
	}
}
