package describe

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/docker/docker/pkg/parsers"

	"github.com/openshift/origin/pkg/api/graph"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
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
	case "Identity":
		return &IdentityDescriber{c}, true
	case "Image":
		return &ImageDescriber{c}, true
	case "ImageStream":
		return &ImageStreamDescriber{c}, true
	case "ImageStreamTag":
		return &ImageStreamTagDescriber{c}, true
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
	case "User":
		return &UserDescriber{c}, true
	case "UserIdentityMapping":
		return &UserIdentityMappingDescriber{c}, true
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
		formatString(out, "Build Pod", buildutil.GetBuildPodName(build))
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
		tag := buildapi.DefaultImageTag
		if len(p.Output.Tag) != 0 {
			tag = p.Output.Tag
		}
		if len(p.Output.To.Namespace) != 0 {
			formatString(out, "Output to", fmt.Sprintf("%s/%s:%s", p.Output.To.Namespace, p.Output.To.Name, tag))
		} else {
			formatString(out, "Output to", fmt.Sprintf("%s:%s", p.Output.To.Name, tag))
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
	if s.From != nil && len(s.From.Name) != 0 {
		if len(s.From.Namespace) != 0 {
			formatString(out, "Image Repository", fmt.Sprintf("%s/%s", s.From.Name, s.From.Namespace))
		} else {
			formatString(out, "Image Repository", s.From.Name)
		}
		if len(s.Tag) != 0 {
			formatString(out, "Image Repository Tag", s.Tag)
		}
	} else {
		formatString(out, "Builder Image", s.Image)
	}
	if len(s.Scripts) != 0 {
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
		if len(trigger.ImageChange.From.Namespace) != 0 {
			formatString(out, "Image Repository Trigger", fmt.Sprintf("%s/%s", trigger.ImageChange.From.Namespace, trigger.ImageChange.From.Name))
		} else {
			formatString(out, "Image Repository Trigger", trigger.ImageChange.From.Name)
		}
		formatString(out, "- Tag", trigger.ImageChange.Tag)
		formatString(out, "- Image", trigger.ImageChange.Image)
		formatString(out, "- LastTriggeredImageID", trigger.ImageChange.LastTriggeredImageID)
	}
}

type sortableBuilds []buildapi.Build

func (s sortableBuilds) Len() int {
	return len(s)
}

func (s sortableBuilds) Less(i, j int) bool {
	return s[i].CreationTimestamp.Before(s[j].CreationTimestamp)
}

func (s sortableBuilds) Swap(i, j int) {
	t := s[i]
	s[i] = s[j]
	s[j] = t
}

func (d *BuildConfigDescriber) Describe(namespace, name string) (string, error) {
	c := d.BuildConfigs(namespace)
	buildConfig, err := c.Get(name)
	if err != nil {
		return "", err
	}
	builds, err := d.Builds(namespace).List(labels.SelectorFromSet(labels.Set{buildapi.BuildConfigLabel: name}), fields.Everything())
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, buildConfig.ObjectMeta)
		if buildConfig.LastVersion == 0 {
			formatString(out, "Latest Version", "Never built")
		} else {
			formatString(out, "Latest Version", strconv.Itoa(buildConfig.LastVersion))
		}
		describeBuildParameters(buildConfig.Parameters, out)
		d.DescribeTriggers(buildConfig, d.host, out)
		if len(builds.Items) == 0 {
			return nil
		}
		fmt.Fprintf(out, "Builds(Name,Status,Creation Time)\n")
		sortedBuilds := sortableBuilds(builds.Items)
		sort.Sort(sortedBuilds)
		c := 0
		for i := range sortedBuilds {
			// iterate backwards so we're printing the newest items first
			build := sortedBuilds[len(sortedBuilds)-1-i]
			fmt.Fprintf(out, fmt.Sprintf("- %s %s %v\n", build.Name, build.Status, build.CreationTimestamp.Rfc3339Copy().Time))
			c++
			// only print the 10 most recent builds.
			if c > 10 {
				break
			}
		}
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

// ImageStreamTagDescriber generates information about a ImageStreamTag (Image).
type ImageStreamTagDescriber struct {
	client.Interface
}

func (d *ImageStreamTagDescriber) Describe(namespace, name string) (string, error) {
	c := d.ImageStreamTags(namespace)
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

// ImageStreamDescriber generates information about a ImageStream
type ImageStreamDescriber struct {
	client.Interface
}

func (d *ImageStreamDescriber) Describe(namespace, name string) (string, error) {
	c := d.ImageStreams(namespace)
	imageStream, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, imageStream.ObjectMeta)
		formatImageStreamTags(out, imageStream)
		formatString(out, "Registry", imageStream.Status.DockerImageRepository)
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

// ProjectStatusDescriber generates extended information about a Project
type ProjectStatusDescriber struct {
	K kclient.Interface
	C client.Interface
}

func (d *ProjectStatusDescriber) Describe(namespace, name string) (string, error) {
	project, err := d.C.Projects().Get(namespace)
	if err != nil {
		return "", err
	}

	svcs, err := d.K.Services(namespace).List(labels.Everything())
	if err != nil {
		return "", err
	}

	bcs, err := d.C.BuildConfigs(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return "", err
	}

	dcs, err := d.C.DeploymentConfigs(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return "", err
	}

	g := graph.New()
	for i := range bcs.Items {
		graph.BuildConfig(g, &bcs.Items[i])
	}
	for i := range dcs.Items {
		graph.DeploymentConfig(g, &dcs.Items[i])
	}
	for i := range svcs.Items {
		graph.Service(g, &svcs.Items[i])
	}
	groups := graph.ServiceAndDeploymentGroups(graph.CoverServices(g))

	return tabbedString(func(out *tabwriter.Writer) error {
		indent := "  "
		if len(project.DisplayName) > 0 && project.DisplayName != namespace {
			fmt.Fprintf(out, "In project %s (%s)\n", project.DisplayName, namespace)
		} else {
			fmt.Fprintf(out, "In project %s\n", namespace)
		}

		for _, group := range groups {
			if len(group.Builds) != 0 {
				for _, flow := range group.Builds {
					if flow.Image != nil {
						if flow.Build != nil {
							fmt.Fprintf(out, "\n%s -> build %s\n", flow.Image.ImageSpec, flow.Build.Name)
						}
					} else {
						fmt.Fprintf(out, "\nbuild %s\n", flow.Build.Name)
					}
				}
				continue
			}
			if len(group.Services) == 0 {
				for _, deploy := range group.Deployments {
					fmt.Fprintln(out)
					printLines(out, indent, 0, describeDeploymentInServiceGroup(deploy)...)
				}
				continue
			}
			fmt.Fprintln(out)
			for _, svc := range group.Services {
				printLines(out, indent, 0, describeServiceInServiceGroup(svc)...)
			}
			for _, deploy := range group.Deployments {
				printLines(out, indent, 1, describeDeploymentInServiceGroup(deploy)...)
			}
		}

		if len(groups) == 0 {
			fmt.Fprintln(out, "\nYou have no services, deployment configs, or build configs. 'osc new-app' can be used to create applications from scratch from existing Docker images and templates.")
		} else {
			fmt.Fprintln(out, "\nTo see more information about a service or deployment config, use 'osc describe service <name>' or 'osc describe dc <name>'.")
			fmt.Fprintln(out, "You can use 'osc get pods,svc,dc,bc,builds' to see lists of each of the types described above.")
		}

		return nil
	})
}

func printLines(out io.Writer, indent string, depth int, lines ...string) {
	for i, s := range lines {
		fmt.Fprintf(out, strings.Repeat(indent, depth))
		if i != 0 {
			fmt.Fprint(out, indent)
		}
		fmt.Fprintln(out, s)
	}
}

func describeDeploymentInServiceGroup(deploy graph.DeploymentFlow) []string {
	if len(deploy.Images) == 1 {
		return []string{fmt.Sprintf("%s deploys %s", deploy.Deployment.Name, describeImageInPipeline(deploy.Images[0], deploy.Deployment.Namespace))}
	}
	lines := []string{fmt.Sprintf("%s deploys:", deploy.Deployment.Name)}
	for _, image := range deploy.Images {
		lines = append(lines, fmt.Sprintf("%s", describeImageInPipeline(image, deploy.Deployment.Namespace)))
	}
	return lines
}

func describeImageInPipeline(pipeline graph.ImagePipeline, namespace string) string {
	switch {
	case pipeline.Image != nil && pipeline.Build != nil:
		return fmt.Sprintf("%s <- %s", describeImageTagInPipeline(pipeline.Image, namespace), describeBuildInPipeline(pipeline.Build.BuildConfig, pipeline.BaseImage))
	case pipeline.Image != nil:
		return describeImageTagInPipeline(pipeline.Image, namespace)
	case pipeline.Build != nil:
		return describeBuildInPipeline(pipeline.Build.BuildConfig, pipeline.BaseImage)
	default:
		return "<unknown>"
	}
}

func describeImageTagInPipeline(image graph.ImageTagLocation, namespace string) string {
	switch t := image.(type) {
	case *graph.ImageStreamTagNode:
		if t.ImageStream.Namespace != namespace {
			return image.ImageSpec()
		}
		return fmt.Sprintf("%s:%s", t.ImageStream.Name, image.ImageTag())
	default:
		return image.ImageSpec()
	}
}

func describeBuildInPipeline(build *buildapi.BuildConfig, baseImage graph.ImageTagLocation) string {
	switch build.Parameters.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		// TODO: handle case where no source repo
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return "docker build; no source set"
		}
		return fmt.Sprintf("docker build of %s", source)
	case buildapi.STIBuildStrategyType:
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return fmt.Sprintf("unconfigured source build %s", build.Name)
		}
		if baseImage == nil {
			return fmt.Sprintf("%s; no image set", source)
		}
		return fmt.Sprintf("building %s on %s", source, baseImage.ImageSpec())
	case buildapi.CustomBuildStrategyType:
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return fmt.Sprintf("custom build %s", build.Name)
		}
		return fmt.Sprintf("custom build of %s", source)
	default:
		return fmt.Sprintf("unrecognized build %s", build.Name)
	}
}

func describeSourceInPipeline(source *buildapi.BuildSource) (string, bool) {
	switch source.Type {
	case buildapi.BuildSourceGit:
		if len(source.Git.Ref) == 0 {
			return source.Git.URI, true
		}
		return fmt.Sprintf("%s#%s", source.Git.URI, source.Git.Ref), true
	}
	return "", false
}

func describeServiceInServiceGroup(svc graph.ServiceReference) []string {
	spec := svc.Service.Spec
	ip := spec.PortalIP
	var port string
	if spec.TargetPort.String() == "0" || ip == "None" {
		port = fmt.Sprintf(":%d", spec.Port)
	} else {
		port = fmt.Sprintf(":%d -> %s", spec.Port, spec.TargetPort.String())
	}
	switch {
	case ip == "None":
		return []string{fmt.Sprintf("service %s (headless%s)", svc.Service.Name, port)}
	case len(ip) == 0:
		return []string{fmt.Sprintf("service %s (initializing%s)", svc.Service.Name, port)}
	default:
		return []string{fmt.Sprintf("service %s (%s%s)", svc.Service.Name, ip, port)}
	}
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
		extensionString = fmt.Sprintf("%#v", rule.AttributeRestrictions.Object)

		buffer := new(bytes.Buffer)
		printer := NewHumanReadablePrinter(true)
		if err := printer.PrintObj(rule.AttributeRestrictions.Object, buffer); err == nil {
			extensionString = strings.TrimSpace(buffer.String())
		}
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

func (d *TemplateDescriber) DescribeObjects(objects []runtime.Object, out *tabwriter.Writer) {
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
		fmt.Fprintf(out, fmt.Sprintf("%s%s\t%s\n", indent, kind, meta.Name))
		//meta.Annotations, _ = d.MetadataAccessor.Annotations(obj)
		//meta.Labels, _ = d.MetadataAccessor.Labels(obj)
		/*if len(meta.Labels) > 0 {
			formatString(out, indent+"Labels", formatLabels(meta.Labels))
		}
		formatAnnotations(out, meta, indent)*/
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
		formatString(out, "Object Labels", formatLabels(template.ObjectLabels))
		out.Write([]byte("\n"))
		out.Flush()
		d.DescribeObjects(template.Objects, out)
		return nil
	})
}

// IdentityDescriber generates information about a user
type IdentityDescriber struct {
	client.Interface
}

func (d *IdentityDescriber) Describe(namespace, name string) (string, error) {
	userClient := d.Users()
	identityClient := d.Identities()

	identity, err := identityClient.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, identity.ObjectMeta)

		if len(identity.User.Name) == 0 {
			formatString(out, "User Name", identity.User.Name)
			formatString(out, "User UID", identity.User.UID)
		} else {
			resolvedUser, err := userClient.Get(identity.User.Name)

			nameValue := identity.User.Name
			uidValue := string(identity.User.UID)

			if kerrs.IsNotFound(err) {
				nameValue += fmt.Sprintf(" (Error: User does not exist)")
			} else if err != nil {
				nameValue += fmt.Sprintf(" (Error: User lookup failed)")
			} else {
				if !util.NewStringSet(resolvedUser.Identities...).Has(name) {
					nameValue += fmt.Sprintf(" (Error: User identities do not include %s)", name)
				}
				if resolvedUser.UID != identity.User.UID {
					uidValue += fmt.Sprintf(" (Error: Actual user UID is %s)", string(resolvedUser.UID))
				}
			}

			formatString(out, "User Name", nameValue)
			formatString(out, "User UID", uidValue)
		}
		return nil
	})

}

// UserIdentityMappingDescriber generates information about a user
type UserIdentityMappingDescriber struct {
	client.Interface
}

func (d *UserIdentityMappingDescriber) Describe(namespace, name string) (string, error) {
	c := d.UserIdentityMappings()

	mapping, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, mapping.ObjectMeta)
		formatString(out, "Identity", mapping.Identity.Name)
		formatString(out, "User Name", mapping.User.Name)
		formatString(out, "User UID", mapping.User.UID)
		return nil
	})
}

// UserDescriber generates information about a user
type UserDescriber struct {
	client.Interface
}

func (d *UserDescriber) Describe(namespace, name string) (string, error) {
	userClient := d.Users()
	identityClient := d.Identities()

	user, err := userClient.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, user.ObjectMeta)
		if len(user.FullName) > 0 {
			formatString(out, "Full Name", user.FullName)
		}

		if len(user.Identities) == 0 {
			formatString(out, "Identities", "<none>")
		} else {
			for i, identity := range user.Identities {
				resolvedIdentity, err := identityClient.Get(identity)

				value := identity
				if kerrs.IsNotFound(err) {
					value += fmt.Sprintf(" (Error: Identity does not exist)")
				} else if err != nil {
					value += fmt.Sprintf(" (Error: Identity lookup failed)")
				} else if resolvedIdentity.User.Name != name {
					value += fmt.Sprintf(" (Error: Identity maps to user name '%s')", resolvedIdentity.User.Name)
				} else if resolvedIdentity.User.UID != user.UID {
					value += fmt.Sprintf(" (Error: Identity maps to user UID '%s')", resolvedIdentity.User.UID)
				}

				if i == 0 {
					formatString(out, "Identities", value)
				} else {
					fmt.Fprintf(out, "           \t%s\n", value)
				}
			}
		}
		return nil
	})
}
