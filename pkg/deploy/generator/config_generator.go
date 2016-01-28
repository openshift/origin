package generator

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation/field"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DeploymentConfigGenerator reconciles a DeploymentConfig with other pieces of deployment-related state
// and produces a DeploymentConfig which represents a potential future DeploymentConfig. If the generated
// state differs from the input state, the LatestVersion field of the output is incremented.
type DeploymentConfigGenerator struct {
	Client GeneratorClient
}

// Generate returns a potential future DeploymentConfig based on the DeploymentConfig specified
// by namespace and name. Returns a RESTful error.
func (g *DeploymentConfigGenerator) Generate(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
	config, err := g.Client.GetDeploymentConfig(ctx, name)
	if err != nil {
		return nil, err
	}

	// Update the containers with new images based on defined triggers
	configChanged := false
	errs := field.ErrorList{}
	causes := []*deployapi.DeploymentCause{}
	for i, trigger := range config.Spec.Triggers {
		params := trigger.ImageChangeParams

		// Only process image change triggers
		if trigger.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		name, tag, ok := imageapi.SplitImageStreamTag(params.From.Name)
		if !ok {
			f := field.NewPath("triggers").Index(i).Child("imageChange", "from")
			errs = append(errs, field.Invalid(f, name, err.Error()))
			continue
		}

		// Find the image repo referred to by the trigger params
		imageStream, err := g.findImageStream(config, params)
		if err != nil {
			f := field.NewPath("triggers").Index(i).Child("imageChange", "from")
			errs = append(errs, field.Invalid(f, name, err.Error()))
			continue
		}

		// Find the latest tag event for the trigger tag
		latestEvent := imageapi.LatestTaggedImage(imageStream, tag)
		if latestEvent == nil {
			f := field.NewPath("triggers").Index(i).Child("imageChange", "tag")
			errs = append(errs, field.Invalid(f, tag, fmt.Sprintf("no image recorded for %s/%s:%s", imageStream.Namespace, imageStream.Name, tag)))
			continue
		}

		// Update containers
		template := config.Spec.Template
		names := sets.NewString(params.ContainerNames...)
		containerChanged := false
		for i := range template.Spec.Containers {
			container := &template.Spec.Containers[i]
			if !names.Has(container.Name) {
				continue
			}
			if len(latestEvent.DockerImageReference) > 0 &&
				container.Image != latestEvent.DockerImageReference {
				// Update the image
				container.Image = latestEvent.DockerImageReference
				// Log the last triggered image ID
				params.LastTriggeredImage = latestEvent.DockerImageReference
				containerChanged = true
			}
		}

		// If any container was updated, create a cause for the change
		if containerChanged {
			configChanged = true
			causes = append(causes,
				&deployapi.DeploymentCause{
					Type: deployapi.DeploymentTriggerOnImageChange,
					ImageTrigger: &deployapi.DeploymentCauseImageTrigger{
						From: kapi.ObjectReference{
							Name: imageapi.JoinImageStreamTag(imageStream.Name, tag),
							Kind: "ImageStreamTag",
						},
					},
				})
		}
	}

	if len(errs) > 0 {
		return nil, errors.NewInvalid(deployapi.Kind("DeploymentConfig"), config.Name, errs)
	}

	// Bump the version if we updated containers or if this is an initial
	// deployment
	if configChanged || config.Status.LatestVersion == 0 {
		config.Status.Details = &deployapi.DeploymentDetails{
			Causes: causes,
		}
		config.Status.LatestVersion++
	}

	return config, nil
}

func (g *DeploymentConfigGenerator) findImageStream(config *deployapi.DeploymentConfig, params *deployapi.DeploymentTriggerImageChangeParams) (*imageapi.ImageStream, error) {
	if len(params.From.Name) > 0 {
		namespace := params.From.Namespace
		if len(namespace) == 0 {
			namespace = config.Namespace
		}
		name, _, ok := imageapi.SplitImageStreamTag(params.From.Name)
		if !ok {
			return nil, fmt.Errorf("invalid ImageStreamTag: %s", params.From.Name)
		}
		return g.Client.GetImageStream(kapi.WithNamespace(kapi.NewContext(), namespace), name)
	}
	return nil, fmt.Errorf("couldn't find image stream for config %s trigger params", deployutil.LabelForDeploymentConfig(config))
}

type GeneratorClient interface {
	GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error)
	GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
	// LEGACY: used, to scan all repositories for a DockerImageReference.  Will be removed
	// when we drop support for reference by DockerImageReference.
	ListImageStreams(ctx kapi.Context) (*imageapi.ImageStreamList, error)
}

type Client struct {
	DCFn   func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error)
	ISFn   func(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
	LISFn  func(ctx kapi.Context) (*imageapi.ImageStreamList, error)
	LISFn2 func(ctx kapi.Context, options *kapi.ListOptions) (*imageapi.ImageStreamList, error)
}

func (c Client) GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
	return c.DCFn(ctx, name)
}
func (c Client) GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
	return c.ISFn(ctx, name)
}
func (c Client) ListImageStreams(ctx kapi.Context) (*imageapi.ImageStreamList, error) {
	if c.LISFn2 != nil {
		return c.LISFn2(ctx, &kapi.ListOptions{})
	}
	return c.LISFn(ctx)
}
