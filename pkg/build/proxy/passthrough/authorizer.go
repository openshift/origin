package passthrough

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/build/proxy/interceptor"
)

// DefaultAuthorizer allows a local client to authorize a build by writing a file
// to the local filesystem, and then passing a cgroup parent to the build that points
// to the correct cgroup container.
// TODO: support normal systemd cgroups.
type DefaultAuthorizer struct {
	Client *docker.Client
	File   string
}

var _ interceptor.BuildAuthorizer = &DefaultAuthorizer{}

var (
	validPodSlice = regexp.MustCompile(
		`^` +
			`/kubepods\.slice/kubepods-(burstable|besteffort|guaranteed)\.slice` +
			`/kubepods-(\w+)-pod(\w+)\.slice` +
			`/docker-(\w+).scope` +
			`$`,
	)
)

func (a *DefaultAuthorizer) AuthorizeBuildRequest(ctx context.Context, build *interceptor.BuildImageOptions, auth *interceptor.AuthOptions) (*interceptor.BuildImageOptions, error) {
	safeBuild, err := copySafe(build)
	if err != nil {
		return nil, err
	}

	switch parent := build.CgroupParent; {
	case strings.HasPrefix(parent, "/kubepods.slice/"):
		podUID, containerID, parent, err := parseKubeCgroupParent(parent)
		if err != nil {
			return nil, err
		}

		container, err := a.Client.InspectContainer(containerID)
		if err != nil {
			return nil, fmt.Errorf("could not verify cgroup parent container exists: %v", err)
		}
		if container.Config.Labels["io.kubernetes.pod.uid"] != podUID {
			return nil, fmt.Errorf("requested container by cgroup is not in the correct pod")
		}
		if container.HostConfig.CgroupParent != parent {
			return nil, fmt.Errorf("the container cgroup parent does not match the expected cgroup parent")
		}
		if len(container.HostConfig.NetworkMode) == 0 {
			return nil, fmt.Errorf("the container does not have a limiting network mode and is not a valid build target")
		}

		token, err := retrieveContainerCredentials(ctx, a.Client, containerID, a.File)
		if err != nil {
			return nil, fmt.Errorf("unable to get contents of the file %s in the requested container: %v", a.File, err)
		}
		if token != auth.Password {
			return nil, fmt.Errorf("container file %s contents do not match provided password", a.File)
		}

		safeBuild.NetworkMode = container.HostConfig.NetworkMode
		safeBuild.CgroupParent = parent
		safeBuild.Names = build.Names

	default:
		return nil, fmt.Errorf("only requests within a cgroup parented pod are allowed (/kubepods.slice/...)")
	}

	return safeBuild, nil
}

// parseKubeCgroupParent expects to receive a valid Kubernetes cgroup hierarchy containing the pod UID and
// the docker container ID. It returns an error if it cannot find a matching value.
// Returns pod UID, container ID, and the parent cgroup if no error is found.
func parseKubeCgroupParent(parent string) (podUID string, containerID string, parentCgroup string, err error) {
	matches := validPodSlice.FindStringSubmatch(parent)
	if len(matches) == 0 {
		return "", "", "", fmt.Errorf("cgroup parent value did not match the expected format for a Kubernetes container (%s)", validPodSlice)
	}
	if matches[1] != matches[2] {
		return "", "", "", fmt.Errorf("cgroup parent value did not match the expected format for a Kubernetes container: pod slice does not have the same QoS value")
	}

	podUIDString := strings.Replace(matches[3], "_", "-", -1)
	container := matches[4]

	// the parent cgroup is the name of the next highest level
	parent = path.Base(path.Dir(parent))

	return podUIDString, container, parent, nil
}

// copySafe returns only the options that have no security impact. All other fields must be explicitly copied.
func copySafe(build *interceptor.BuildImageOptions) (*interceptor.BuildImageOptions, error) {
	return &interceptor.BuildImageOptions{
		// Names
		Dockerfile:          build.Dockerfile,
		NoCache:             build.NoCache,
		SuppressOutput:      build.SuppressOutput,
		Pull:                build.Pull,
		RmTmpContainer:      build.RmTmpContainer,
		ForceRmTmpContainer: build.ForceRmTmpContainer,
		//Memory
		//Memswap
		//CPUShares
		//CPUQuota
		//CPUPeriod
		//CPUSetCPUs
		Labels:     build.Labels,
		Remote:     build.Remote,
		ContextDir: build.ContextDir,
		// Ulimits
		BuildArgs: build.BuildArgs,
		// NetworkMode
		// CgroupParent

		// parameters that are not in go-dockerclient yet
		ExtraHosts: build.ExtraHosts,
		// CPUSetMems
		// CacheFrom
		// ShmSize
		Squash: build.Squash,
		// Isolation
	}, nil
}

// retrieveContainerCredentials attempts to read a single file from the filesystem of the specified container.
func retrieveContainerCredentials(ctx context.Context, client *docker.Client, containerID, file string) (string, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 50*1024))
	if err := client.DownloadFromContainer(containerID, docker.DownloadFromContainerOptions{
		Context:           ctx,
		Path:              file,
		OutputStream:      buf,
		InactivityTimeout: 3 * time.Second,
	}); err != nil {
		return "", err
	}

	r := tar.NewReader(buf)
	h, err := r.Next()
	if err != nil {
		return "", fmt.Errorf("unable to read first credentials entry: %v", err)
	}
	if h.FileInfo().IsDir() {
		return "", fmt.Errorf("%s must not be a directory", file)
	}
	if h.Name != path.Base(file) {
		return "", fmt.Errorf("unexpected credentials tar entry: %v", err)
	}
	buf = &bytes.Buffer{}
	if _, err := buf.ReadFrom(r); err != nil {
		return "", fmt.Errorf("unable to read from credentials archive: %v", err)
	}
	return buf.String(), nil
}

// imageSafeCharacters are characters allowed to be part of a Docker image name.
const imageSafeCharacters = "abcdefghijklmnopqrstuvwxyz0123456789"

// randSeq returns a sequence of random characters drawn from source. It returns
// an error if cryptographic randomness is not available or source is more than 255
// characters.
func randSeq(source string, n int) (string, error) {
	if len(source) > 255 {
		return "", fmt.Errorf("source must be less than 256 bytes long")
	}
	random := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", err
	}
	for i := range random {
		random[i] = source[random[i]%byte(len(source))]
	}
	return string(random), nil
}
