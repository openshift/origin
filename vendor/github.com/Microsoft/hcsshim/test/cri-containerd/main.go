// +build functional

package cri_containerd

import (
	"context"
	"net"
	"testing"
	"time"

	_ "github.com/Microsoft/hcsshim/test/functional/manifest"
	"google.golang.org/grpc"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

const (
	daemonAddress                = "tcp://127.0.0.1:2376"
	connectTimeout               = time.Second * 10
	testNamespace                = "cri-containerd-test"
	wcowProcessRuntimeHandler    = "runhcs-wcow-process"
	wcowHypervisorRuntimeHandler = "runhcs-wcow-hypervisor"
	lcowRuntimeHandler           = "runhcs-lcow"
	imageWindowsRS5Nanoserver    = "mcr.microsoft.com/windows/nanoserver:1809"
	imageWindowsRS5Servercore    = "mcr.microsoft.com/windows/servercore:1809"
	imageLcowK8sPause            = "k8s.gcr.io/pause:3.1"
	imageLcowAlpine              = "docker.io/library/alpine:latest"
)

func newTestRuntimeClient(t *testing.T) runtime.RuntimeServiceClient {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, daemonAddress, grpc.WithInsecure(), grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout("tcp", "127.0.0.1:2376", timeout)
	}))
	if err != nil {
		t.Fatalf("failed to dial runtime client: %v", err)
	}
	return runtime.NewRuntimeServiceClient(conn)
}

func newTestImageClient(t *testing.T) runtime.ImageServiceClient {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, daemonAddress, grpc.WithInsecure(), grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout("tcp", "127.0.0.1:2376", timeout)
	}))
	if err != nil {
		t.Fatalf("failed to dial runtime client: %v", err)
	}
	return runtime.NewImageServiceClient(conn)
}

func pullRequiredImages(t *testing.T, images []string) {
	pullRequiredImagesWithLabels(t, images, map[string]string{
		"sandbox-platform": "windows/amd64", // Not required for Windows but makes the test safer depending on defaults in the config.
	})
}

func pullRequiredLcowImages(t *testing.T, images []string) {
	pullRequiredImagesWithLabels(t, images, map[string]string{
		"sandbox-platform": "linux/amd64",
	})
}

func pullRequiredImagesWithLabels(t *testing.T, images []string, labels map[string]string) {
	if len(images) < 1 {
		return
	}

	client := newTestImageClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sb := &runtime.PodSandboxConfig{
		Labels: labels,
	}
	for _, image := range images {
		_, err := client.PullImage(ctx, &runtime.PullImageRequest{
			Image: &runtime.ImageSpec{
				Image: image,
			},
			SandboxConfig: sb,
		})
		if err != nil {
			t.Fatalf("failed PullImage for image: %s, with error: %v", image, err)
		}
	}
}
