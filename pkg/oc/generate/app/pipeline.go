package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	kuval "k8s.io/apimachinery/pkg/util/validation"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	extensions "k8s.io/kubernetes/pkg/apis/extensions"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/oc/generate"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/util/docker/dockerfile"
)

// A PipelineBuilder creates Pipeline instances.
type PipelineBuilder interface {
	To(string) PipelineBuilder

	NewBuildPipeline(string, *ImageRef, *SourceRepository, bool) (*Pipeline, error)
	NewImagePipeline(string, *ImageRef) (*Pipeline, error)
}

// NewPipelineBuilder returns a PipelineBuilder using name as a base name. A
// PipelineBuilder always creates pipelines with unique names, so that the
// actual name of a pipeline (Pipeline.Name) might differ from the base name.
// The pipelines created with a PipelineBuilder will have access to the given
// environment. The boolean outputDocker controls whether builds will output to
// an image stream tag or docker image reference.
func NewPipelineBuilder(name string, environment Environment, dockerStrategyOptions *buildapi.DockerStrategyOptions, outputDocker bool) PipelineBuilder {
	return &pipelineBuilder{
		nameGenerator:         NewUniqueNameGenerator(name),
		environment:           environment,
		outputDocker:          outputDocker,
		dockerStrategyOptions: dockerStrategyOptions,
	}
}

type pipelineBuilder struct {
	nameGenerator         UniqueNameGenerator
	environment           Environment
	outputDocker          bool
	to                    string
	dockerStrategyOptions *buildapi.DockerStrategyOptions
}

func (pb *pipelineBuilder) To(name string) PipelineBuilder {
	pb.to = name
	return pb
}

// NewBuildPipeline creates a new pipeline with components that are expected to
// be built.
func (pb *pipelineBuilder) NewBuildPipeline(from string, input *ImageRef, sourceRepository *SourceRepository, binary bool) (*Pipeline, error) {
	strategy, source, err := StrategyAndSourceForRepository(sourceRepository, input)
	if err != nil {
		return nil, fmt.Errorf("can't build %q: %v", from, err)
	}

	var name string
	output := &ImageRef{
		OutputImage:   true,
		AsImageStream: !pb.outputDocker,
	}
	if len(pb.to) > 0 {
		outputImageRef, err := imageapi.ParseDockerImageReference(pb.to)
		if err != nil {
			return nil, err
		}
		output.Reference = outputImageRef
		name, err = pb.nameGenerator.Generate(NameSuggestions{source, output, input})
		if err != nil {
			return nil, err
		}
	} else {
		name, err = pb.nameGenerator.Generate(NameSuggestions{source, input})
		if err != nil {
			return nil, err
		}
		output.Reference = imageapi.DockerImageReference{
			Name: name,
			Tag:  imageapi.DefaultImageTag,
		}
	}
	source.Name = name

	// Append any exposed ports from Dockerfile to input image
	if sourceRepository.GetStrategy() == generate.StrategyDocker && sourceRepository.Info() != nil {
		node := sourceRepository.Info().Dockerfile.AST()
		ports := dockerfile.LastExposedPorts(node)
		if len(ports) > 0 {
			if input.Info == nil {
				input.Info = &imageapi.DockerImage{
					Config: &imageapi.DockerConfig{},
				}
			}
			input.Info.Config.ExposedPorts = map[string]struct{}{}
			for _, p := range ports {
				input.Info.Config.ExposedPorts[p] = struct{}{}
			}
		}
	}

	if input != nil {
		// TODO: assumes that build doesn't change the image metadata. In the future
		// we could get away with deferred generation possibly.
		output.Info = input.Info
	}

	build := &BuildRef{
		Source:   source,
		Input:    input,
		Strategy: strategy,
		Output:   output,
		Env:      pb.environment,
		DockerStrategyOptions: pb.dockerStrategyOptions,
		Binary:                binary,
	}

	return &Pipeline{
		Name:       name,
		From:       from,
		InputImage: input,
		Image:      output,
		Build:      build,
	}, nil
}

// NewImagePipeline creates a new pipeline with components that are not expected
// to be built.
func (pb *pipelineBuilder) NewImagePipeline(from string, input *ImageRef) (*Pipeline, error) {
	name, err := pb.nameGenerator.Generate(input)
	if err != nil {
		return nil, err
	}
	input.ObjectName = name

	return &Pipeline{
		Name:  name,
		From:  from,
		Image: input,
	}, nil
}

// Pipeline holds components.
type Pipeline struct {
	Name string
	From string

	InputImage *ImageRef
	Build      *BuildRef
	Image      *ImageRef
	Deployment *DeploymentConfigRef
	Labels     map[string]string
}

// NeedsDeployment sets the pipeline for deployment.
func (p *Pipeline) NeedsDeployment(env Environment, labels map[string]string, asTest bool) error {
	if p.Deployment != nil {
		return nil
	}
	p.Deployment = &DeploymentConfigRef{
		Name: p.Name,
		Images: []*ImageRef{
			p.Image,
		},
		Env:    env,
		Labels: labels,
		AsTest: asTest,
	}
	return nil
}

// Objects converts all the components in the pipeline into runtime objects.
func (p *Pipeline) Objects(accept, objectAccept Acceptor) (Objects, error) {
	objects := Objects{}
	if p.InputImage != nil && p.InputImage.AsImageStream && accept.Accept(p.InputImage) {
		repo, err := p.InputImage.ImageStream()
		if err != nil {
			return nil, err
		}
		if objectAccept.Accept(repo) {
			objects = append(objects, repo)
		}
	}
	if p.Image != nil && p.Image.AsImageStream && accept.Accept(p.Image) {
		repo, err := p.Image.ImageStream()
		if err != nil {
			return nil, err
		}
		if objectAccept.Accept(repo) {
			objects = append(objects, repo)
		} else {
			// if the image stream exists, if possible create the imagestream tag referenced if that does not exist
			tag, err := p.Image.ImageStreamTag()
			if err != nil {
				return nil, err
			}
			if objectAccept.Accept(tag) {
				objects = append(objects, tag)
			}
		}
	}
	if p.Build != nil && accept.Accept(p.Build) {
		build, err := p.Build.BuildConfig()
		if err != nil {
			return nil, err
		}
		if objectAccept.Accept(build) {
			objects = append(objects, build)
		}
		if p.Build.Source != nil && p.Build.Source.SourceImage != nil && p.Build.Source.SourceImage.AsImageStream && accept.Accept(p.Build.Source.SourceImage) {
			srcImage, err := p.Build.Source.SourceImage.ImageStream()
			if err != nil {
				return nil, err
			}
			if objectAccept.Accept(srcImage) {
				objects = append(objects, srcImage)
			}
		}
	}
	if p.Deployment != nil && accept.Accept(p.Deployment) {
		dc, err := p.Deployment.DeploymentConfig()
		if err != nil {
			return nil, err
		}
		if objectAccept.Accept(dc) {
			objects = append(objects, dc)
		}
	}
	return objects, nil
}

// PipelineGroup is a group of Pipelines.
type PipelineGroup []*Pipeline

// Reduce squashes all common components from the pipelines.
func (g PipelineGroup) Reduce() error {
	var deployment *DeploymentConfigRef
	for _, p := range g {
		if p.Deployment == nil || p.Deployment == deployment {
			continue
		}
		if deployment == nil {
			deployment = p.Deployment
		} else {
			deployment.Images = append(deployment.Images, p.Deployment.Images...)
			deployment.Env = NewEnvironment(deployment.Env, p.Deployment.Env)
			p.Deployment = deployment
		}
	}
	return nil
}

func (g PipelineGroup) String() string {
	s := []string{}
	for _, p := range g {
		s = append(s, p.From)
	}
	return strings.Join(s, "+")
}

// MakeSimpleName strips any non-alphanumeric characters out of a string and returns
// either an empty string or a string which is valid for most Kubernetes resources.
func MakeSimpleName(name string) string {
	name = strings.ToLower(name)
	name = invalidServiceChars.ReplaceAllString(name, "")
	name = strings.TrimFunc(name, func(r rune) bool { return r == '-' })
	if len(name) > kuval.DNS1035LabelMaxLength {
		name = name[:kuval.DNS1035LabelMaxLength]
	}
	return name
}

var invalidServiceChars = regexp.MustCompile("[^-a-z0-9]")

func makeValidServiceName(name string) (string, string) {
	if len(validation.ValidateServiceName(name, false)) == 0 {
		return name, ""
	}
	name = MakeSimpleName(name)
	if len(name) == 0 {
		return "", "service-"
	}
	return name, ""
}

type sortablePorts []kapi.ContainerPort

func (s sortablePorts) Len() int      { return len(s) }
func (s sortablePorts) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortablePorts) Less(i, j int) bool {
	return s[i].ContainerPort < s[j].ContainerPort
}

// portName returns a unique key for the given port and protocol which can be
// used as a service port name.
func portName(port int, protocol kapi.Protocol) string {
	if protocol == "" {
		protocol = kapi.ProtocolTCP
	}
	return strings.ToLower(fmt.Sprintf("%d-%s", port, protocol))
}

// GenerateService creates a simple service for the provided elements.
func GenerateService(meta metav1.ObjectMeta, selector map[string]string) *kapi.Service {
	name, generateName := makeValidServiceName(meta.Name)
	svc := &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			GenerateName: generateName,
			Labels:       meta.Labels,
		},
		Spec: kapi.ServiceSpec{
			Selector: selector,
		},
	}
	return svc
}

// AllContainerPorts creates a sorted list of all ports in all provided containers.
func AllContainerPorts(containers ...kapi.Container) []kapi.ContainerPort {
	var ports []kapi.ContainerPort
	for _, container := range containers {
		ports = append(ports, container.Ports...)
	}
	sort.Sort(sortablePorts(ports))
	return ports
}

// UniqueContainerToServicePorts creates one service port for each unique container port.
func UniqueContainerToServicePorts(ports []kapi.ContainerPort) []kapi.ServicePort {
	var result []kapi.ServicePort
	svcPorts := map[string]struct{}{}
	for _, p := range ports {
		name := portName(int(p.ContainerPort), p.Protocol)
		_, exists := svcPorts[name]
		if exists {
			continue
		}
		svcPorts[name] = struct{}{}
		result = append(result, kapi.ServicePort{
			Name:       name,
			Port:       p.ContainerPort,
			Protocol:   p.Protocol,
			TargetPort: intstr.FromInt(int(p.ContainerPort)),
		})
	}
	return result
}

// AddServices sets up services for the provided objects.
func AddServices(objects Objects, firstPortOnly bool) Objects {
	svcs := []runtime.Object{}
	for _, o := range objects {
		switch t := o.(type) {
		case *appsapi.DeploymentConfig:
			svc := addServiceInternal(t.Spec.Template.Spec.Containers, t.ObjectMeta, t.Spec.Selector, firstPortOnly)
			if svc != nil {
				svcs = append(svcs, svc)
			}
		case *extensions.DaemonSet:
			svc := addServiceInternal(t.Spec.Template.Spec.Containers, t.ObjectMeta, t.Spec.Template.Labels, firstPortOnly)
			if svc != nil {
				svcs = append(svcs, svc)
			}
		}
	}
	return append(objects, svcs...)
}

// addServiceInternal utility used by AddServices to create services for multiple types.
func addServiceInternal(containers []kapi.Container, objectMeta metav1.ObjectMeta, selector map[string]string, firstPortOnly bool) *kapi.Service {
	ports := UniqueContainerToServicePorts(AllContainerPorts(containers...))
	if len(ports) == 0 {
		return nil
	}
	if firstPortOnly {
		ports = ports[:1]
	}
	svc := GenerateService(objectMeta, selector)
	svc.Spec.Ports = ports
	return svc
}

// AddRoutes sets up routes for the provided objects.
func AddRoutes(objects Objects) Objects {
	routes := []runtime.Object{}
	for _, o := range objects {
		switch t := o.(type) {
		case *kapi.Service:
			routes = append(routes, &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:   t.Name,
					Labels: t.Labels,
				},
				Spec: routeapi.RouteSpec{
					To: routeapi.RouteTargetReference{
						Name: t.Name,
					},
				},
			})
		}
	}
	return append(objects, routes...)
}

type acceptNew struct{}

// AcceptNew only accepts runtime.Objects with an empty resource version.
var AcceptNew Acceptor = acceptNew{}

// Accept accepts any kind of object.
func (acceptNew) Accept(from interface{}) bool {
	_, meta, err := objectMetaData(from)
	if err != nil {
		return false
	}
	if len(meta.GetResourceVersion()) > 0 {
		return false
	}
	return true
}

type acceptUnique struct {
	typer   runtime.ObjectTyper
	objects map[string]struct{}
}

// Accept accepts any kind of object it hasn't accepted before.
func (a *acceptUnique) Accept(from interface{}) bool {
	obj, meta, err := objectMetaData(from)
	if err != nil {
		return false
	}
	gvk, _, err := a.typer.ObjectKinds(obj)
	if err != nil {
		return false
	}
	key := fmt.Sprintf("%s/%s/%s", gvk[0].Kind, meta.GetNamespace(), meta.GetName())
	_, exists := a.objects[key]
	if exists {
		return false
	}
	a.objects[key] = struct{}{}
	return true
}

// NewAcceptUnique creates an acceptor that only accepts unique objects by kind
// and name.
func NewAcceptUnique(typer runtime.ObjectTyper) Acceptor {
	return &acceptUnique{
		typer:   typer,
		objects: map[string]struct{}{},
	}
}

type acceptNonExistentImageStream struct {
	typer     runtime.ObjectTyper
	getter    imageclient.ImageInterface
	namespace string
}

// Accept accepts any non-ImageStream object or an ImageStream that does
// not exist in the api server
func (a *acceptNonExistentImageStream) Accept(from interface{}) bool {
	obj, _, err := objectMetaData(from)
	if err != nil {
		return false
	}
	gvk, _, err := a.typer.ObjectKinds(obj)
	if err != nil {
		return false
	}
	gk := gvk[0].GroupKind()
	if !(imageapi.Kind("ImageStream") == gk || imageapi.LegacyKind("ImageStream") == gk) {
		return true
	}
	is, ok := from.(*imageapi.ImageStream)
	if !ok {
		glog.V(4).Infof("type cast to image stream %#v not right for an unanticipated reason", from)
		return true
	}
	imgstrm, err := a.getter.ImageStreams(a.namespace).Get(is.Name, metav1.GetOptions{})
	if err == nil && imgstrm != nil {
		glog.V(4).Infof("acceptor determined that imagestream %s in namespace %s exists so don't accept: %#v", is.Name, a.namespace, imgstrm)
		return false
	}
	return true
}

// NewAcceptNonExistentImageStream creates an acceptor that accepts an object
// if it is either a) not an ImageStream, or b) or an ImageStream which does not
// yet exist in master
func NewAcceptNonExistentImageStream(typer runtime.ObjectTyper, getter imageclient.ImageInterface, namespace string) Acceptor {
	return &acceptNonExistentImageStream{
		typer:     typer,
		getter:    getter,
		namespace: namespace,
	}
}

type acceptNonExistentImageStreamTag struct {
	typer     runtime.ObjectTyper
	getter    imageclient.ImageInterface
	namespace string
}

// Accept accepts any non-ImageStreamTag object or an ImageStreamTag that does
// not exist in the api server
func (a *acceptNonExistentImageStreamTag) Accept(from interface{}) bool {
	obj, _, err := objectMetaData(from)
	if err != nil {
		return false
	}
	gvk, _, err := a.typer.ObjectKinds(obj)
	if err != nil {
		return false
	}
	gk := gvk[0].GroupKind()
	if !(imageapi.Kind("ImageStreamTag") == gk || imageapi.LegacyKind("ImageStreamTag") == gk) {
		return true
	}
	ist, ok := from.(*imageapi.ImageStreamTag)
	if !ok {
		glog.V(4).Infof("type cast to imagestreamtag %#v not right for an unanticipated reason", from)
		return true
	}
	tag, err := a.getter.ImageStreamTags(a.namespace).Get(ist.Name, metav1.GetOptions{})
	if err == nil && tag != nil {
		glog.V(4).Infof("acceptor determined that imagestreamtag %s in namespace %s exists so don't accept", ist.Name, a.namespace)
		return false
	}
	return true
}

// NewAcceptNonExistentImageStreamTag creates an acceptor that accepts an object
// if it is either a) not an ImageStreamTag, or b) or an ImageStreamTag which does not
// yet exist in master
func NewAcceptNonExistentImageStreamTag(typer runtime.ObjectTyper, getter imageclient.ImageInterface, namespace string) Acceptor {
	return &acceptNonExistentImageStreamTag{
		typer:     typer,
		getter:    getter,
		namespace: namespace,
	}
}

func objectMetaData(raw interface{}) (runtime.Object, metav1.Object, error) {
	obj, ok := raw.(runtime.Object)
	if !ok {
		return nil, nil, fmt.Errorf("%#v is not a runtime.Object", raw)
	}
	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, nil, err
	}
	return obj, meta, nil
}

type acceptBuildConfigs struct {
	typer runtime.ObjectTyper
}

// Accept accepts BuildConfigs and ImageStreams.
func (a *acceptBuildConfigs) Accept(from interface{}) bool {
	obj, _, err := objectMetaData(from)
	if err != nil {
		return false
	}
	gvk, _, err := a.typer.ObjectKinds(obj)
	if err != nil {
		return false
	}
	gk := gvk[0].GroupKind()
	return buildapi.Kind("BuildConfig") == gk || imageapi.Kind("ImageStream") == gk
}

// NewAcceptBuildConfigs creates an acceptor accepting BuildConfig objects
// and ImageStreams objects.
func NewAcceptBuildConfigs(typer runtime.ObjectTyper) Acceptor {
	return &acceptBuildConfigs{
		typer: typer,
	}
}

// Acceptors is a list of acceptors that behave like a single acceptor.
// All acceptors must accept an object for it to be accepted.
type Acceptors []Acceptor

// Accept iterates through all acceptors and determines whether the object
// should be accepted.
func (aa Acceptors) Accept(from interface{}) bool {
	for _, a := range aa {
		if !a.Accept(from) {
			return false
		}
	}
	return true
}

type acceptAll struct{}

// AcceptAll accepts all objects.
var AcceptAll Acceptor = acceptAll{}

// Accept accepts everything.
func (acceptAll) Accept(_ interface{}) bool {
	return true
}

// Objects is a set of runtime objects.
type Objects []runtime.Object

// Acceptor is an interface for accepting objects.
type Acceptor interface {
	Accept(from interface{}) bool
}

type acceptFirst struct {
	handled map[interface{}]struct{}
}

// NewAcceptFirst returns a new Acceptor.
func NewAcceptFirst() Acceptor {
	return &acceptFirst{make(map[interface{}]struct{})}
}

// Accept accepts any object it hasn't accepted before.
func (s *acceptFirst) Accept(from interface{}) bool {
	if _, ok := s.handled[from]; ok {
		return false
	}
	s.handled[from] = struct{}{}
	return true
}
