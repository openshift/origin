package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/builder/parser"
	"github.com/fsouza/go-dockerclient"

	kapi "k8s.io/kubernetes/pkg/api"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"

	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/docker/dockerfile"
	"github.com/openshift/origin/pkg/util/namer"
)

// ImageRefGenerator is an interface for generating ImageRefs
//
// Generators for ImageRef
// - Name              -> ImageRef
// - ImageRepo + tag   -> ImageRef
type ImageRefGenerator interface {
	FromName(name string) (*ImageRef, error)
	FromNameAndPorts(name string, ports []string) (*ImageRef, error)
	FromStream(repo *imageapi.ImageStream, tag string) (*ImageRef, error)
	FromDockerfile(name string, dir string, context string) (*ImageRef, error)
}

// SecretAccessor is an interface for retrieving secrets from the calling context.
type SecretAccessor interface {
	Token() (string, error)
	CACert() (string, error)
}

type imageRefGenerator struct{}

// NewImageRefGenerator creates a new ImageRefGenerator
func NewImageRefGenerator() ImageRefGenerator {
	return &imageRefGenerator{}
}

// FromName generates an ImageRef from a given name
func (g *imageRefGenerator) FromName(name string) (*ImageRef, error) {
	ref, err := imageapi.ParseDockerImageReference(name)
	if err != nil {
		return nil, err
	}
	return &ImageRef{
		Reference: ref,
		Info: &imageapi.DockerImage{
			Config: &imageapi.DockerConfig{},
		},
	}, nil
}

// FromNameAndPorts generates an ImageRef from a given name and ports
func (g *imageRefGenerator) FromNameAndPorts(name string, ports []string) (*ImageRef, error) {
	present := struct{}{}
	imageRef, err := g.FromName(name)
	if err != nil {
		return nil, err
	}
	exposedPorts := map[string]struct{}{}

	for _, p := range ports {
		exposedPorts[p] = present
	}

	imageRef.Info = &imageapi.DockerImage{
		Config: &imageapi.DockerConfig{
			ExposedPorts: exposedPorts,
		},
	}
	return imageRef, nil
}

// FromDockerfile generates an ImageRef from a given name, directory, and context path.
// The directory and context path will be joined and the resulting path should be a
// Dockerfile from where the image's ports will be extracted.
func (g *imageRefGenerator) FromDockerfile(name string, dir string, context string) (*ImageRef, error) {
	// Look for Dockerfile in repository
	file, err := os.Open(filepath.Join(dir, context, "Dockerfile"))
	if err != nil {
		return nil, err
	}

	node, err := parser.Parse(file)
	if err != nil {
		return nil, err
	}
	ports := dockerfile.LastExposedPorts(node)

	return g.FromNameAndPorts(name, ports)
}

// FromStream generates an ImageRef from an OpenShift ImageStream
func (g *imageRefGenerator) FromStream(stream *imageapi.ImageStream, tag string) (*ImageRef, error) {
	imageRef := &ImageRef{
		Stream: stream,
	}

	if tagged := imageapi.LatestTaggedImage(stream, tag); tagged != nil {
		if ref, err := imageapi.ParseDockerImageReference(tagged.DockerImageReference); err == nil {
			imageRef.ResolvedReference = &ref
			imageRef.Reference = ref
		}
	}

	if pullSpec := stream.Status.DockerImageRepository; len(pullSpec) != 0 {
		ref, err := imageapi.ParseDockerImageReference(pullSpec)
		if err != nil {
			return nil, err
		}
		switch {
		case len(tag) > 0:
			ref.Tag = tag
		case len(tag) == 0 && len(ref.Tag) == 0:
			ref.Tag = imageapi.DefaultImageTag
		}
		imageRef.Reference = ref
	}

	return imageRef, nil
}

// ImageRef is a reference to an image
type ImageRef struct {
	Reference imageapi.DockerImageReference
	// If specified, a more specific location the image is available at
	ResolvedReference *imageapi.DockerImageReference

	AsResolvedImage bool
	AsImageStream   bool
	OutputImage     bool
	Insecure        bool
	HasEmptyDir     bool

	Env Environment

	// ObjectName overrides the name of the ImageStream produced
	// but does not affect the DockerImageReference
	ObjectName string

	// This should *only* be set if the image stream already exists
	Stream *imageapi.ImageStream
	Info   *imageapi.DockerImage
}

// Exists returns true if the image stream exists
func (r *ImageRef) Exists() bool {
	return r.Stream != nil
}

// ObjectReference returns an object reference from the image reference
func (r *ImageRef) ObjectReference() kapi.ObjectReference {
	switch {
	case r.Stream != nil:
		return kapi.ObjectReference{
			Kind:      "ImageStreamTag",
			Name:      imageapi.JoinImageStreamTag(r.Stream.Name, r.Reference.Tag),
			Namespace: r.Stream.Namespace,
		}
	case r.AsImageStream:
		return kapi.ObjectReference{
			Kind: "ImageStreamTag",
			Name: imageapi.JoinImageStreamTag(r.Reference.Name, r.Reference.Tag),
		}
	default:
		return kapi.ObjectReference{
			Kind: "DockerImage",
			Name: r.PullSpec(),
		}
	}
}

func (r *ImageRef) PullSpec() string {
	if r.AsResolvedImage && r.ResolvedReference != nil {
		return r.ResolvedReference.String()
	}
	return r.Reference.String()
}

// RepoName returns the name of the image in namespace/name format
func (r *ImageRef) RepoName() string {
	name := r.Reference.Namespace
	if len(name) > 0 {
		name += "/"
	}
	name += r.Reference.Name
	return name
}

// SuggestName suggests a name for an image reference
func (r *ImageRef) SuggestName() (string, bool) {
	if r != nil && len(r.ObjectName) > 0 {
		return r.ObjectName, true
	}
	if r == nil || len(r.Reference.Name) == 0 {
		return "", false
	}
	return r.Reference.Name, true
}

// BuildOutput returns the BuildOutput of an image reference
func (r *ImageRef) BuildOutput() (*buildapi.BuildOutput, error) {
	if r == nil {
		return &buildapi.BuildOutput{}, nil
	}
	imageRepo, err := r.ImageStream()
	if err != nil {
		return nil, err
	}
	kind := "ImageStreamTag"
	if !r.AsImageStream {
		kind = "DockerImage"
	}
	return &buildapi.BuildOutput{
		To: &kapi.ObjectReference{
			Kind: kind,
			Name: imageapi.JoinImageStreamTag(imageRepo.Name, r.Reference.Tag),
		},
	}, nil
}

// BuildTriggers sets up build triggers for the base image
func (r *ImageRef) BuildTriggers() []buildapi.BuildTriggerPolicy {
	if r.Stream == nil && !r.AsImageStream {
		return nil
	}
	return []buildapi.BuildTriggerPolicy{
		{
			Type:        buildapi.ImageChangeBuildTriggerType,
			ImageChange: &buildapi.ImageChangeTrigger{},
		},
	}
}

// ImageStream returns an ImageStream from an image reference
func (r *ImageRef) ImageStream() (*imageapi.ImageStream, error) {
	if r.Stream != nil {
		return r.Stream, nil
	}

	name, ok := r.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest an ImageStream name for %q", r.Reference.String())
	}

	stream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
	}
	if !r.OutputImage {
		// Ignore AsResolvedImage here because we are attempting to get images from this location.
		stream.Spec.DockerImageRepository = r.Reference.AsRepository().String()
		if r.Insecure {
			stream.ObjectMeta.Annotations = map[string]string{
				imageapi.InsecureRepositoryAnnotation: "true",
			}
		}
	}

	return stream, nil
}

// DeployableContainer sets up a container for the image ready for deployment
func (r *ImageRef) DeployableContainer() (container *kapi.Container, triggers []deployapi.DeploymentTriggerPolicy, err error) {
	name, ok := r.SuggestName()
	if !ok {
		return nil, nil, fmt.Errorf("unable to suggest a container name for the image %q", r.Reference.String())
	}
	if r.AsImageStream {
		tag := r.Reference.Tag
		if len(tag) == 0 {
			tag = imageapi.DefaultImageTag
		}
		imageChangeParams := &deployapi.DeploymentTriggerImageChangeParams{
			Automatic:      true,
			ContainerNames: []string{name},
			Tag:            tag,
		}
		if r.Stream != nil {
			imageChangeParams.From = kapi.ObjectReference{
				Kind:      "ImageStream",
				Name:      r.Stream.Name,
				Namespace: r.Stream.Namespace,
			}
		} else {
			imageChangeParams.From = kapi.ObjectReference{
				Kind: "ImageStream",
				Name: name,
			}
		}
		triggers = []deployapi.DeploymentTriggerPolicy{
			{
				Type:              deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: imageChangeParams,
			},
		}
	}

	container = &kapi.Container{
		Name:  name,
		Image: r.PullSpec(),
	}

	// If imageInfo present, append ports
	if r.Info != nil && r.Info.Config != nil {
		ports := []string{}
		// ExposedPorts can consist of multiple space-separated ports
		for exposed := range r.Info.Config.ExposedPorts {
			ports = append(ports, strings.Split(exposed, " ")...)
		}

		for _, sp := range ports {
			p := docker.Port(sp)
			port, err := strconv.Atoi(p.Port())
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse port %q: %v", p.Port(), err)
			}

			container.Ports = append(container.Ports, kapi.ContainerPort{
				ContainerPort: port,
				Protocol:      kapi.Protocol(strings.ToUpper(p.Proto())),
			})
		}

		// Create volume mounts with names based on container name
		maxDigits := len(fmt.Sprintf("%d", len(r.Info.Config.Volumes)))
		baseName := namer.GetName(container.Name, volumeNameInfix, kvalidation.LabelValueMaxLength-maxDigits-1)
		i := 1
		for volume := range r.Info.Config.Volumes {
			r.HasEmptyDir = true
			container.VolumeMounts = append(container.VolumeMounts, kapi.VolumeMount{
				Name:      fmt.Sprintf("%s-%d", baseName, i),
				ReadOnly:  false,
				MountPath: volume,
			})
			i++
		}
		// TODO: Append environment variables
	}

	container.Env = append(container.Env, r.Env.List()...)

	return container, triggers, nil
}

func (r *ImageRef) InstallablePod(generatorInput GeneratorInput, secretAccessor SecretAccessor, serviceAccountName string) (*kapi.Pod, *kapi.Secret, error) {
	name, ok := r.SuggestName()
	if !ok {
		return nil, nil, fmt.Errorf("can't suggest a name for the provided image %q", r.Reference.Exact())
	}

	meta := kapi.ObjectMeta{
		Name: fmt.Sprintf("%s-install", name),
	}

	container, _, err := r.DeployableContainer()
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate an installable container: %v", err)
	}
	container.Name = "install"

	// inject the POD_NAMESPACE resolver first
	namespaceEnv := kapi.EnvVar{
		Name: "POD_NAMESPACE",
		ValueFrom: &kapi.EnvVarSource{
			FieldRef: &kapi.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.namespace",
			},
		},
	}
	container.Env = append([]kapi.EnvVar{namespaceEnv}, container.Env...)

	// give installers 4 hours to complete
	deadline := int64(60 * 60 * 4)
	pod := &kapi.Pod{
		ObjectMeta: meta,
		Spec: kapi.PodSpec{
			RestartPolicy:         kapi.RestartPolicyNever,
			ActiveDeadlineSeconds: &deadline,
		},
	}

	var secret *kapi.Secret
	if token := generatorInput.Token; token != nil {
		if token.ServiceAccount {
			pod.Spec.ServiceAccountName = serviceAccountName
		}
		if token.Env != nil {
			containerToken, err := secretAccessor.Token()
			if err != nil {
				return nil, nil, err
			}
			container.Env = append(container.Env, kapi.EnvVar{
				Name:  *token.Env,
				Value: containerToken,
			})
		}
		if token.File != nil {
			containerToken, err := secretAccessor.Token()
			if err != nil {
				return nil, nil, err
			}
			crt, err := secretAccessor.CACert()
			if err != nil {
				return nil, nil, err
			}

			secret = &kapi.Secret{
				ObjectMeta: meta,

				Type: "kubernetes.io/token",
				Data: map[string][]byte{
					kapi.ServiceAccountTokenKey: []byte(containerToken),
				},
			}
			if len(crt) > 0 {
				secret.Data[kapi.ServiceAccountRootCAKey] = []byte(crt)
			}
			pod.Spec.Volumes = append(pod.Spec.Volumes, kapi.Volume{
				Name: "generate-token",
				VolumeSource: kapi.VolumeSource{
					Secret: &kapi.SecretVolumeSource{SecretName: meta.Name},
				},
			})
			container.VolumeMounts = append(container.VolumeMounts, kapi.VolumeMount{
				Name:      "generate-token",
				MountPath: *token.File,
			})
		}
	}

	pod.Spec.Containers = []kapi.Container{*container}
	return pod, secret, nil
}
