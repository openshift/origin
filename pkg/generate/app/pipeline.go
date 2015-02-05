package app

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deploy "github.com/openshift/origin/pkg/deploy/api"
)

type Pipeline struct {
	From string

	InputImage *ImageRef
	Build      *BuildRef
	Image      *ImageRef
	Deployment *DeploymentConfigRef
}

func NewImagePipeline(from string, image *ImageRef) (*Pipeline, error) {
	return &Pipeline{
		From:  from,
		Image: image,
	}, nil
}

func NewBuildPipeline(from string, input *ImageRef, strategy *BuildStrategyRef, source *SourceRef) (*Pipeline, error) {
	pipeline := &Pipeline{
		From: from,
	}

	strategy.Base = input

	name, ok := NameSuggestions{source, input}.SuggestName()
	if !ok {
		name = fmt.Sprintf("app%d", rand.Intn(10000))
	}

	output := &ImageRef{
		Name: name,
		Tag:  "latest",

		AsImageRepository: true,
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
	}

	pipeline.InputImage = input
	pipeline.Image = output
	pipeline.Build = build

	return pipeline, nil
}

func (p *Pipeline) NeedsDeployment(env Environment) error {
	if p.Deployment != nil {
		return nil
	}
	p.Deployment = &DeploymentConfigRef{
		Images: []*ImageRef{
			p.Image,
		},
		Env: env,
	}
	return nil
}

func (p *Pipeline) Objects(accept Acceptor) (Objects, error) {
	objects := Objects{}
	if p.InputImage != nil && p.InputImage.AsImageRepository && accept.Accept(p.InputImage) {
		repo, err := p.InputImage.ImageRepository()
		if err != nil {
			return nil, err
		}
		objects = append(objects, repo)
	}
	if p.Image != nil && p.Image.AsImageRepository && accept.Accept(p.Image) {
		repo, err := p.Image.ImageRepository()
		if err != nil {
			return nil, err
		}
		objects = append(objects, repo)
	}
	if p.Build != nil && accept.Accept(p.Build) {
		build, err := p.Build.BuildConfig()
		if err != nil {
			return nil, err
		}
		objects = append(objects, build)
	}
	if p.Deployment != nil && accept.Accept(p.Deployment) {
		dc, err := p.Deployment.DeploymentConfig()
		if err != nil {
			return nil, err
		}
		objects = append(objects, dc)
	}
	return objects, nil
}

type PipelineGroup []*Pipeline

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

const maxServiceNameLength = 24

var invalidServiceChars = regexp.MustCompile("[^-a-z0-9]")

func makeValidServiceName(name string) (string, string) {
	if ok, _ := validation.ValidateServiceName(name, false); ok {
		return name, ""
	}
	name = strings.ToLower(name)
	name = invalidServiceChars.ReplaceAllString(name, "")
	name = strings.TrimFunc(name, func(r rune) bool { return r == '-' })
	switch {
	case len(name) == 0:
		name = "service-"
	case len(name) > maxServiceNameLength:
		name = name[:maxServiceNameLength]
	}
	return "", name
}

func AddServices(objects Objects) Objects {
	svcs := []runtime.Object{}
	for _, o := range objects {
		switch t := o.(type) {
		case *deploy.DeploymentConfig:
			for _, container := range t.Template.ControllerTemplate.Template.Spec.Containers {
				for _, port := range container.Ports {
					p := port.ContainerPort
					name, generateName := makeValidServiceName(t.Name)
					svcs = append(svcs, &kapi.Service{
						ObjectMeta: kapi.ObjectMeta{
							Name:         name,
							GenerateName: generateName,
						},
						Spec: kapi.ServiceSpec{
							ContainerPort: kutil.NewIntOrStringFromInt(p),
							Port:          p,
							Selector:      t.Template.ControllerTemplate.Selector,
						},
					})
				}
				break
			}
		}
	}
	return append(svcs, objects...)
}

type Objects []runtime.Object

type Acceptor interface {
	Accept(from interface{}) bool
}

type acceptFirst struct {
	handled map[interface{}]struct{}
}

func NewAcceptFirst() Acceptor {
	return &acceptFirst{make(map[interface{}]struct{})}
}

func (s *acceptFirst) Accept(from interface{}) bool {
	if _, ok := s.handled[from]; ok {
		return false
	}
	s.handled[from] = struct{}{}
	return true
}
