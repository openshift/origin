package describe

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kruntime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
)

func DescriberFor(kind string, c *client.Client, kclient kclient.Interface, host string) (kctl.Describer, bool) {
	switch kind {
	case "Build":
		return &BuildDescriber{c}, true
	case "BuildConfig":
		return &BuildConfigDescriber{c, host}, true
	case "Deployment":
		return &DeploymentDescriber{c}, true
	case "DeploymentConfig":
		return NewDeploymentConfigDescriber(c, kclient), true
	case "Image":
		return &ImageDescriber{c}, true
	case "ImageRepository":
		return &ImageRepositoryDescriber{c}, true
	case "Route":
		return &RouteDescriber{c}, true
	case "Project":
		return &ProjectDescriber{c}, true
	case "Policy":
		return &PolicyDescriber{c}, true
	case "PolicyBinding":
		return &PolicyBindingDescriber{c}, true
	}
	return nil, false
}

// BuildDescriber generates information about a build
type BuildDescriber struct {
	client.Interface
}

func (d *BuildDescriber) DescribeUser(out *tabwriter.Writer, label string, u buildapi.SourceControlUser) {
	if len(u.Name) > 0 && len(u.Email) > 0 {
		formatString(out, label, fmt.Sprintf("%s <%s>", u.Name, u.Email))
		return
	}
	if len(u.Name) > 0 {
		formatString(out, label, u.Name)
		return
	}
	if len(u.Email) > 0 {
		formatString(out, label, u.Email)
	}
}

func (d *BuildDescriber) DescribeParameters(p buildapi.BuildParameters, out *tabwriter.Writer) {
	formatString(out, "Strategy", p.Strategy.Type)
	switch p.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		if p.Strategy.DockerStrategy != nil && len(p.Strategy.DockerStrategy.ContextDir) == 0 {
			formatString(out, "Context Directory", p.Strategy.DockerStrategy.ContextDir)
		}
		if p.Strategy.DockerStrategy != nil && p.Strategy.DockerStrategy.NoCache {
			formatString(out, "No Cache", "yes")
		}
		if p.Strategy.DockerStrategy != nil {
			formatString(out, "BaseImage", p.Strategy.DockerStrategy.BaseImage)
		}
	case buildapi.STIBuildStrategyType:
		formatString(out, "Builder Image", p.Strategy.STIStrategy.Image)
		if p.Strategy.STIStrategy.Clean {
			formatString(out, "Clean Build", "yes")
		}
	case buildapi.CustomBuildStrategyType:
		formatString(out, "Builder Image", p.Strategy.CustomStrategy.Image)
		if p.Strategy.CustomStrategy.ExposeDockerSocket {
			formatString(out, "Expose Docker Socket", "yes")
		}
		if len(p.Strategy.CustomStrategy.Env) != 0 {
			formatString(out, "Environment", formatLabels(convertEnv(p.Strategy.CustomStrategy.Env)))
		}
	}
	formatString(out, "Source Type", p.Source.Type)
	if p.Source.Git != nil {
		formatString(out, "URL", p.Source.Git.URI)
		if len(p.Source.Git.Ref) > 0 {
			formatString(out, "Ref", p.Source.Git.Ref)
		}
	}
	if p.Output.To != nil {
		if p.Output.To.Namespace != "" {
			formatString(out, "Output to", fmt.Sprintf("%s/%s", p.Output.To.Namespace, p.Output.To.Name))
		} else {
			formatString(out, "Output to", p.Output.To.Name)
		}
	}

	formatString(out, "Output Spec", p.Output.DockerImageReference)
	if p.Revision != nil && p.Revision.Type == buildapi.BuildSourceGit && p.Revision.Git != nil {
		formatString(out, "Git Commit", p.Revision.Git.Commit)
		d.DescribeUser(out, "Revision Author", p.Revision.Git.Author)
		d.DescribeUser(out, "Revision Committer", p.Revision.Git.Committer)
		if len(p.Revision.Git.Message) > 0 {
			formatString(out, "Revision Message", p.Revision.Git.Message)
		}
	}
}

func (d *BuildDescriber) Describe(namespace, name string) (string, error) {
	c := d.Builds(namespace)
	build, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, build.ObjectMeta)
		formatString(out, "Status", bold(build.Status))
		formatString(out, "Build Pod", build.PodName)
		d.DescribeParameters(build.Parameters, out)
		return nil
	})
}

// BuildConfigDescriber generates information about a buildConfig
type BuildConfigDescriber struct {
	client.Interface
	// TODO: this is broken, webhook URL generation should be done by client interface using
	// the string value
	host string
}

// DescribeTriggers generates information about the triggers associated with a buildconfig
func (d *BuildConfigDescriber) DescribeTriggers(bc *buildapi.BuildConfig, host string, out *tabwriter.Writer) {
	webhooks := webhookURL(bc, host)
	for whType, whURL := range webhooks {
		t := strings.Title(whType)
		formatString(out, "Webhook "+t, whURL)
	}
	for _, trigger := range bc.Triggers {
		if trigger.Type != buildapi.ImageChangeBuildTriggerType {
			continue
		}
		if trigger.ImageChange.From.Namespace != "" {
			formatString(out, "Image Repository Trigger", fmt.Sprintf("%s/%s", trigger.ImageChange.From.Namespace, trigger.ImageChange.From.Name))
		} else {
			formatString(out, "Image Repository Trigger", trigger.ImageChange.From.Name)
		}
		formatString(out, "- Tag", trigger.ImageChange.Tag)
		formatString(out, "- Image", trigger.ImageChange.Image)
		formatString(out, "- LastTriggeredImageID", trigger.ImageChange.LastTriggeredImageID)
	}
}

func (d *BuildConfigDescriber) Describe(namespace, name string) (string, error) {
	c := d.BuildConfigs(namespace)
	buildConfig, err := c.Get(name)
	if err != nil {
		return "", err
	}

	buildDescriber := &BuildDescriber{}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, buildConfig.ObjectMeta)
		buildDescriber.DescribeParameters(buildConfig.Parameters, out)
		d.DescribeTriggers(buildConfig, d.host, out)
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
		formatString(out, "Docker Image", image.DockerImageReference)
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
		formatString(out, "Tags", formatLabels(imageRepository.Tags))
		formatString(out, "Registry", imageRepository.Status.DockerImageRepository)
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
		formatString(out, "Host", route.Host)
		formatString(out, "Path", route.Path)
		formatString(out, "Service", route.ServiceName)
		return nil
	})
}

// ProjectDescriber generates information about a Project
type ProjectDescriber struct {
	client.Interface
}

func (d *ProjectDescriber) Describe(namespace, name string) (string, error) {
	c := d.Projects()
	project, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, project.ObjectMeta)
		formatString(out, "Display Name", project.DisplayName)
		return nil
	})
}

// PolicyDescriber generates information about a Project
type PolicyDescriber struct {
	client.Interface
}

// TODO make something a lot prettier
func (d *PolicyDescriber) Describe(namespace, name string) (string, error) {
	c := d.Policies(namespace)
	policy, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, policy.ObjectMeta)
		formatString(out, "Last Modified", policy.LastModified)

		// display the rules in a consistent order
		sortedKeys := make([]string, 0)
		for key := range policy.Roles {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)

		for _, key := range sortedKeys {
			role := policy.Roles[key]
			fmt.Fprint(out, key+"\tVerbs\tResources\tExtension\n")
			for _, rule := range role.Rules {
				extensionString := ""
				if rule.AttributeRestrictions != (kruntime.EmbeddedObject{}) {
					extensionString = fmt.Sprintf("%v", rule.AttributeRestrictions)
				}

				fmt.Fprintf(out, "%v\t%v\t%v\t%v\n",
					"",
					rule.Verbs,
					rule.Resources,
					extensionString)

			}
		}

		return nil
	})
}

// PolicyBindingDescriber generates information about a Project
type PolicyBindingDescriber struct {
	client.Interface
}

// TODO make something a lot prettier
func (d *PolicyBindingDescriber) Describe(namespace, name string) (string, error) {
	c := d.PolicyBindings(namespace)
	policyBinding, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, policyBinding.ObjectMeta)
		formatString(out, "Last Modified", policyBinding.LastModified)
		formatString(out, "Policy", policyBinding.PolicyRef.Namespace)

		// display the rules in a consistent order
		sortedKeys := make([]string, 0)
		for key := range policyBinding.RoleBindings {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)

		for _, key := range sortedKeys {
			roleBinding := policyBinding.RoleBindings[key]
			formatString(out, "RoleBinding["+key+"]", " ")
			formatString(out, "\tRole", roleBinding.RoleRef.Name)
			formatString(out, "\tUsers", roleBinding.UserNames)
			formatString(out, "\tGroups", roleBinding.GroupNames)
		}

		return nil
	})
}
