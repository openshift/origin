// +build functional

package cri_containerd

import (
	"context"
	"fmt"
	"testing"

	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func createContainer(t *testing.T, client runtime.RuntimeServiceClient, ctx context.Context, request *runtime.CreateContainerRequest) string {
	response, err := client.CreateContainer(ctx, request)
	if err != nil {
		t.Fatalf("failed CreateContainer in sandbox: %s, with: %v", request.PodSandboxId, err)
	}
	return response.ContainerId
}

func stopAndRemoveContainer(t *testing.T, client runtime.RuntimeServiceClient, ctx context.Context, containerID string) {
	_, err := client.StopContainer(ctx, &runtime.StopContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		// Error here so we can still attempt the delete
		t.Errorf("failed StopContainer request for container: %s, with: %v", containerID, err)
	}
	_, err = client.RemoveContainer(ctx, &runtime.RemoveContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		t.Fatalf("failed StopContainer request for container: %s, with: %v", containerID, err)
	}
}

func runCreateContainerTest(t *testing.T, runtimeHandler string, request *runtime.CreateContainerRequest) {
	sandboxRequest := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name() + "-Sandbox",
				Uid:       "0",
				Namespace: testNamespace,
			},
		},
		RuntimeHandler: runtimeHandler,
	}
	runCreateContainerTestWithSandbox(t, sandboxRequest, request)
}

func runCreateContainerTestWithSandbox(t *testing.T, sandboxRequest *runtime.RunPodSandboxRequest, request *runtime.CreateContainerRequest) {
	client := newTestRuntimeClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	podID := runPodSandbox(t, client, ctx, sandboxRequest)
	defer func() {
		stopAndRemovePodSandbox(t, client, ctx, podID)
	}()

	request.PodSandboxId = podID
	request.SandboxConfig = sandboxRequest.Config
	containerID := createContainer(t, client, ctx, request)
	defer func() {
		stopAndRemoveContainer(t, client, ctx, containerID)
	}()
	_, err := client.StartContainer(ctx, &runtime.StartContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		t.Fatalf("failed StartContainer request for container: %s, with: %v", containerID, err)
	}
}

func Test_CreateContainer_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			// Hold this command open until killed
			Command: []string{
				"top",
			},
		},
	}
	runCreateContainerTest(t, lcowRuntimeHandler, request)
}

func Test_CreateContainer_WCOW_Process_Tty(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Tty will hold this open until killed.
			Command: []string{
				"cmd",
			},
			Tty: true,
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_WCOW_Hypervisor_Tty(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Tty will hold this open until killed.
			Command: []string{
				"cmd",
			},
			Tty: true,
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_LCOW_Tty(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			// Tty will hold this open until killed.
			Command: []string{
				"sh",
			},
			Tty: true,
		},
	}
	runCreateContainerTest(t, lcowRuntimeHandler, request)
}

func Test_CreateContainer_LCOW_Privileged(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	sandboxRequest := &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Name:      t.Name() + "-Sandbox",
				Uid:       "0",
				Namespace: testNamespace,
			},
			Linux: &runtime.LinuxPodSandboxConfig{
				SecurityContext: &runtime.LinuxSandboxSecurityContext{
					Privileged: true,
				},
			},
		},
		RuntimeHandler: lcowRuntimeHandler,
	}

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			// Hold this command open until killed
			Command: []string{
				"top",
			},
			Linux: &runtime.LinuxContainerConfig{
				SecurityContext: &runtime.LinuxContainerSecurityContext{
					Privileged: true,
				},
			},
		},
	}
	runCreateContainerTestWithSandbox(t, sandboxRequest, request)
}

func Test_CreateContainer_MemorySize_Config_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					MemoryLimitInBytes: 768 * 1024 * 1024, // 768MB
				},
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_MemorySize_Annotation_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.memory.sizeinmb": fmt.Sprintf("%d", 768*1024*1024), // 768MB
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_MemorySize_Config_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					MemoryLimitInBytes: 768 * 1024 * 1024, // 768MB
				},
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_MemorySize_Annotation_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.memory.sizeinmb": fmt.Sprintf("%d", 768*1024*1024), // 768MB
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_MemorySize_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			// Hold this command open until killed
			Command: []string{
				"top",
			},
			Linux: &runtime.LinuxContainerConfig{
				Resources: &runtime.LinuxContainerResources{
					MemoryLimitInBytes: 768 * 1024 * 1024, // 768MB
				},
			},
		},
	}
	runCreateContainerTest(t, lcowRuntimeHandler, request)
}

func Test_CreateContainer_CPUCount_Config_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					CpuCount: 1,
				},
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_CPUCount_Annotation_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.count": "1",
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_CPUCount_Config_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					CpuCount: 1,
				},
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_CPUCount_Annotation_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.count": "1",
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_CPUCount_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			// Hold this command open until killed
			Command: []string{
				"top",
			},
			Linux: &runtime.LinuxContainerConfig{
				Resources: &runtime.LinuxContainerResources{
					CpusetCpus: "0-3",
				},
			},
		},
	}
	runCreateContainerTest(t, lcowRuntimeHandler, request)
}

func Test_CreateContainer_CPULimit_Config_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					CpuMaximum: 9000,
				},
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_CPULimit_Annotation_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.limit": "9000",
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_CPULimit_Config_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					CpuMaximum: 9000,
				},
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_CPULimit_Annotation_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.limit": "9000",
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_CPUQuota_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			// Hold this command open until killed
			Command: []string{
				"top",
			},
			Linux: &runtime.LinuxContainerConfig{
				Resources: &runtime.LinuxContainerResources{
					CpuQuota:  1000000,
					CpuPeriod: 500000,
				},
			},
		},
	}
	runCreateContainerTest(t, lcowRuntimeHandler, request)
}

func Test_CreateContainer_CPUWeight_Config_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					CpuShares: 500,
				},
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_CPUWeight_Annotation_WCOW_Process(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.weight": "500",
			},
		},
	}
	runCreateContainerTest(t, wcowProcessRuntimeHandler, request)
}

func Test_CreateContainer_CPUWeight_Config_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Windows: &runtime.WindowsContainerConfig{
				Resources: &runtime.WindowsContainerResources{
					CpuMaximum: 500,
				},
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_CPUWeight_Annotation_WCOW_Hypervisor(t *testing.T) {
	pullRequiredImages(t, []string{imageWindowsRS5Nanoserver})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageWindowsRS5Nanoserver,
			},
			// Hold this command open until killed (pause for Windows)
			Command: []string{
				"cmd",
				"/c",
				"ping",
				"-t",
				"127.0.0.1",
			},
			Annotations: map[string]string{
				"io.microsoft.container.processor.limit": "500",
			},
		},
	}
	runCreateContainerTest(t, wcowHypervisorRuntimeHandler, request)
}

func Test_CreateContainer_CPUShares_LCOW(t *testing.T) {
	pullRequiredLcowImages(t, []string{imageLcowK8sPause, imageLcowAlpine})

	request := &runtime.CreateContainerRequest{
		Config: &runtime.ContainerConfig{
			Metadata: &runtime.ContainerMetadata{
				Name: t.Name() + "-Container",
			},
			Image: &runtime.ImageSpec{
				Image: imageLcowAlpine,
			},
			// Hold this command open until killed
			Command: []string{
				"top",
			},
			Linux: &runtime.LinuxContainerConfig{
				Resources: &runtime.LinuxContainerResources{
					CpuShares: 1024,
				},
			},
		},
	}
	runCreateContainerTest(t, lcowRuntimeHandler, request)
}
