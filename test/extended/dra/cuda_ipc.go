package dra

import (
	"context"
	"fmt"
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

type shareDataWithCUDAIPC struct {
	f      *framework.Framework
	class  string
	node   *corev1.Node
	driver *nvidia.NvidiaDRADriverGPU
}

func (spec shareDataWithCUDAIPC) Test(ctx context.Context, t testing.TB) {
	namespace := spec.f.Namespace.Name
	clientset := spec.f.ClientSet

	newRequest := func(name, gpuIndex string) resourceapi.DeviceRequest {
		return resourceapi.DeviceRequest{
			Name:            name,
			DeviceClassName: spec.class,
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: fmt.Sprintf("device.attributes['%s'].index == %s", spec.class, gpuIndex),
					},
				},
			},
		}
	}
	claim := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "single-gpu",
		},
	}
	claim.Spec.Devices.Requests = []resourceapi.DeviceRequest{
		newRequest("producer", "0"),
		newRequest("consumer", "1"),
	}

	// producer pod
	producer := helper.NewPod(namespace, "producer")
	producer.Spec.Containers = []corev1.Container{producerCtr()}
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
	consumer.Spec.Containers = []corev1.Container{consumerCtr()}
	consumer.Spec.Containers[0].Resources.Claims = []corev1.ResourceClaim{{Name: "gpu", Request: "consumer"}}
	consumer.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:              "gpu",
			ResourceClaimName: ptr.To(claim.Name),
		},
	}
	shared(consumer)

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

	ctr := pod.Spec.Containers[0]
	ctr.VolumeMounts = append(ctr.VolumeMounts, corev1.VolumeMount{Name: "shared-volume", MountPath: "/shared"})
	ctr.SecurityContext = &corev1.SecurityContext{
		Privileged: ptr.To(true),
		Capabilities: &corev1.Capabilities{
			Add: []corev1.Capability{"IPC_LOCK"},
		},
	}
}

func producerCtr() corev1.Container {
	return corev1.Container{
		Name:    "producer",
		Image:   "nvidia/cuda:12.4.1-devel-ubuntu22.04",
		Command: []string{"/bin/bash", "-c"},
		Env: []corev1.EnvVar{
			{Name: "CUDA_VISIBLE_DEVICES", Value: "0"},
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

          printf("Producer: Using GPU 0 for CUDA operations...\n");
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

func consumerCtr() corev1.Container {
	return corev1.Container{
		Name:    "consumer",
		Image:   "nvidia/cuda:12.4.1-devel-ubuntu22.04",
		Command: []string{"/bin/bash", "-c"},
		Env: []corev1.EnvVar{
			{Name: "CUDA_VISIBLE_DEVICES", Value: "1"},
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

      int main() {
          void* devPtr;
          cudaIpcMemHandle_t handle;
          int deviceCount;

          printf("Consumer: Initializing CUDA...\n");
          fflush(stdout);

          // Check available GPUs
          cudaError_t err = cudaGetDeviceCount(&deviceCount);
          if (err != cudaSuccess) {
              printf("ERROR getting device count: %s\n", cudaGetErrorString(err));
              return 1;
          }
          printf("Consumer: Found %d GPU(s)\n", deviceCount);

          // List all available GPUs
          for (int i = 0; i < deviceCount; i++) {
              cudaDeviceProp prop;
              cudaGetDeviceProperties(&prop, i);
              printf("Consumer: GPU %d: %s\n", i, prop.name);
          }
          fflush(stdout);

          err = cudaSetDevice(1);
          if (err != cudaSuccess) {
              printf("ERROR setting CUDA device: %s\n", cudaGetErrorString(err));
              return 1;
          }

          printf("Consumer: Using GPU 1 for CUDA operations...\n");

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
