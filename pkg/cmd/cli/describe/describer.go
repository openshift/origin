package describe

import (
	"fmt"
	"reflect"
	"strings"
	"text/tabwriter"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/docker/docker/pkg/parsers"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func DescriberFor(kind string, c *client.Client, kclient kclient.Interface, host string) (kctl.Describer, bool) {
	switch kind {
	case "Build":
		return &BuildDescriber{c}, true
	case "BuildConfig":
		return &BuildConfigDescriber{c, host}, true
	case "BuildLog":
		return &BuildLogDescriber{c}, true
	case "Deployment":
		return &DeploymentDescriber{c}, true
	case "DeploymentConfig":
		return NewDeploymentConfigDescriber(c, kclient), true
	case "Image":
		return &ImageDescriber{c}, true
	case "ImageRepository":
		return &ImageRepositoryDescriber{c}, true
	case "ImageRepositoryTag":
		return &ImageRepositoryTagDescriber{c}, true
	case "ImageStreamImage":
		return &ImageStreamImageDescriber{c}, true
	case "Route":
		return &RouteDescriber{c}, true
	case "Project":
		return &ProjectDescriber{c}, true
	case "Template":
		return &TemplateDescriber{c, meta.NewAccessor(), kapi.Scheme, nil}, true
	case "Policy":
		return &PolicyDescriber{c}, true
	case "PolicyBinding":
		return &PolicyBindingDescriber{c}, true
	case "RoleBinding":
		return &RoleBindingDescriber{c}, true
	case "Role":
		return &RoleDescriber{c}, true
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

func (d *BuildDescriber) Describe(namespace, name string) (string, error) {
	c := d.Builds(namespace)
	build, err := c.Get(name)
	if err != nil {
		return "", err
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, build.ObjectMeta)
		formatString(out, "BuildConfig", build.Labels[buildapi.BuildConfigLabel])
		formatString(out, "Status", bold(build.Status))
		if build.StartTimestamp != nil {
			formatString(out, "Started", build.StartTimestamp.Time)
		}
		if build.CompletionTimestamp != nil {
			formatString(out, "Finished", build.CompletionTimestamp.Time)
		}
		// Create the time object with second-level precision so we don't get
		// output like "duration: 1.2724395728934s"
		t := util.Now().Rfc3339Copy()
		if build.StartTimestamp != nil && build.CompletionTimestamp != nil {
			// time a build ran from pod creation to build finish or cancel
			formatString(out, "Duration", build.CompletionTimestamp.Sub(build.StartTimestamp.Rfc3339Copy().Time))
		} else if build.CompletionTimestamp != nil && build.Status == buildapi.BuildStatusCancelled {
			// time a build waited for its pod before ultimately being canceled before that pod was created
			formatString(out, "Duration", fmt.Sprintf("waited for %s", build.CompletionTimestamp.Sub(build.CreationTimestamp.Rfc3339Copy().Time)))
		} else if build.CompletionTimestamp != nil && build.Status != buildapi.BuildStatusCancelled {
			// for some reason we never saw the pod enter the running state, so we don't know when it
			// "started", so instead print out the time from creation to completion.
			formatString(out, "Duration", build.CompletionTimestamp.Sub(build.CreationTimestamp.Rfc3339Copy().Time))
		} else if build.StartTimestamp == nil && build.Status != buildapi.BuildStatusCancelled {
			// time a new build has been waiting for its pod to be created so it can run
			formatString(out, "Duration", fmt.Sprintf("waiting for %s", t.Sub(build.CreationTimestamp.Rfc3339Copy().Time)))
		} else if build.CompletionTimestamp == nil {
			// time a still running build has been running in a pod
			formatString(out, "Duration", fmt.Sprintf("running for %s", t.Sub(build.StartTimestamp.Rfc3339Copy().Time)))
		}
		formatString(out, "Build Pod", build.PodName)
		describeBuildParameters(build.Parameters, out)
		return nil
	})
}

// BuildConfigDescriber generates information about a buildConfig
type BuildConfigDescriber struct {
	client.Interface
	host string
}

func describeBuildParameters(p buildapi.BuildParameters, out *tabwriter.Writer) {
	formatString(out, "Strategy", p.Strategy.Type)
	switch p.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		if p.Strategy.DockerStrategy != nil && p.Strategy.DockerStrategy.NoCache {
			formatString(out, "No Cache", "yes")
		}
		if p.Strategy.DockerStrategy != nil {
			formatString(out, "Image", p.Strategy.DockerStrategy.Image)
		}
	case buildapi.STIBuildStrategyType:
		describeSTIStrategy(p.Strategy.STIStrategy, out)
	case buildapi.CustomBuildStrategyType:
		formatString(out, "Image", p.Strategy.CustomStrategy.Image)
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
		if len(p.Source.ContextDir) > 0 {
			formatString(out, "ContextDir", p.Source.ContextDir)
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
	if len(p.Output.PushSecretName) > 0 {
		formatString(out, "Push Secret", p.Output.PushSecretName)
	}

	if p.Revision != nil && p.Revision.Type == buildapi.BuildSourceGit && p.Revision.Git != nil {
		buildDescriber := &BuildDescriber{}

		formatString(out, "Git Commit", p.Revision.Git.Commit)
		buildDescriber.DescribeUser(out, "Revision Author", p.Revision.Git.Author)
		buildDescriber.DescribeUser(out, "Revision Committer", p.Revision.Git.Committer)
		if len(p.Revision.Git.Message) > 0 {
			formatString(out, "Revision Message", p.Revision.Git.Message)
		}
	}
}

func describeSTIStrategy(s *buildapi.STIBuildStrategy, out *tabwriter.Writer) {
	if s.From != nil && s.From.Name != "" {
		if s.From.Namespace != "" {
			formatString(out, "Image Repository", fmt.Sprintf("%s/%s", s.From.Name, s.From.Namespace))
		} else {
			formatString(out, "Image Repository", s.From.Name)
		}
		if s.Tag != "" {
			formatString(out, "Image Repository Tag", s.Tag)
		}
	} else {
		formatString(out, "Builder Image", s.Image)
	}
	if s.Scripts != "" {
		formatString(out, "Scripts", s.Scripts)
	}
	if s.Incremental {
		formatString(out, "Incremental Build", "yes")
	}
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

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, buildConfig.ObjectMeta)
		describeBuildParameters(buildConfig.Parameters, out)
		d.DescribeTriggers(buildConfig, d.host, out)
		return nil
	})
}

// BuildLogDescriber generates information about a BuildLog
type BuildLogDescriber struct {
	client.Interface
}

func (d *BuildLogDescriber) Describe(namespace, name string) (string, error) {
	return fmt.Sprintf("Name: %s/%s, Labels:", namespace, name), nil
}

// ImageDescriber generates information about a Image
type ImageDescriber struct {
	client.Interface
}

func (d *ImageDescriber) Describe(namespace, name string) (string, error) {
	c := d.Images()
	image, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return describeImage(image)
}

func describeImage(image *imageapi.Image) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, image.ObjectMeta)
		formatString(out, "Docker Image", image.DockerImageReference)
		return nil
	})
}

// ImageRepositoryTagDescriber generates information about a ImageRepositoryTag (Image).
type ImageRepositoryTagDescriber struct {
	client.Interface
}

func (d *ImageRepositoryTagDescriber) Describe(namespace, name string) (string, error) {
	c := d.ImageRepositoryTags(namespace)
	repo, tag := parsers.ParseRepositoryTag(name)
	if tag == "" {
		// TODO use repo's preferred default, when that's coded
		tag = "latest"
	}
	image, err := c.Get(repo, tag)
	if err != nil {
		return "", err
	}

	return describeImage(image)
}

// ImageStreamImageDescriber generates information about a ImageStreamImage (Image).
type ImageStreamImageDescriber struct {
	client.Interface
}

func (d *ImageStreamImageDescriber) Describe(namespace, name string) (string, error) {
	c := d.ImageStreamImages(namespace)
	repo, id := parsers.ParseRepositoryTag(name)
	image, err := c.Get(repo, id)
	if err != nil {
		return "", err
	}

	return describeImage(image)
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
		formatString(out, "Status", project.Status.Phase)
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

		// using .List() here because I always want the sorted order that it provides
		for _, key := range util.KeySet(reflect.ValueOf(policy.Roles)).List() {
			role := policy.Roles[key]
			fmt.Fprint(out, key+"\t"+policyRuleHeadings+"\n")
			for _, rule := range role.Rules {
				describePolicyRule(out, rule, "\t")
			}
		}

		return nil
	})
}

const policyRuleHeadings = "Verbs\tResources\tResource Names\tExtension"

func describePolicyRule(out *tabwriter.Writer, rule authorizationapi.PolicyRule, indent string) {
	extensionString := ""
	if rule.AttributeRestrictions != (runtime.EmbeddedObject{}) {
		extensionString = fmt.Sprintf("%v", rule.AttributeRestrictions)
	}

	fmt.Fprintf(out, indent+"%v\t%v\t%v\t%v\n",
		rule.Verbs.List(),
		rule.Resources.List(),
		rule.ResourceNames.List(),
		extensionString)
}

// RoleDescriber generates information about a Project
type RoleDescriber struct {
	client.Interface
}

func (d *RoleDescriber) Describe(namespace, name string) (string, error) {
	c := d.Roles(namespace)
	role, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, role.ObjectMeta)

		fmt.Fprint(out, policyRuleHeadings+"\n")
		for _, rule := range role.Rules {
			describePolicyRule(out, rule, "")

		}

		return nil
	})
}

// PolicyBindingDescriber generates information about a Project
type PolicyBindingDescriber struct {
	client.Interface
}

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

		// using .List() here because I always want the sorted order that it provides
		for _, key := range util.KeySet(reflect.ValueOf(policyBinding.RoleBindings)).List() {
			roleBinding := policyBinding.RoleBindings[key]
			formatString(out, "RoleBinding["+key+"]", " ")
			formatString(out, "\tRole", roleBinding.RoleRef.Name)
			formatString(out, "\tUsers", roleBinding.Users.List())
			formatString(out, "\tGroups", roleBinding.Groups.List())
		}

		return nil
	})
}

// RoleBindingDescriber generates information about a Project
type RoleBindingDescriber struct {
	client.Interface
}

func (d *RoleBindingDescriber) Describe(namespace, name string) (string, error) {
	c := d.RoleBindings(namespace)
	roleBinding, err := c.Get(name)
	if err != nil {
		return "", err
	}

	role, err := d.Roles(roleBinding.RoleRef.Namespace).Get(roleBinding.RoleRef.Name)
	return DescribeRoleBinding(roleBinding, role, err)
}

// DescribeRoleBinding prints out information about a role binding and its associated role
func DescribeRoleBinding(roleBinding *authorizationapi.RoleBinding, role *authorizationapi.Role, err error) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, roleBinding.ObjectMeta)

		formatString(out, "Role", roleBinding.RoleRef.Namespace+"/"+roleBinding.RoleRef.Name)
		formatString(out, "Users", roleBinding.Users.List())
		formatString(out, "Groups", roleBinding.Groups.List())

		switch {
		case err != nil:
			formatString(out, "Policy Rules", fmt.Sprintf("error: %v", err))

		case role != nil:
			fmt.Fprint(out, policyRuleHeadings+"\n")
			for _, rule := range role.Rules {
				describePolicyRule(out, rule, "")
			}

		default:
			formatString(out, "Policy Rules", "<none>")
		}

		return nil
	})
}

// DescribeRole prints out information about a role
func DescribeRole(role *authorizationapi.Role) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, role.ObjectMeta)

		fmt.Fprint(out, policyRuleHeadings+"\n")
		for _, rule := range role.Rules {
			describePolicyRule(out, rule, "")
		}

		return nil
	})
}

// TemplateDescriber generates information about a template
type TemplateDescriber struct {
	client.Interface
	meta.MetadataAccessor
	runtime.ObjectTyper
	DescribeObject func(obj runtime.Object, out *tabwriter.Writer) (bool, error)
}

func (d *TemplateDescriber) DescribeParameters(params []templateapi.Parameter, out *tabwriter.Writer) {
	formatString(out, "Parameters", " ")
	indent := "    "
	for _, p := range params {
		formatString(out, indent+"Name", p.Name)
		formatString(out, indent+"Description", p.Description)
		if len(p.Generate) == 0 {
			formatString(out, indent+"Value", p.Value)
			continue
		}
		if len(p.Value) > 0 {
			formatString(out, indent+"Value", p.Value)
			formatString(out, indent+"Generated (ignored)", p.Generate)
			formatString(out, indent+"From", p.From)
		} else {
			formatString(out, indent+"Generated", p.Generate)
			formatString(out, indent+"From", p.From)
		}
		out.Write([]byte("\n"))
	}
}

func (d *TemplateDescriber) DescribeObjects(objects []runtime.Object, labels map[string]string, out *tabwriter.Writer) {
	formatString(out, "Objects", " ")

	indent := "    "
	for _, obj := range objects {
		if d.DescribeObject != nil {
			if ok, _ := d.DescribeObject(obj, out); ok {
				out.Write([]byte("\n"))
				continue
			}
		}

		_, kind, _ := d.ObjectTyper.ObjectVersionAndKind(obj)
		meta := kapi.ObjectMeta{}
		meta.Name, _ = d.MetadataAccessor.Name(obj)
		meta.Annotations, _ = d.MetadataAccessor.Annotations(obj)
		meta.Labels, _ = d.MetadataAccessor.Labels(obj)
		fmt.Fprintf(out, fmt.Sprintf("%s%s\t%s\n", indent, kind, meta.Name))
		if len(meta.Labels) > 0 {
			formatString(out, indent+"Labels", formatLabels(meta.Labels))
		}
		formatAnnotations(out, meta, indent)
	}
	if len(labels) > 0 {
		out.Write([]byte("\n"))
		formatString(out, indent+"Common Labels", formatLabels(labels))
	}
}

func (d *TemplateDescriber) Describe(namespace, name string) (string, error) {
	c := d.Templates(namespace)
	template, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, template.ObjectMeta)
		out.Write([]byte("\n"))
		out.Flush()
		d.DescribeParameters(template.Parameters, out)
		out.Write([]byte("\n"))
		d.DescribeObjects(template.Objects, template.ObjectLabels, out)
		return nil
	})
}
