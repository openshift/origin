package builder

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/kubernetes/pkg/client/retry"

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

const (
	// containerNamePrefix prefixes the name of containers launched by a build.
	// We cannot reuse the prefix "k8s" because we don't want the containers to
	// be managed by a kubelet.
	containerNamePrefix = "openshift"
)

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
func buildInfo(build *api.Build, sourceInfo *git.SourceInfo) []KeyValue {
	kv := []KeyValue{
		{"OPENSHIFT_BUILD_NAME", build.Name},
		{"OPENSHIFT_BUILD_NAMESPACE", build.Namespace},
	}
	if build.Spec.Source.Git != nil {
		kv = append(kv, KeyValue{"OPENSHIFT_BUILD_SOURCE", build.Spec.Source.Git.URI})
		if build.Spec.Source.Git.Ref != "" {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_REFERENCE", build.Spec.Source.Git.Ref})
		}

		if sourceInfo != nil && len(sourceInfo.CommitID) != 0 {
			kv = append(kv, KeyValue{"OPENSHIFT_BUILD_COMMIT", sourceInfo.CommitID})
		} else if build.Spec.Revision != nil && build.Spec.Revision.Git != nil && build.Spec.Revision.Git.Commit != "" {
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
	}, docker.AttachToContainerOptions{
		// Stream logs to stdout and stderr.
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
	})
}

func updateBuildRevision(build *api.Build, sourceInfo *git.SourceInfo) *api.SourceRevision {
	if build.Spec.Revision != nil {
		return build.Spec.Revision
	}
	return &api.SourceRevision{
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
}

func retryBuildStatusUpdate(build *api.Build, client client.BuildInterface, sourceRev *api.SourceRevision) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// before updating, make sure we are using the latest version of the build
		latestBuild, err := client.Get(build.Name)
		if err != nil {
			// usually this means we failed to get resources due to the missing
			// privilleges
			return err
		}
		if sourceRev != nil {
			latestBuild.Spec.Revision = sourceRev
			latestBuild.ResourceVersion = ""
		}
		latestBuild.Status.Phase = build.Status.Phase
		latestBuild.Status.Reason = build.Status.Reason
		latestBuild.Status.Message = build.Status.Message
		latestBuild.Status.Output.To = build.Status.Output.To

		if _, err := client.UpdateDetails(latestBuild); err != nil {
			return err
		}
		return nil
	})
}

func handleBuildStatusUpdate(build *api.Build, client client.BuildInterface, sourceRev *api.SourceRevision) {
	if updateErr := retryBuildStatusUpdate(build, client, sourceRev); updateErr != nil {
		glog.Infof("error: Unable to update build status: %v", updateErr)
	}
}
