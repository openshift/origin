package describe

import (
	"bytes"
	"fmt"
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

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// DescriberFor returns a describer for a given kind of resource
func DescriberFor(kind string, c *client.Client, kclient kclient.Interface, host string) (kctl.Describer, bool) {
	switch kind {
	case "Build":
		return &BuildDescriber{c, kclient}, true
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
	case "ClusterPolicy":
		return &ClusterPolicyDescriber{c}, true
	case "ClusterPolicyBinding":
		return &ClusterPolicyBindingDescriber{c}, true
	case "ClusterRoleBinding":
		return &ClusterRoleBindingDescriber{c}, true
	case "ClusterRole":
		return &ClusterRoleDescriber{c}, true
	case "User":
		return &UserDescriber{c}, true
	case "UserIdentityMapping":
		return &UserIdentityMappingDescriber{c}, true
	}
	return nil, false
}

// BuildDescriber generates information about a build
type BuildDescriber struct {
	osClient   client.Interface
	kubeClient kclient.Interface
}

// DescribeUser formats the description of a user
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

// Describe returns the description of a build
func (d *BuildDescriber) Describe(namespace, name string) (string, error) {
	c := d.osClient.Builds(namespace)
	build, err := c.Get(name)
	if err != nil {
		return "", err
	}
	events, _ := d.kubeClient.Events(namespace).Search(build)
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
		formatString(out, "Duration", describeBuildDuration(build))
		formatString(out, "Build Pod", buildutil.GetBuildPodName(build))
		describeBuildParameters(build.Parameters, out)
		if events != nil {
			kctl.DescribeEvents(events, out)
		}

		return nil
	})
}

func describeBuildDuration(build *buildapi.Build) string {
	t := util.Now().Rfc3339Copy()
	if build.StartTimestamp == nil && build.Status == buildapi.BuildStatusCancelled {
		// time a build waited for its pod before ultimately being canceled before that pod was created
		return fmt.Sprintf("waited for %s", build.CompletionTimestamp.Rfc3339Copy().Time.Sub(build.CreationTimestamp.Rfc3339Copy().Time))
	} else if build.StartTimestamp == nil && build.Status != buildapi.BuildStatusCancelled {
		// time a new build has been waiting for its pod to be created so it can run
		return fmt.Sprintf("waiting for %v", t.Sub(build.CreationTimestamp.Rfc3339Copy().Time))
	} else if build.StartTimestamp != nil && build.CompletionTimestamp == nil {
		// time a still running build has been running in a pod
		return fmt.Sprintf("running for %v", build.Duration)
	}
	return fmt.Sprintf("%v", build.Duration)
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
		describeDockerStrategy(p.Strategy.DockerStrategy, out)
	case buildapi.STIBuildStrategyType:
		describeSTIStrategy(p.Strategy.STIStrategy, out)
	case buildapi.CustomBuildStrategyType:
		describeCustomStrategy(p.Strategy.CustomStrategy, out)
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
		if len(p.Source.SourceSecretName) > 0 {
			formatString(out, "Source Secret", p.Source.SourceSecretName)
		}
	}
	if p.Output.To != nil {
		tag := imageapi.DefaultImageTag
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
			formatString(out, "Image Reference", fmt.Sprintf("%s %s/%s", s.From.Kind, s.From.Namespace, s.From.Name))
		} else {
			formatString(out, "Image Reference", fmt.Sprintf("%s %s", s.From.Kind, s.From.Name))
		}
	}
	if len(s.Scripts) != 0 {
		formatString(out, "Scripts", s.Scripts)
	}
	if s.Incremental {
		formatString(out, "Incremental Build", "yes")
	}
}

func describeDockerStrategy(s *buildapi.DockerBuildStrategy, out *tabwriter.Writer) {
	if s.From != nil && len(s.From.Name) != 0 {
		if len(s.From.Namespace) != 0 {
			formatString(out, "Image Reference", fmt.Sprintf("%s %s/%s", s.From.Kind, s.From.Namespace, s.From.Name))
		} else {
			formatString(out, "Image Reference", fmt.Sprintf("%s %s", s.From.Kind, s.From.Name))
		}
	}
	if s.NoCache {
		formatString(out, "No Cache", "true")
	}
}

func describeCustomStrategy(s *buildapi.CustomBuildStrategy, out *tabwriter.Writer) {
	if s.From != nil && len(s.From.Name) != 0 {
		if len(s.From.Namespace) != 0 {
			formatString(out, "Image Reference", fmt.Sprintf("%s %s/%s", s.From.Kind, s.From.Namespace, s.From.Name))
		} else {
			formatString(out, "Image Reference", fmt.Sprintf("%s %s", s.From.Kind, s.From.Name))
		}
	}
	if s.ExposeDockerSocket {
		formatString(out, "Expose Docker Socket", "yes")
	}
	if len(s.Env) != 0 {
		formatString(out, "Environment", formatLabels(convertEnv(s.Env)))
	}
}

// DescribeTriggers generates information about the triggers associated with a buildconfig
func (d *BuildConfigDescriber) DescribeTriggers(bc *buildapi.BuildConfig, out *tabwriter.Writer) {
	webhooks := webhookURL(bc, d.Interface)
	for whType, whURL := range webhooks {
		t := strings.Title(whType)
		formatString(out, "Webhook "+t, whURL)
	}
	for _, trigger := range bc.Triggers {
		if trigger.Type != buildapi.ImageChangeBuildTriggerType {
			continue
		}
		fmt.Fprintf(out, fmt.Sprintf("Image Repository Trigger\n"))
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

// Describe returns the description of a buildConfig
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
		d.DescribeTriggers(buildConfig, out)
		if len(builds.Items) == 0 {
			return nil
		}
		fmt.Fprintf(out, "Builds:\n  Name\tStatus\tDuration\tCreation Time\n")
		sortedBuilds := sortableBuilds(builds.Items)
		sort.Sort(sortedBuilds)
		for i := range sortedBuilds {
			// iterate backwards so we're printing the newest items first
			build := sortedBuilds[len(sortedBuilds)-1-i]
			fmt.Fprintf(out, "  %s \t%s \t%v \t%v\n",
				build.Name,
				strings.ToLower(string(build.Status)),
				describeBuildDuration(&build),
				build.CreationTimestamp.Rfc3339Copy().Time)
			// only print the 10 most recent builds.
			if i == 9 {
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

// Describe returns the description of a buildLog
func (d *BuildLogDescriber) Describe(namespace, name string) (string, error) {
	return fmt.Sprintf("Name: %s/%s, Labels:", namespace, name), nil
}

// ImageDescriber generates information about a Image
type ImageDescriber struct {
	client.Interface
}

// Describe returns the description of an image
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

// Describe returns the description of an imageStreamTag
func (d *ImageStreamTagDescriber) Describe(namespace, name string) (string, error) {
	c := d.ImageStreamTags(namespace)
	repo, tag := parsers.ParseRepositoryTag(name)
	if tag == "" {
		// TODO use repo's preferred default, when that's coded
		tag = imageapi.DefaultImageTag
	}
	imageStreamTag, err := c.Get(repo, tag)
	if err != nil {
		return "", err
	}

	return describeImage(&imageStreamTag.Image)
}

// ImageStreamImageDescriber generates information about a ImageStreamImage (Image).
type ImageStreamImageDescriber struct {
	client.Interface
}

// Describe returns the description of an imageStreamImage
func (d *ImageStreamImageDescriber) Describe(namespace, name string) (string, error) {
	c := d.ImageStreamImages(namespace)
	repo, id := parsers.ParseRepositoryTag(name)
	imageStreamImage, err := c.Get(repo, id)
	if err != nil {
		return "", err
	}

	return describeImage(&imageStreamImage.Image)
}

// ImageStreamDescriber generates information about a ImageStream
type ImageStreamDescriber struct {
	client.Interface
}

// Describe returns the description of an imageStream
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

// Describe returns the description of a route
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

// Describe returns the description of a project
func (d *ProjectDescriber) Describe(namespace, name string) (string, error) {
	c := d.Projects()
	project, err := c.Get(name)
	if err != nil {
		return "", err
	}
	nodeSelector := ""
	if len(project.ObjectMeta.Annotations) > 0 {
		if ns, ok := project.ObjectMeta.Annotations["openshift.io/node-selector"]; ok {
			nodeSelector = ns
		}
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, project.ObjectMeta)
		formatString(out, "Display Name", project.Annotations["displayName"])
		formatString(out, "Status", project.Status.Phase)
		formatString(out, "Node Selector", nodeSelector)
		return nil
	})
}

// TemplateDescriber generates information about a template
type TemplateDescriber struct {
	client.Interface
	meta.MetadataAccessor
	runtime.ObjectTyper
	kctl.ObjectDescriber
}

// DescribeParameters prints out information about the parameters of a template
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

// describeObjects prints out information about the objects of a template
func (d *TemplateDescriber) describeObjects(objects []runtime.Object, out *tabwriter.Writer) {
	formatString(out, "Objects", " ")
	indent := "    "
	for _, obj := range objects {
		if d.ObjectDescriber != nil {
			output, err := d.DescribeObject(obj)
			if err != nil {
				fmt.Fprintf(out, "error: %v\n", err)
				continue
			}
			fmt.Fprint(out, output)
			fmt.Fprint(out, "\n")
			continue
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

// Describe returns the description of a template
func (d *TemplateDescriber) Describe(namespace, name string) (string, error) {
	c := d.Templates(namespace)
	template, err := c.Get(name)
	if err != nil {
		return "", err
	}
	return d.DescribeTemplate(template)
}

func (d *TemplateDescriber) DescribeTemplate(template *templateapi.Template) (string, error) {
	// TODO: write error?
	_ = runtime.DecodeList(template.Objects, kapi.Scheme, runtime.UnstructuredJSONScheme)

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, template.ObjectMeta)
		out.Write([]byte("\n"))
		out.Flush()
		d.DescribeParameters(template.Parameters, out)
		out.Write([]byte("\n"))
		formatString(out, "Object Labels", formatLabels(template.ObjectLabels))
		out.Write([]byte("\n"))
		out.Flush()
		d.describeObjects(template.Objects, out)
		return nil
	})
}

// IdentityDescriber generates information about a user
type IdentityDescriber struct {
	client.Interface
}

// Describe returns the description of an identity
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

// Describe returns the description of a userIdentity
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

// Describe returns the description of a user
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

// policy describers

// PolicyDescriber generates information about a Project
type PolicyDescriber struct {
	client.Interface
}

// Describe returns the description of a policy
// TODO make something a lot prettier
func (d *PolicyDescriber) Describe(namespace, name string) (string, error) {
	c := d.Policies(namespace)
	policy, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return DescribePolicy(policy)
}

func DescribePolicy(policy *authorizationapi.Policy) (string, error) {
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

// Describe returns the description of a role
func (d *RoleDescriber) Describe(namespace, name string) (string, error) {
	c := d.Roles(namespace)
	role, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return DescribeRole(role)
}

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

// PolicyBindingDescriber generates information about a Project
type PolicyBindingDescriber struct {
	client.Interface
}

// Describe returns the description of a policyBinding
func (d *PolicyBindingDescriber) Describe(namespace, name string) (string, error) {
	c := d.PolicyBindings(namespace)
	policyBinding, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return DescribePolicyBinding(policyBinding)
}

func DescribePolicyBinding(policyBinding *authorizationapi.PolicyBinding) (string, error) {
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

// Describe returns the description of a roleBinding
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

// ClusterPolicyDescriber generates information about a Project
type ClusterPolicyDescriber struct {
	client.Interface
}

// Describe returns the description of a policy
// TODO make something a lot prettier
func (d *ClusterPolicyDescriber) Describe(namespace, name string) (string, error) {
	c := d.ClusterPolicies()
	policy, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return DescribePolicy(authorizationapi.ToPolicy(policy))
}

type ClusterRoleDescriber struct {
	client.Interface
}

// Describe returns the description of a role
func (d *ClusterRoleDescriber) Describe(namespace, name string) (string, error) {
	c := d.ClusterRoles()
	role, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return DescribeRole(authorizationapi.ToRole(role))
}

// ClusterPolicyBindingDescriber generates information about a Project
type ClusterPolicyBindingDescriber struct {
	client.Interface
}

// Describe returns the description of a policyBinding
func (d *ClusterPolicyBindingDescriber) Describe(namespace, name string) (string, error) {
	c := d.ClusterPolicyBindings()
	policyBinding, err := c.Get(name)
	if err != nil {
		return "", err
	}

	return DescribePolicyBinding(authorizationapi.ToPolicyBinding(policyBinding))
}

// ClusterRoleBindingDescriber generates information about a Project
type ClusterRoleBindingDescriber struct {
	client.Interface
}

// Describe returns the description of a roleBinding
func (d *ClusterRoleBindingDescriber) Describe(namespace, name string) (string, error) {
	c := d.ClusterRoleBindings()
	roleBinding, err := c.Get(name)
	if err != nil {
		return "", err
	}

	role, err := d.ClusterRoles().Get(roleBinding.RoleRef.Name)
	return DescribeRoleBinding(authorizationapi.ToRoleBinding(roleBinding), authorizationapi.ToRole(role), err)
}
