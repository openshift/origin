package builder

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/generate/git"
	utilglog "github.com/openshift/origin/pkg/util/glog"
)

// glog is a placeholder until the builders pass an output stream down
// client facing libraries should not be using glog
var glog = utilglog.ToFile(os.Stderr, 2)

const OriginalSourceURLAnnotationKey = "openshift.io/original-source-url"

// KeyValue can be used to build ordered lists of key-value pairs.
type KeyValue struct {
	Key   string
	Value string
}

// GitClient performs git operations
type GitClient interface {
	CloneWithOptions(dir string, url string, args ...string) error
	Checkout(dir string, ref string) error
	SubmoduleUpdate(dir string, init, recursive bool) error
	TimedListRemote(timeout time.Duration, url string, args ...string) (string, string, error)
	GetInfo(location string) (*git.SourceInfo, []error)
}

// buildInfo returns a slice of KeyValue pairs with build metadata to be
// inserted into Docker images produced by build.
func buildInfo(build *api.Build) []KeyValue {
	kv := []KeyValue{
		{"OPENSHIFT_BUILD_NAME", build.Name},
		{"OPENSHIFT_BUILD_NAMESPACE", build.Namespace},
	}
	if build.Spec.Source.Git != nil {
		sourceURL := build.Spec.Source.Git.URI
		if originalURL, ok := build.Annotations[OriginalSourceURLAnnotationKey]; ok {
			sourceURL = originalURL
		}
		kv = append(kv, KeyValue{"OPENSHIFT_BUILD_SOURCE", sourceURL})
		if build.Spec.Source.Git.Ref != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_REFERENCE", build.Spec.Source.Git.Ref})
		}
		if build.Spec.Revision != nil && build.Spec.Revision.Git != nil && build.Spec.Revision.Git.Commit != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_COMMIT", build.Spec.Revision.Git.Commit})
		}
	}
	if build.Spec.Strategy.SourceStrategy != nil {
		env := build.Spec.Strategy.SourceStrategy.Env
		for _, e := range env {
			kv = append(kv, KeyValue{e.Name, e.Value})
		}
	}
	return kv
}

func updateBuildRevision(c client.BuildInterface, build *api.Build, sourceInfo *git.SourceInfo) {
	if build.Spec.Revision != nil {
		return
	}
	build.Spec.Revision = &api.SourceRevision{
		Git: &api.GitSourceRevision{
			Commit:  sourceInfo.CommitID,
			Message: sourceInfo.Message,
			Author: api.SourceControlUser{
				Name:  sourceInfo.AuthorName,
				Email: sourceInfo.AuthorEmail,
			},
			Committer: api.SourceControlUser{
				Name:  sourceInfo.CommitterName,
				Email: sourceInfo.CommitterEmail,
			},
		},
	}

	// Reset ResourceVersion to avoid a conflict with other updates to the build
	build.ResourceVersion = ""

	glog.V(4).Infof("Setting build revision to %#v", build.Spec.Revision.Git)
	_, err := c.UpdateDetails(build)
	if err != nil {
		glog.V(0).Infof("error: An error occurred saving build revision: %v", err)
	}
}

// randomBuildTag generates a random tag used for building images in such a way
// that the built image can be referred to unambiguously even in the face of
// concurrent builds with the same name in the same namespace.
func randomBuildTag(namespace, name string) string {
	repo := fmt.Sprintf("%s/%s", namespace, name)
	randomTag := fmt.Sprintf("%08x", rand.Uint32())
	maxRepoLen := reference.NameTotalLengthMax - len(randomTag) - 1
	if len(repo) > maxRepoLen {
		repo = fmt.Sprintf("%x", sha1.Sum([]byte(repo)))
	}
	return fmt.Sprintf("%s:%s", repo, randomTag)
}

// containerNamePrefix prefixes the name of containers launched by a build. We
// cannot reuse the prefix "k8s" because we don't want the containers to be
// managed by a kubelet.
const containerNamePrefix = "openshift"

// containerName creates names for Docker containers launched by a build. It is
// meant to resemble Kubernetes' pkg/kubelet/dockertools.BuildDockerName.
func containerName(strategyName, buildName, namespace, containerPurpose string) string {
	uid := fmt.Sprintf("%08x", rand.Uint32())
	return fmt.Sprintf("%s_%s-build_%s_%s_%s_%s",
		containerNamePrefix,
		strategyName,
		buildName,
		namespace,
		containerPurpose,
		uid)
}

// execPostCommitHook uses the client to execute a command based on the
// postCommitSpec in a new ephemeral Docker container running the given image.
// It returns an error if the hook cannot be run or returns a non-zero exit
// code.
func execPostCommitHook(client DockerClient, postCommitSpec api.BuildPostCommitSpec, image, containerName string) error {
	command := postCommitSpec.Command
	args := postCommitSpec.Args
	script := postCommitSpec.Script
	if script == "" && len(command) == 0 && len(args) == 0 {
		// Post commit hook is not set, return early.
		return nil
	}
	glog.V(0).Infof("Running post commit hook ...")
	glog.V(4).Infof("Post commit hook spec: %+v", postCommitSpec)

	if script != "" {
		// The `-i` flag is needed to support CentOS and RHEL images
		// that use Software Collections (SCL), in order to have the
		// appropriate collections enabled in the shell. E.g., in the
		// Ruby image, this is necessary to make `ruby`, `bundle` and
		// other binaries available in the PATH.
		command = []string{"/bin/sh", "-ic"}
		args = append([]string{script, command[0]}, args...)
	}

	limits, err := GetCGroupLimits()
	if err != nil {
		return fmt.Errorf("read cgroup limits: %v", err)
	}

	return dockerRun(client, docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image:      image,
			Entrypoint: command,
			Cmd:        args,
		},
		HostConfig: &docker.HostConfig{
			// Limit container's resource allocation.
			CPUShares:  limits.CPUShares,
			CPUPeriod:  limits.CPUPeriod,
			CPUQuota:   limits.CPUQuota,
			Memory:     limits.MemoryLimitBytes,
			MemorySwap: limits.MemorySwap,
		},
	}, docker.LogsOptions{
		// Stream logs to stdout and stderr.
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Follow:       true,
		Stdout:       true,
		Stderr:       true,
	})
}
