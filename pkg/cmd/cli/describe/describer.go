package describe

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/pkg/units"

	"github.com/docker/docker/pkg/parsers"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	kctl "k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func describerMap(c *client.Client, kclient kclient.Interface, host string) map[string]kctl.Describer {
	m := map[string]kctl.Describer{
		"Build":                      &BuildDescriber{c, kclient},
		"BuildConfig":                &BuildConfigDescriber{c, host},
		"BuildLog":                   &BuildLogDescriber{c},
		"DeploymentConfig":           NewDeploymentConfigDescriber(c, kclient),
		"Identity":                   &IdentityDescriber{c},
		"Image":                      &ImageDescriber{c},
		"ImageStream":                &ImageStreamDescriber{c},
		"ImageStreamTag":             &ImageStreamTagDescriber{c},
		"ImageStreamImage":           &ImageStreamImageDescriber{c},
		"Route":                      &RouteDescriber{c},
		"Project":                    &ProjectDescriber{c, kclient},
		"Template":                   &TemplateDescriber{c, meta.NewAccessor(), kapi.Scheme, nil},
		"Policy":                     &PolicyDescriber{c},
		"PolicyBinding":              &PolicyBindingDescriber{c},
		"RoleBinding":                &RoleBindingDescriber{c},
		"Role":                       &RoleDescriber{c},
		"ClusterPolicy":              &ClusterPolicyDescriber{c},
		"ClusterPolicyBinding":       &ClusterPolicyBindingDescriber{c},
		"ClusterRoleBinding":         &ClusterRoleBindingDescriber{c},
		"ClusterRole":                &ClusterRoleDescriber{c},
		"User":                       &UserDescriber{c},
		"Group":                      &GroupDescriber{c.Groups()},
		"UserIdentityMapping":        &UserIdentityMappingDescriber{c},
		"SecurityContextConstraints": &SecurityContextConstraintsDescriber{c},
	}
	return m
}

// List of all resource types we can describe
func DescribableResources() []string {
	// Include describable resources in kubernetes
	keys := kctl.DescribableResources()

	for k := range describerMap(nil, nil, "") {
		resource := strings.ToLower(k)
		keys = append(keys, resource)
	}
	return keys
}

// DescriberFor returns a describer for a given kind of resource
func DescriberFor(kind string, c *client.Client, kclient kclient.Interface, host string) (kctl.Describer, bool) {
	f, ok := describerMap(c, kclient, host)[kind]
	if ok {
		return f, true
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
	if events == nil {
		events = &kapi.EventList{}
	}
	// get also pod events and merge it all into one list for describe
	if pod, err := d.kubeClient.Pods(namespace).Get(buildutil.GetBuildPodName(build)); err == nil {
		if podEvents, _ := d.kubeClient.Events(namespace).Search(pod); podEvents != nil {
			events.Items = append(events.Items, podEvents.Items...)
		}
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, build.ObjectMeta)
		if build.Status.Config != nil {
			formatString(out, "Build Config", build.Status.Config.Name)
		}
		if build.Status.StartTimestamp != nil {
			formatString(out, "Started", build.Status.StartTimestamp.Time)
		}
		if build.Status.CompletionTimestamp != nil {
			formatString(out, "Finished", build.Status.CompletionTimestamp.Time)
		}
		// Create the time object with second-level precision so we don't get
		// output like "duration: 1.2724395728934s"
		formatString(out, "Duration", describeBuildDuration(build))
		formatString(out, "Build Pod", buildutil.GetBuildPodName(build))
		describeBuildSpec(build.Spec, out)
		status := bold(build.Status.Phase)
		if build.Status.Message != "" {
			status += " (" + build.Status.Message + ")"
		}
		formatString(out, "Status", status)
		kctl.DescribeEvents(events, out)

		return nil
	})
}

func describeBuildDuration(build *buildapi.Build) string {
	t := util.Now().Rfc3339Copy()
	if build.Status.StartTimestamp == nil &&
		build.Status.CompletionTimestamp != nil &&
		(build.Status.Phase == buildapi.BuildPhaseCancelled ||
			build.Status.Phase == buildapi.BuildPhaseFailed ||
			build.Status.Phase == buildapi.BuildPhaseError) {
		// time a build waited for its pod before ultimately being canceled before that pod was created
		return fmt.Sprintf("waited for %s", build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(build.CreationTimestamp.Rfc3339Copy().Time))
	} else if build.Status.StartTimestamp == nil && build.Status.Phase != buildapi.BuildPhaseCancelled {
		// time a new build has been waiting for its pod to be created so it can run
		return fmt.Sprintf("waiting for %v", t.Sub(build.CreationTimestamp.Rfc3339Copy().Time))
	} else if build.Status.StartTimestamp != nil && build.Status.CompletionTimestamp == nil {
		// time a still running build has been running in a pod
		return fmt.Sprintf("running for %v", build.Status.Duration)
	}
	return fmt.Sprintf("%v", build.Status.Duration)
}

// BuildConfigDescriber generates information about a buildConfig
type BuildConfigDescriber struct {
	client.Interface
	host string
}

// TODO: remove when internal SourceBuildStrategyType is refactored to "Source"
func describeStrategy(strategyType buildapi.BuildStrategyType) buildapi.BuildStrategyType {
	if strategyType == buildapi.SourceBuildStrategyType {
		strategyType = buildapi.BuildStrategyType("Source")
	}
	return strategyType
}

func describeBuildSpec(p buildapi.BuildSpec, out *tabwriter.Writer) {
	formatString(out, "Strategy", describeStrategy(p.Strategy.Type))

	formatString(out, "Source Type", p.Source.Type)
	if p.Source.Dockerfile != nil {
		if len(strings.TrimSpace(*p.Source.Dockerfile)) == 0 {
			formatString(out, "Dockerfile", "")
		} else {
			fmt.Fprintf(out, "Dockerfile:\n")
			for _, s := range strings.Split(*p.Source.Dockerfile, "\n") {
				fmt.Fprintf(out, "  %s\n", s)
			}
		}
	}
	if p.Source.Git != nil {
		formatString(out, "URL", p.Source.Git.URI)
		if len(p.Source.Git.Ref) > 0 {
			formatString(out, "Ref", p.Source.Git.Ref)
		}
		if len(p.Source.ContextDir) > 0 {
			formatString(out, "ContextDir", p.Source.ContextDir)
		}
		if p.Source.SourceSecret != nil {
			formatString(out, "Source Secret", p.Source.SourceSecret.Name)
		}
		if p.Revision != nil && p.Revision.Git != nil {
			rev := p.Revision.Git
			formatString(out, "Commit", rev.Commit)
			if len(rev.Author.Name) != 0 {
				formatString(out, "Author", rev.Author.Name)
			}
			if len(rev.Committer.Name) != 0 {
				formatString(out, "Committer", rev.Committer.Name)
			}
			formatString(out, "Message", rev.Message)
		}
	}

	switch p.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		describeDockerStrategy(p.Strategy.DockerStrategy, out)
	case buildapi.SourceBuildStrategyType:
		describeSourceStrategy(p.Strategy.SourceStrategy, out)
	case buildapi.CustomBuildStrategyType:
		describeCustomStrategy(p.Strategy.CustomStrategy, out)
	}

	if p.Output.To != nil {
		if len(p.Output.To.Namespace) != 0 {
			formatString(out, "Output to", fmt.Sprintf("%s %s/%s", p.Output.To.Kind, p.Output.To.Namespace, p.Output.To.Name))
		} else {
			formatString(out, "Output to", fmt.Sprintf("%s %s", p.Output.To.Kind, p.Output.To.Name))
		}
	}

	if p.Output.PushSecret != nil {
		formatString(out, "Push Secret", p.Output.PushSecret.Name)
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

func describeSourceStrategy(s *buildapi.SourceBuildStrategy, out *tabwriter.Writer) {
	if len(s.From.Name) != 0 {
		if len(s.From.Namespace) != 0 {
			formatString(out, "From Image", fmt.Sprintf("%s %s/%s", s.From.Kind, s.From.Namespace, s.From.Name))
		} else {
			formatString(out, "From Image", fmt.Sprintf("%s %s", s.From.Kind, s.From.Name))
		}
	}
	if len(s.Scripts) != 0 {
		formatString(out, "Scripts", s.Scripts)
	}
	if s.PullSecret != nil {
		formatString(out, "Pull Secret Name", s.PullSecret.Name)
	}
	if s.Incremental {
		formatString(out, "Incremental Build", "yes")
	}
	if s.ForcePull {
		formatString(out, "Force Pull", "yes")
	}
}

func describeDockerStrategy(s *buildapi.DockerBuildStrategy, out *tabwriter.Writer) {
	if s.From != nil && len(s.From.Name) != 0 {
		if len(s.From.Namespace) != 0 {
			formatString(out, "From Image", fmt.Sprintf("%s %s/%s", s.From.Kind, s.From.Namespace, s.From.Name))
		} else {
			formatString(out, "From Image", fmt.Sprintf("%s %s", s.From.Kind, s.From.Name))
		}
	}
	if s.PullSecret != nil {
		formatString(out, "Pull Secret Name", s.PullSecret.Name)
	}
	if s.NoCache {
		formatString(out, "No Cache", "true")
	}
	if s.ForcePull {
		formatString(out, "Force Pull", "true")
	}
}

func describeCustomStrategy(s *buildapi.CustomBuildStrategy, out *tabwriter.Writer) {
	if len(s.From.Name) != 0 {
		if len(s.From.Namespace) != 0 {
			formatString(out, "Image Reference", fmt.Sprintf("%s %s/%s", s.From.Kind, s.From.Namespace, s.From.Name))
		} else {
			formatString(out, "Image Reference", fmt.Sprintf("%s %s", s.From.Kind, s.From.Name))
		}
	}
	if s.ExposeDockerSocket {
		formatString(out, "Expose Docker Socket", "yes")
	}
	if s.ForcePull {
		formatString(out, "Force Pull", "yes")
	}
	if s.PullSecret != nil {
		formatString(out, "Pull Secret Name", s.PullSecret.Name)
	}
	for i, env := range s.Env {
		if i == 0 {
			formatString(out, "Environment", formatEnv(env))
		} else {
			formatString(out, "", formatEnv(env))
		}
	}
}

// DescribeTriggers generates information about the triggers associated with a buildconfig
func (d *BuildConfigDescriber) DescribeTriggers(bc *buildapi.BuildConfig, out *tabwriter.Writer) {
	describeBuildTriggers(bc.Spec.Triggers, out)
	webhooks := webhookURL(bc, d.Interface)
	for whType, whURL := range webhooks {
		t := strings.Title(whType)
		formatString(out, "Webhook "+t, whURL)
	}
}

func describeBuildTriggers(triggers []buildapi.BuildTriggerPolicy, w *tabwriter.Writer) {
	if len(triggers) == 0 {
		formatString(w, "Triggered by", "<none>")
		return
	}

	labels := []string{}

	for _, t := range triggers {
		switch t.Type {
		case buildapi.GitHubWebHookBuildTriggerType, buildapi.GenericWebHookBuildTriggerType:
			continue
		case buildapi.ConfigChangeBuildTriggerType:
			labels = append(labels, "Config")
		case buildapi.ImageChangeBuildTriggerType:
			if t.ImageChange != nil && t.ImageChange.From != nil && len(t.ImageChange.From.Name) > 0 {
				labels = append(labels, fmt.Sprintf("Image(%s %s)", t.ImageChange.From.Kind, t.ImageChange.From.Name))
			} else {
				labels = append(labels, string(t.Type))
			}
		case "":
			labels = append(labels, "<unknown>")
		default:
			labels = append(labels, string(t.Type))
		}
	}

	desc := strings.Join(labels, ", ")
	formatString(w, "Triggered by", desc)
}

// Describe returns the description of a buildConfig
func (d *BuildConfigDescriber) Describe(namespace, name string) (string, error) {
	c := d.BuildConfigs(namespace)
	buildConfig, err := c.Get(name)
	if err != nil {
		return "", err
	}
	buildList, err := d.Builds(namespace).List(labels.SelectorFromSet(labels.Set{buildapi.BuildConfigLabel: name}), fields.Everything())
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, buildConfig.ObjectMeta)
		if buildConfig.Status.LastVersion == 0 {
			formatString(out, "Latest Version", "Never built")
		} else {
			formatString(out, "Latest Version", strconv.Itoa(buildConfig.Status.LastVersion))
		}
		describeBuildSpec(buildConfig.Spec.BuildSpec, out)
		d.DescribeTriggers(buildConfig, out)
		if len(buildList.Items) == 0 {
			return nil
		}
		fmt.Fprintf(out, "\nBuild\tStatus\tDuration\tCreation Time\n")

		builds := buildList.Items
		sort.Sort(sort.Reverse(buildapi.BuildSliceByCreationTimestamp(builds)))

		for i, build := range builds {
			fmt.Fprintf(out, "%s \t%s \t%v \t%v\n",
				build.Name,
				strings.ToLower(string(build.Status.Phase)),
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

	return describeImage(image, "")
}

func describeImage(image *imageapi.Image, imageName string) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, image.ObjectMeta)
		formatString(out, "Docker Image", image.DockerImageReference)
		if len(imageName) > 0 {
			formatString(out, "Image Name", imageName)
		}
		formatString(out, "Parent Image", image.DockerImageMetadata.Parent)
		formatString(out, "Layer Size", units.HumanSize(float64(image.DockerImageMetadata.Size)))
		formatString(out, "Image Created", fmt.Sprintf("%s ago", formatRelativeTime(image.DockerImageMetadata.Created.Time)))
		formatString(out, "Author", image.DockerImageMetadata.Author)
		formatString(out, "Arch", image.DockerImageMetadata.Architecture)
		describeDockerImage(out, image.DockerImageMetadata.Config)
		return nil
	})
}

func describeDockerImage(out *tabwriter.Writer, image *imageapi.DockerConfig) {
	if image == nil {
		return
	}
	hasCommand := false
	if len(image.Entrypoint) > 0 {
		hasCommand = true
		formatString(out, "Entrypoint", strings.Join(image.Entrypoint, " "))
	}
	if len(image.Cmd) > 0 {
		hasCommand = true
		formatString(out, "Command", strings.Join(image.Cmd, " "))
	}
	if !hasCommand {
		formatString(out, "Command", "")
	}
	formatString(out, "Working Dir", image.WorkingDir)
	formatString(out, "User", image.User)
	ports := sets.NewString()
	for k := range image.ExposedPorts {
		ports.Insert(k)
	}
	formatString(out, "Exposes Ports", strings.Join(ports.List(), ", "))
	for i, env := range image.Env {
		if i == 0 {
			formatString(out, "Environment", env)
		} else {
			fmt.Fprintf(out, "\t%s\n", env)
		}
	}
	volumes := sets.NewString()
	for k := range image.Volumes {
		volumes.Insert(k)
	}
	for i, volume := range volumes.List() {
		if i == 0 {
			formatString(out, "Volumes", volume)
		} else {
			fmt.Fprintf(out, "\t%s\n", volume)
		}
	}
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

	return describeImage(&imageStreamTag.Image, imageStreamTag.Image.Name)
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

	return describeImage(&imageStreamImage.Image, imageStreamImage.Image.Name)
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
		formatString(out, "Docker Pull Spec", imageStream.Status.DockerImageRepository)
		formatImageStreamTags(out, imageStream)
		return nil
	})
}

type SecurityContextConstraintsDescriber struct {
	client.Interface
}

func (d *SecurityContextConstraintsDescriber) Describe(namespace, name string) (string, error) {
	scc, err := d.SecurityContextConstraints().Get(name)
	if err != nil {
		return "", err
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, scc.ObjectMeta)
		formatString(out, "AllowPrivilegedContainer", scc.AllowPrivilegedContainer)
		formatString(out, "AllowHostPorts", scc.AllowHostPorts)
		formatString(out, "AllowHostNetwork", scc.AllowHostNetwork)
		formatString(out, "AllowHostDirVolumePlugin", scc.AllowHostDirVolumePlugin)
		formatString(out, "AllowedCapabilities", scc.AllowedCapabilities)
		formatString(out, "RunAsUser", scc.RunAsUser.Type)
		formatString(out, "SELinuxContext", scc.SELinuxContext.Type)
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
		formatString(out, "Host", route.Spec.Host)
		formatString(out, "Path", route.Spec.Path)
		formatString(out, "Service", route.Spec.To.Name)

		tlsTerm := ""
		if route.Spec.TLS != nil {
			tlsTerm = string(route.Spec.TLS.Termination)
		}
		formatString(out, "TLS Termination", tlsTerm)
		return nil
	})
}

// ProjectDescriber generates information about a Project
type ProjectDescriber struct {
	osClient   client.Interface
	kubeClient kclient.Interface
}

// Describe returns the description of a project
func (d *ProjectDescriber) Describe(namespace, name string) (string, error) {
	projectsClient := d.osClient.Projects()
	project, err := projectsClient.Get(name)
	if err != nil {
		return "", err
	}
	resourceQuotasClient := d.kubeClient.ResourceQuotas(name)
	resourceQuotaList, err := resourceQuotasClient.List(labels.Everything())
	if err != nil {
		return "", err
	}
	limitRangesClient := d.kubeClient.LimitRanges(name)
	limitRangeList, err := limitRangesClient.List(labels.Everything())
	if err != nil {
		return "", err
	}

	nodeSelector := ""
	if len(project.ObjectMeta.Annotations) > 0 {
		if ns, ok := project.ObjectMeta.Annotations[projectapi.ProjectNodeSelector]; ok {
			nodeSelector = ns
		}
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, project.ObjectMeta)
		formatString(out, "Display Name", project.Annotations[projectapi.ProjectDisplayName])
		formatString(out, "Description", project.Annotations[projectapi.ProjectDescription])
		formatString(out, "Status", project.Status.Phase)
		formatString(out, "Node Selector", nodeSelector)
		if len(resourceQuotaList.Items) == 0 {
			formatString(out, "Quota", "")
		} else {
			fmt.Fprintf(out, "Quota:\n")
			for i := range resourceQuotaList.Items {
				resourceQuota := &resourceQuotaList.Items[i]
				fmt.Fprintf(out, "\tName:\t%s\n", resourceQuota.Name)
				fmt.Fprintf(out, "\tResource\tUsed\tHard\n")
				fmt.Fprintf(out, "\t--------\t----\t----\n")

				resources := []kapi.ResourceName{}
				for resource := range resourceQuota.Status.Hard {
					resources = append(resources, resource)
				}
				sort.Sort(kctl.SortableResourceNames(resources))

				msg := "\t%v\t%v\t%v\n"
				for i := range resources {
					resource := resources[i]
					hardQuantity := resourceQuota.Status.Hard[resource]
					usedQuantity := resourceQuota.Status.Used[resource]
					fmt.Fprintf(out, msg, resource, usedQuantity.String(), hardQuantity.String())
				}
			}
		}
		if len(limitRangeList.Items) == 0 {
			formatString(out, "Resource limits", "")
		} else {
			fmt.Fprintf(out, "Resource limits:\n")
			for i := range limitRangeList.Items {
				limitRange := &limitRangeList.Items[i]
				fmt.Fprintf(out, "\tName:\t%s\n", limitRange.Name)
				fmt.Fprintf(out, "\tType\tResource\tMin\tMax\tDefault\n")
				fmt.Fprintf(out, "\t----\t--------\t---\t---\t---\n")
				for i := range limitRange.Spec.Limits {
					item := limitRange.Spec.Limits[i]
					maxResources := item.Max
					minResources := item.Min
					defaultResources := item.Default

					set := map[kapi.ResourceName]bool{}
					for k := range maxResources {
						set[k] = true
					}
					for k := range minResources {
						set[k] = true
					}
					for k := range defaultResources {
						set[k] = true
					}

					for k := range set {
						// if no value is set, we output -
						maxValue := "-"
						minValue := "-"
						defaultValue := "-"

						maxQuantity, maxQuantityFound := maxResources[k]
						if maxQuantityFound {
							maxValue = maxQuantity.String()
						}

						minQuantity, minQuantityFound := minResources[k]
						if minQuantityFound {
							minValue = minQuantity.String()
						}

						defaultQuantity, defaultQuantityFound := defaultResources[k]
						if defaultQuantityFound {
							defaultValue = defaultQuantity.String()
						}

						msg := "\t%v\t%v\t%v\t%v\t%v\n"
						fmt.Fprintf(out, msg, item.Type, k, minValue, maxValue, defaultValue)
					}
				}
			}
		}
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
		if len(p.DisplayName) > 0 {
			formatString(out, indent+"Display Name", p.DisplayName)
		}
		if len(p.Description) > 0 {
			formatString(out, indent+"Description", p.Description)
		}
		formatString(out, indent+"Required", p.Required)
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
				if !sets.NewString(resolvedUser.Identities...).Has(name) {
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

// GroupDescriber generates information about a group
type GroupDescriber struct {
	c client.GroupInterface
}

// Describe returns the description of a group
func (d *GroupDescriber) Describe(namespace, name string) (string, error) {
	group, err := d.c.Get(name)
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, group.ObjectMeta)

		if len(group.Users) == 0 {
			formatString(out, "Users", "<none>")
		} else {
			for i, user := range group.Users {
				if i == 0 {
					formatString(out, "Users", user)
				} else {
					fmt.Fprintf(out, "           \t%s\n", user)
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
		for _, key := range sets.KeySet(reflect.ValueOf(policy.Roles)).List() {
			role := policy.Roles[key]
			fmt.Fprint(out, key+"\t"+policyRuleHeadings+"\n")
			for _, rule := range role.Rules {
				describePolicyRule(out, rule, "\t")
			}
		}

		return nil
	})
}

const policyRuleHeadings = "Verbs\tResources\tResource Names\tNon-Resource URLs\tExtension"

func describePolicyRule(out *tabwriter.Writer, rule authorizationapi.PolicyRule, indent string) {
	extensionString := ""
	if rule.AttributeRestrictions != (runtime.EmbeddedObject{}) {
		extensionString = fmt.Sprintf("%#v", rule.AttributeRestrictions.Object)

		buffer := new(bytes.Buffer)
		printer := NewHumanReadablePrinter(true, false, false, false, []string{})
		if err := printer.PrintObj(rule.AttributeRestrictions.Object, buffer); err == nil {
			extensionString = strings.TrimSpace(buffer.String())
		}
	}

	fmt.Fprintf(out, indent+"%v\t%v\t%v\t%v\t%v\n",
		rule.Verbs.List(),
		rule.Resources.List(),
		rule.ResourceNames.List(),
		rule.NonResourceURLs.List(),
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
		for _, key := range sets.KeySet(reflect.ValueOf(policyBinding.RoleBindings)).List() {
			roleBinding := policyBinding.RoleBindings[key]
			users, groups, sas, others := authorizationapi.SubjectsStrings(roleBinding.Namespace, roleBinding.Subjects)

			formatString(out, "RoleBinding["+key+"]", " ")
			formatString(out, "\tRole", roleBinding.RoleRef.Name)
			formatString(out, "\tUsers", strings.Join(users, ", "))
			formatString(out, "\tGroups", strings.Join(groups, ", "))
			formatString(out, "\tServiceAccounts", strings.Join(sas, ", "))
			formatString(out, "\tSubjects", strings.Join(others, ", "))
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

	var role *authorizationapi.Role
	if len(roleBinding.RoleRef.Namespace) == 0 {
		var clusterRole *authorizationapi.ClusterRole
		clusterRole, err = d.ClusterRoles().Get(roleBinding.RoleRef.Name)
		role = authorizationapi.ToRole(clusterRole)
	} else {
		role, err = d.Roles(roleBinding.RoleRef.Namespace).Get(roleBinding.RoleRef.Name)
	}

	return DescribeRoleBinding(roleBinding, role, err)
}

// DescribeRoleBinding prints out information about a role binding and its associated role
func DescribeRoleBinding(roleBinding *authorizationapi.RoleBinding, role *authorizationapi.Role, err error) (string, error) {
	users, groups, sas, others := authorizationapi.SubjectsStrings(roleBinding.Namespace, roleBinding.Subjects)

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, roleBinding.ObjectMeta)

		formatString(out, "Role", roleBinding.RoleRef.Namespace+"/"+roleBinding.RoleRef.Name)
		formatString(out, "Users", strings.Join(users, ", "))
		formatString(out, "Groups", strings.Join(groups, ", "))
		formatString(out, "ServiceAccounts", strings.Join(sas, ", "))
		formatString(out, "Subjects", strings.Join(others, ", "))

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
