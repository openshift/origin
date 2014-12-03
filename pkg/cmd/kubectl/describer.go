package kubectl

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
)

func DescriberFor(kind string, c *client.Client, cmd *cobra.Command) (kctl.Describer, bool) {
	switch kind {
	case "Build":
		return &BuildDescriber{c}, true
	case "BuildConfig":
		return &BuildConfigDescriber{c, cmd}, true
	case "Deployment":
		return &DeploymentDescriber{c}, true
	case "DeploymentConfig":
		return &DeploymentDescriber{c}, true
	case "Image":
		return &ImageDescriber{c}, true
	case "ImageRepository":
		return &ImageRepositoryDescriber{c}, true
	case "Route":
		return &RouteDescriber{}, true
	}
	return nil, false
}

// BuildDescriber generates information about a build
type BuildDescriber struct {
	client.Interface
}

func (d *BuildDescriber) DescribeParameters(p buildapi.BuildParameters, out *tabwriter.Writer) {
	fmt.Fprintf(out, "Strategy:\t%s\n", string(p.Strategy.Type))
	fmt.Fprintf(out, "Source Type:\t%s\n", string(p.Source.Type))
	if p.Source.Git != nil {
		fmt.Fprintf(out, "URL:\t%s\n", string(p.Source.Git.URI))
		if len(p.Source.Git.Ref) > 0 {
			fmt.Fprintf(out, "Ref:\t%s\n", string(p.Source.Git.Ref))
		}
	}
	fmt.Fprintf(out, "Output Image:\t%s\n", string(p.Output.ImageTag))
	fmt.Fprintf(out, "Output Registry:\t%s\n", string(p.Output.Registry))
}

func (d *BuildDescriber) Describe(namespace, name string) (string, error) {
	c := d.Builds(namespace)
	build, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, build.ObjectMeta)
		fmt.Fprintf(out, "Status:\t%s\n", string(build.Status))
		fmt.Fprintf(out, "Build Pod:\t%s\n", string(build.PodName))
		d.DescribeParameters(build.Parameters, out)
		return nil
	})
}

// BuildConfigDescriber generates information about a buildConfig
type BuildConfigDescriber struct {
	client.Interface
	*cobra.Command
}

func (d *BuildConfigDescriber) Describe(namespace, name string) (string, error) {
	c := d.BuildConfigs(namespace)
	buildConfig, err := c.Get(name)
	if err != nil {
		return "", err
	}

	var kubeConfig *kclient.Config
	if d.Command != nil {
		kubeConfig = kubecmd.GetKubeConfig(d.Command)
	}

	webhooks := webhookUrl(buildConfig, kubeConfig)
	buildDescriber := &BuildDescriber{}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, buildConfig.ObjectMeta)
		buildDescriber.DescribeParameters(buildConfig.Parameters, out)
		for whType, whURL := range webhooks {
			fmt.Fprintf(out, "Webhook %s:\t%s\n", strings.Title(string(whType)), string(whURL))
		}
		return nil
	})
}

// DeploymentDescriber generates information about a deployment
type DeploymentDescriber struct {
	client.Interface
}

func (d *DeploymentDescriber) Describe(namespace, name string) (string, error) {
	c := d.Deployments(namespace)
	deployment, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, deployment.ObjectMeta)
		fmt.Fprintf(out, "Status:\t%s\n", string(deployment.Status))
		fmt.Fprintf(out, "Strategy:\t%s\n", string(deployment.Strategy.Type))
		fmt.Fprintf(out, "Causes:\n")
		if deployment.Details != nil {
			for _, c := range deployment.Details.Causes {
				fmt.Fprintf(out, "\t\t%s\n", string(c.Type))
			}
		}
		return nil
	})
}

// DeploymentConfigDescriber generates information about a DeploymentConfig
type DeploymentConfigDescriber struct {
	client.Interface
}

func (d *DeploymentConfigDescriber) Describe(namespace, name string) (string, error) {
	c := d.DeploymentConfigs(namespace)
	deploymentConfig, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, deploymentConfig.ObjectMeta)
		fmt.Fprintf(out, "Latest Version:\t%s\n", string(deploymentConfig.LatestVersion))
		fmt.Fprintf(out, "Triggers:\t\n")
		for _, t := range deploymentConfig.Triggers {
			fmt.Fprintf(out, "Type:\t%s\n", t.Type)
		}
		return nil
	})
}

// ImageDescriber generates information about a Image
type ImageDescriber struct {
	client.Interface
}

func (d *ImageDescriber) Describe(namespace, name string) (string, error) {
	c := d.Images(namespace)
	image, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, image.ObjectMeta)
		fmt.Fprintf(out, "Docker Image:\t%s\n", string(image.DockerImageReference))
		return nil
	})
}

// ImageRepositoryDescriber generates information about a ImageRepository
type ImageRepositoryDescriber struct {
	client.Interface
}

func (d *ImageRepositoryDescriber) Describe(namespace, name string) (string, error) {
	c := d.ImageRepositories(namespace)
	imageRepository, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, imageRepository.ObjectMeta)
		fmt.Fprintf(out, "Tags:\t%s\n", formatLabels(imageRepository.Tags))
		fmt.Fprintf(out, "Docker Repository:\t%s\n", string(imageRepository.DockerImageRepository))
		return nil
	})
}

// RouteDescriber generates information about a Route
type RouteDescriber struct {
	client.Interface
}

func (d *RouteDescriber) Describe(namespace, name string) (string, error) {
	c := d.Routes(namespace)
	route, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, route.ObjectMeta)
		fmt.Fprintf(out, "Host:\t%s\n", string(route.Host))
		fmt.Fprintf(out, "Path:\t%s\n", string(route.Path))
		fmt.Fprintf(out, "Service Name:\t%s\n", string(route.ServiceName))
		return nil
	})
}
