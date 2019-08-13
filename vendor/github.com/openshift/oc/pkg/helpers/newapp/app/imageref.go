package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/parser"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/oc/pkg/helpers/newapp/docker/dockerfile"
	"github.com/openshift/oc/pkg/helpers/newapp/portutils"
)

// ImageRefGenerator is an interface for generating ImageRefs
//
// Generators for ImageRef
// - Name              -> ImageRef
// - ImageRepo + tag   -> ImageRef
type ImageRefGenerator interface {
	FromName(name string) (*ImageRef, error)
	FromNameAndPorts(name string, ports []string) (*ImageRef, error)
	FromStream(repo *imagev1.ImageStream, tag string) (*ImageRef, error)
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
	ref, err := reference.Parse(name)
	if err != nil {
		return nil, err
	}
	return &ImageRef{
		Reference: ref,
		Info: &dockerv10.DockerImage{
			Config: &dockerv10.DockerConfig{},
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

	imageRef.Info = &dockerv10.DockerImage{
		Config: &dockerv10.DockerConfig{
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
	defer file.Close()
	node, err := parser.Parse(file)
	if err != nil {
		return nil, err
	}
	ports := dockerfile.LastExposedPorts(node.AST)

	return g.FromNameAndPorts(name, ports)
}

// FromStream generates an ImageRef from an OpenShift ImageStream
func (g *imageRefGenerator) FromStream(stream *imagev1.ImageStream, tag string) (*ImageRef, error) {
	imageRef := &ImageRef{
		Stream: stream,
	}

	if tagged := imageutil.LatestTaggedImage(stream, tag); tagged != nil {
		if ref, err := reference.Parse(tagged.DockerImageReference); err == nil {
			imageRef.ResolvedReference = &ref
			imageRef.Reference = ref
		}
	}

	if pullSpec := stream.Status.DockerImageRepository; len(pullSpec) != 0 {
		ref, err := reference.Parse(pullSpec)
		if err != nil {
			return nil, err
		}
		imageRef.Reference = ref
	}
	switch {
	case len(tag) > 0:
		imageRef.Reference.Tag = tag
	case len(tag) == 0 && len(imageRef.Reference.Tag) == 0:
		imageRef.Reference.Tag = imagev1.DefaultImageTag
	}

	return imageRef, nil
}

// ImageRef is a reference to an image
type ImageRef struct {
	Reference reference.DockerImageReference
	// If specified, a more specific location the image is available at
	ResolvedReference *reference.DockerImageReference

	AsResolvedImage bool
	AsImageStream   bool
	OutputImage     bool
	Insecure        bool
	HasEmptyDir     bool
	// TagDirectly will create the image stream using a tag for this reference, not a bulk
	// import.
	TagDirectly bool
	// Tag defines tag that other components will reference this image by if set. Must be
	// set with TagDirectly (otherwise tag remapping is not possible).
	Tag string
	// InternalDefaultTag is the default tag for other components that reference this image
	InternalDefaultTag string
	// Env represents a set of additional environment to add to this image.
	Env Environment
	// ObjectName overrides the name of the ImageStream produced
	// but does not affect the DockerImageReference
	ObjectName string

	// ContainerFn overrides normal container generation with a custom function.
	ContainerFn func(*corev1.Container)

	// Stream and Info should *only* be set if the image stream already exists
	Stream *imagev1.ImageStream
	Info   *dockerv10.DockerImage
}

// Exists returns true if the image stream exists
func (r *ImageRef) Exists() bool {
	return r.Stream != nil
}

// ObjectReference returns an object reference to this ref (as it would exist during generation)
func (r *ImageRef) ObjectReference() corev1.ObjectReference {
	switch {
	case r.Stream != nil:
		return corev1.ObjectReference{
			Kind:      "ImageStreamTag",
			Name:      imageutil.JoinImageStreamTag(r.Stream.Name, r.Reference.Tag),
			Namespace: r.Stream.Namespace,
		}
	case r.AsImageStream:
		name, _ := r.SuggestName()
		return corev1.ObjectReference{
			Kind: "ImageStreamTag",
			Name: imageutil.JoinImageStreamTag(name, r.InternalTag()),
		}
	default:
		return corev1.ObjectReference{
			Kind: "DockerImage",
			Name: r.PullSpec(),
		}
	}
}

func (r *ImageRef) InternalTag() string {
	tag := r.Tag
	if len(tag) == 0 {
		tag = r.Reference.Tag
	}
	if len(tag) == 0 {
		tag = r.InternalDefaultTag
	}
	if len(tag) == 0 {
		tag = imagev1.DefaultImageTag
	}
	return tag
}

func (r *ImageRef) PullSpec() string {
	if r.AsResolvedImage && r.ResolvedReference != nil {
		return r.ResolvedReference.Exact()
	}
	return r.Reference.Exact()
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
	if r == nil {
		return "", false
	}
	if len(r.ObjectName) > 0 {
		return r.ObjectName, true
	}
	if r.Stream != nil {
		return r.Stream.Name, true
	}
	if len(r.Reference.Name) > 0 {
		return r.Reference.Name, true
	}
	return "", false
}

// SuggestNamespace suggests a namespace for an image reference
func (r *ImageRef) SuggestNamespace() string {
	if r == nil {
		return ""
	}
	if len(r.ObjectName) > 0 {
		return ""
	}
	if r.Stream != nil {
		return r.Stream.Namespace
	}
	return ""
}

// BuildOutput returns the BuildOutput of an image reference
func (r *ImageRef) BuildOutput() (*buildv1.BuildOutput, error) {
	if r == nil {
		return &buildv1.BuildOutput{}, nil
	}
	if !r.AsImageStream {
		return &buildv1.BuildOutput{
			To: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: r.Reference.String(),
			},
		}, nil
	}
	imageRepo, err := r.ImageStream()
	if err != nil {
		return nil, err
	}
	return &buildv1.BuildOutput{
		To: &corev1.ObjectReference{
			Kind: "ImageStreamTag",
			Name: imageutil.JoinImageStreamTag(imageRepo.Name, r.Reference.Tag),
		},
	}, nil
}

// BuildTriggers sets up build triggers for the base image
func (r *ImageRef) BuildTriggers() []buildv1.BuildTriggerPolicy {
	if r.Stream == nil && !r.AsImageStream {
		return nil
	}
	return []buildv1.BuildTriggerPolicy{
		{
			Type:        buildv1.ImageChangeBuildTriggerType,
			ImageChange: &buildv1.ImageChangeTrigger{},
		},
	}
}

// ImageStream returns an ImageStream from an image reference
func (r *ImageRef) ImageStream() (*imagev1.ImageStream, error) {
	if r.Stream != nil {
		return r.Stream, nil
	}

	name, ok := r.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest an ImageStream name for %q", r.Reference.String())
	}

	stream := &imagev1.ImageStream{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta: metav1.TypeMeta{APIVersion: imagev1.SchemeGroupVersion.String(), Kind: "ImageStream"},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if r.OutputImage {
		return stream, nil
	}

	// Legacy path, talking to a server that cannot do granular import of exact image stream spec tags.
	if !r.TagDirectly {
		// Ignore AsResolvedImage here because we are attempting to get images from this location.
		stream.Spec.DockerImageRepository = r.Reference.AsRepository().String()
		if r.Insecure {
			stream.ObjectMeta.Annotations = map[string]string{
				imagev1.InsecureRepositoryAnnotation: "true",
			}
		}
		return stream, nil
	}

	if stream.Spec.Tags == nil {
		stream.Spec.Tags = []imagev1.TagReference{}
	}

	stream.Spec.Tags = append(stream.Spec.Tags, imagev1.TagReference{
		Name: r.InternalTag(),
		// Make this a constant
		Annotations: map[string]string{"openshift.io/imported-from": r.Reference.Exact()},
		From: &corev1.ObjectReference{
			Kind: "DockerImage",
			Name: r.PullSpec(),
		},
		ImportPolicy: imagev1.TagImportPolicy{Insecure: r.Insecure},
	})

	return stream, nil
}

// ImageStreamTag returns an ImageStreamTag from an image reference
func (r *ImageRef) ImageStreamTag() (*imagev1.ImageStreamTag, error) {
	name, ok := r.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest an ImageStream name for %q", r.Reference.String())
	}
	istname := imageutil.JoinImageStreamTag(name, r.Reference.Tag)
	ist := &imagev1.ImageStreamTag{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta: metav1.TypeMeta{APIVersion: imagev1.SchemeGroupVersion.String(), Kind: "ImageStreamTag"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        istname,
			Namespace:   r.SuggestNamespace(),
			Annotations: map[string]string{"openshift.io/imported-from": r.Reference.Exact()},
		},
		Tag: &imagev1.TagReference{
			Name: r.InternalTag(),
			From: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: r.PullSpec(),
			},
			ImportPolicy: imagev1.TagImportPolicy{Insecure: r.Insecure},
		},
	}
	return ist, nil
}

// DeployableContainer sets up a container for the image ready for deployment
func (r *ImageRef) DeployableContainer() (container *corev1.Container, triggers []appsv1.DeploymentTriggerPolicy, err error) {
	name, ok := r.SuggestName()
	if !ok {
		return nil, nil, fmt.Errorf("unable to suggest a container name for the image %q", r.Reference.String())
	}
	if r.AsImageStream {
		triggers = []appsv1.DeploymentTriggerPolicy{
			{
				Type: appsv1.DeploymentTriggerOnImageChange,
				ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{name},
					From:           r.ObjectReference(),
				},
			},
		}
	}

	container = &corev1.Container{
		Name:  name,
		Image: r.PullSpec(),
		Env:   r.Env.List(),
	}

	if r.ContainerFn != nil {
		r.ContainerFn(container)
		return container, triggers, nil
	}

	// If imageInfo present, append ports
	if r.Info != nil && r.Info.Config != nil {
		ports := []string{}
		// ExposedPorts can consist of multiple space-separated ports
		for exposed := range r.Info.Config.ExposedPorts {
			ports = append(ports, strings.Split(exposed, " ")...)
		}

		dockerPorts, _ := portutils.FilterPortAndProtocolArray(ports)
		for _, dp := range dockerPorts {
			intPort, _ := strconv.Atoi(dp.Port())
			container.Ports = append(container.Ports, corev1.ContainerPort{
				ContainerPort: int32(intPort),
				Protocol:      corev1.Protocol(strings.ToUpper(dp.Proto())),
			})
		}

		// Create volume mounts with names based on container name
		maxDigits := len(fmt.Sprintf("%d", len(r.Info.Config.Volumes)))
		baseName := naming.GetName(container.Name, volumeNameInfix, kvalidation.LabelValueMaxLength-maxDigits-1)
		i := 1
		for volume := range r.Info.Config.Volumes {
			r.HasEmptyDir = true
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      fmt.Sprintf("%s-%d", baseName, i),
				ReadOnly:  false,
				MountPath: volume,
			})
			i++
		}
	}

	return container, triggers, nil
}

func (r *ImageRef) InstallablePod(generatorInput GeneratorInput, secretAccessor SecretAccessor, serviceAccountName string) (*corev1.Pod, *corev1.Secret, error) {
	name, ok := r.SuggestName()
	if !ok {
		return nil, nil, fmt.Errorf("can't suggest a name for the provided image %q", r.Reference.Exact())
	}

	meta := metav1.ObjectMeta{
		Name: fmt.Sprintf("%s-install", name),
	}

	container, _, err := r.DeployableContainer()
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate an installable container: %v", err)
	}
	container.Name = "install"

	// inject the POD_NAMESPACE resolver first
	namespaceEnv := corev1.EnvVar{
		Name: "POD_NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.namespace",
			},
		},
	}
	container.Env = append([]corev1.EnvVar{namespaceEnv}, container.Env...)

	// give installers 4 hours to complete
	deadline := int64(60 * 60 * 4)
	pod := &corev1.Pod{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta:   metav1.TypeMeta{APIVersion: metav1.SchemeGroupVersion.String(), Kind: "Pod"},
		ObjectMeta: meta,
		Spec: corev1.PodSpec{
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: &deadline,
		},
	}

	var secret *corev1.Secret
	if token := generatorInput.Token; token != nil {
		if token.ServiceAccount {
			pod.Spec.ServiceAccountName = serviceAccountName
		}
		if token.Env != nil {
			containerToken, err := secretAccessor.Token()
			if err != nil {
				return nil, nil, err
			}
			container.Env = append(container.Env, corev1.EnvVar{
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

			secret = &corev1.Secret{
				// this is ok because we know exactly how we want to be serialized
				TypeMeta:   metav1.TypeMeta{APIVersion: metav1.SchemeGroupVersion.String(), Kind: "Secret"},
				ObjectMeta: meta,

				Type: "kubernetes.io/token",
				Data: map[string][]byte{
					corev1.ServiceAccountTokenKey: []byte(containerToken),
				},
			}
			if len(crt) > 0 {
				secret.Data[corev1.ServiceAccountRootCAKey] = []byte(crt)
			}
			pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
				Name: "generate-token",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: meta.Name},
				},
			})
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      "generate-token",
				MountPath: *token.File,
			})
		}
	}

	pod.Spec.Containers = []corev1.Container{*container}
	return pod, secret, nil
}
