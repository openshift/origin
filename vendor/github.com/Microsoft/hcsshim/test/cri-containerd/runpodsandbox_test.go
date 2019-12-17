// +build functional

package cri_containerd

import (
	"context"
	"fmt"
	"testing"

	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func runPodSandbox(t *testing.T, client runtime.RuntimeServiceClient, ctx context.Context, request *runtime.RunPodSandboxRequest) string {
	response, err := client.RunPodSandbox(ctx, request)
	if err != nil {
		t.Fatalf("failed RunPodSandbox request with: %v", err)
	}
	return response.PodSandboxId
}

func stopAndRemovePodSandbox(t *testing.T, client runtime.RuntimeServiceClient, ctx context.Context, podID string) {
	_, err := client.StopPodSandbox(ctx, &runtime.StopPodSandboxRequest{
		PodSandboxId: podID,
	})
	if err != nil {
		// Error here so we can still attempt the delete
		t.Errorf("failed StopPodSandbox for sandbox: %s, request with: %v", podID, err)
	}
	_, err = client.RemovePodSandbox(ctx, &runtime.RemovePodSandboxRequest{
		PodSandboxId: podID,
	})
	if err != nil {
		t.Fatalf("failed RemovePodSandbox for sandbox: %s, request with: %v", podID, err)
	}
}

func runPodSandboxTest(t *testing.T, request *runtime.RunPodSandboxRequest) {
	client := newTestRuntimeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	podID := runPodSandbox(t, client, ctx, request)
	stopAndRemovePodSandbox(t, client, ctx, podID)
}

func Test_RunPodSandbox_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_VirtualMemory_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.allowovercommit": "true",
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_VirtualMemory_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.allowovercommit": "true",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_VirtualMemory_DeferredCommit_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.allowovercommit":      "true",
				"io.microsoft.virtualmachine.computetopology.memory.enabledeferredcommit": "true",
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_VirtualMemory_DeferredCommit_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.allowovercommit":      "true",
				"io.microsoft.virtualmachine.computetopology.memory.enabledeferredcommit": "true",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_PhysicalMemory_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.allowovercommit": "false",
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_PhysicalMemory_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.allowovercommit": "false",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_MemorySize_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.container.memory.sizeinmb": "128",
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_MemorySize_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.sizeinmb": "768", // 128 is too small for WCOW. It is really slow boot.
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_MemorySize_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.memory.sizeinmb": "128",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPUCount_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.count": "1",
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPUCount_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.processor.count": "1",
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPUCount_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.processor.count": "1",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPULimit_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.limit": "9000",
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPULimit_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.processor.limit": "9000",
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPULimit_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.processor.limit": "9000",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPUWeight_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.weight": "500",
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPUWeight_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.processor.weight": "500",
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_CPUWeight_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.computetopology.processor.weight": "500",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_StorageQoSBandwithMax_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.container.storage.qos.bandwidthmaximum": fmt.Sprintf("%d", 1024*1024), // 1MB/s
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_StorageQoSBandwithMax_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.storageqos.bandwidthmaximum": fmt.Sprintf("%d", 1024*1024), // 1MB/s
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_StorageQoSBandwithMax_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.storageqos.bandwidthmaximum": fmt.Sprintf("%d", 1024*1024), // 1MB/s
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_StorageQoSIopsMax_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.container.storage.qos.iopsmaximum": "300",
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_StorageQoSIopsMax_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.storageqos.iopsmaximum": "300",
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_StorageQoSIopsMax_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.storageqos.iopsmaximum": "300",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_InitrdBoot_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.lcow.preferredrootfstype": "initrd",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_RootfsVhdBoot_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			Annotations: map[string]string{
				"io.microsoft.virtualmachine.lcow.preferredrootfstype": "vhd",
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_DnsConfig_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			DnsConfig: &runtime.DNSConfig{
				Searches: []string{"8.8.8.8", "8.8.4.4"},
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
	// TODO: JTERRY75 - This is just a boot test at present. We need to create a
	// container, exec the ipconfig and parse the results to verify that the
	// searches are set.
}

func Test_RunPodSandbox_DnsConfig_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			DnsConfig: &runtime.DNSConfig{
				Searches: []string{"8.8.8.8", "8.8.4.4"},
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
	// TODO: JTERRY75 - This is just a boot test at present. We need to create a
	// container, exec the ipconfig and parse the results to verify that the
	// searches are set.
}

func Test_RunPodSandbox_DnsConfig_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			DnsConfig: &runtime.DNSConfig{
				Searches: []string{"8.8.8.8", "8.8.4.4"},
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
	// TODO: JTERRY75 - This is just a boot test at present. We need to create a
	// container, cat /etc/resolv.conf and parse the results to verify that the
	// searches are set.
}

func Test_RunPodSandbox_PortMappings_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			PortMappings: []*runtime.PortMapping{
				{
					Protocol:      runtime.Protocol_TCP,
					ContainerPort: 80,
					HostPort:      8080,
				},
			},
		},
		RuntimeHandler: wcowProcessRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_PortMappings_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			PortMappings: []*runtime.PortMapping{
				{
					Protocol:      runtime.Protocol_TCP,
					ContainerPort: 80,
					HostPort:      8080,
				},
			},
		},
		RuntimeHandler: wcowHypervisorRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}

func Test_RunPodSandbox_PortMappings_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause})

	request := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name(),
				Uid:       "0",
				Namespace: testNamespace,
			},
			PortMappings: []*runtime.PortMapping{
				{
					Protocol:      runtime.Protocol_TCP,
					ContainerPort: 80,
					HostPort:      8080,
				},
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}
	runPodSandboxTest(t, request)
}
