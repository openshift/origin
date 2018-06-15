package describe

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	units "github.com/docker/go-units"
	"github.com/golang/glog"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kprinters "k8s.io/kubernetes/pkg/printers"
	kinternalprinters "k8s.io/kubernetes/pkg/printers/internalversion"

	oapi "github.com/openshift/origin/pkg/api"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oauthorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	onetworkclient "github.com/openshift/origin/pkg/network/generated/internalclientset/typed/network/internalversion"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset/typed/quota/internalversion"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeclient "github.com/openshift/origin/pkg/route/generated/internalclientset/typed/route/internalversion"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

func describerMap(clientConfig *rest.Config, kclient kclientset.Interface, host string, withCoreGroup bool) map[schema.GroupKind]kprinters.Describer {
	// FIXME: This should use the client factory
	// we can't fail and we can't log at a normal level because this is sometimes called with `nils` for help :(
	oauthorizationClient, err := oauthorizationclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	onetworkClient, err := onetworkclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	userClient, err := userclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	quotaClient, err := quotaclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	imageClient, err := imageclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	appsClient, err := appsclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	buildClient, err := buildclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	templateClient, err := templateclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	routeClient, err := routeclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	projectClient, err := projectclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	oauthClient, err := oauthclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}
	securityClient, err := securityclient.NewForConfig(clientConfig)
	if err != nil {
		glog.V(1).Info(err)
	}

	m := map[schema.GroupKind]kprinters.Describer{
		buildapi.Kind("Build"):                          &BuildDescriber{buildClient, kclient},
		buildapi.Kind("BuildConfig"):                    &BuildConfigDescriber{buildClient, kclient, host},
		appsapi.Kind("DeploymentConfig"):                &DeploymentConfigDescriber{appsClient, kclient, nil},
		imageapi.Kind("Image"):                          &ImageDescriber{imageClient},
		imageapi.Kind("ImageStream"):                    &ImageStreamDescriber{imageClient},
		imageapi.Kind("ImageStreamTag"):                 &ImageStreamTagDescriber{imageClient},
		imageapi.Kind("ImageStreamImage"):               &ImageStreamImageDescriber{imageClient},
		routeapi.Kind("Route"):                          &RouteDescriber{routeClient, kclient},
		projectapi.Kind("Project"):                      &ProjectDescriber{projectClient, kclient},
		templateapi.Kind("Template"):                    &TemplateDescriber{templateClient, meta.NewAccessor(), legacyscheme.Scheme, nil},
		templateapi.Kind("TemplateInstance"):            &TemplateInstanceDescriber{kclient, templateClient, nil},
		authorizationapi.Kind("RoleBinding"):            &RoleBindingDescriber{oauthorizationClient},
		authorizationapi.Kind("Role"):                   &RoleDescriber{oauthorizationClient},
		authorizationapi.Kind("ClusterRoleBinding"):     &ClusterRoleBindingDescriber{oauthorizationClient},
		authorizationapi.Kind("ClusterRole"):            &ClusterRoleDescriber{oauthorizationClient},
		authorizationapi.Kind("RoleBindingRestriction"): &RoleBindingRestrictionDescriber{oauthorizationClient},
		oauthapi.Kind("OAuthAccessToken"):               &OAuthAccessTokenDescriber{oauthClient},
		userapi.Kind("Identity"):                        &IdentityDescriber{userClient},
		userapi.Kind("User"):                            &UserDescriber{userClient},
		userapi.Kind("Group"):                           &GroupDescriber{userClient},
		userapi.Kind("UserIdentityMapping"):             &UserIdentityMappingDescriber{userClient},
		quotaapi.Kind("ClusterResourceQuota"):           &ClusterQuotaDescriber{quotaClient},
		quotaapi.Kind("AppliedClusterResourceQuota"):    &AppliedClusterQuotaDescriber{quotaClient},
		networkapi.Kind("ClusterNetwork"):               &ClusterNetworkDescriber{onetworkClient},
		networkapi.Kind("HostSubnet"):                   &HostSubnetDescriber{onetworkClient},
		networkapi.Kind("NetNamespace"):                 &NetNamespaceDescriber{onetworkClient},
		networkapi.Kind("EgressNetworkPolicy"):          &EgressNetworkPolicyDescriber{onetworkClient},
		securityapi.Kind("SecurityContextConstraints"):  &SecurityContextConstraintsDescriber{securityClient},
	}

	// Register the legacy ("core") API group for all kinds as well.
	if withCoreGroup {
		for _, t := range legacyscheme.Scheme.KnownTypes(oapi.SchemeGroupVersion) {
			coreKind := oapi.SchemeGroupVersion.WithKind(t.Name())
			for g, d := range m {
				if g.Kind == coreKind.Kind {
					m[oapi.Kind(g.Kind)] = d
				}
			}
		}
	}
	return m
}

// DescribableResources lists all of the resource types we can describe
func DescribableResources() []string {
	// Include describable resources in kubernetes
	keys := kinternalprinters.DescribableResources()

	for k := range describerMap(&rest.Config{}, nil, "", false) {
		resource := strings.ToLower(k.Kind)
		keys = append(keys, resource)
	}
	return keys
}

// DescriberFor returns a describer for a given kind of resource
func DescriberFor(kind schema.GroupKind, clientConfig *rest.Config, kclient kclientset.Interface, host string) (kprinters.Describer, bool) {
	f, ok := describerMap(clientConfig, kclient, host, true)[kind]
	if ok {
		return f, true
	}
	return nil, false
}

// BuildDescriber generates information about a build
type BuildDescriber struct {
	buildClient buildclient.BuildInterface
	kubeClient  kclientset.Interface
}

// Describe returns the description of a build
func (d *BuildDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.buildClient.Builds(namespace)
	build, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	events, _ := d.kubeClient.Core().Events(namespace).Search(legacyscheme.Scheme, build)
	if events == nil {
		events = &kapi.EventList{}
	}
	// get also pod events and merge it all into one list for describe
	if pod, err := d.kubeClient.Core().Pods(namespace).Get(buildapi.GetBuildPodName(build), metav1.GetOptions{}); err == nil {
		if podEvents, _ := d.kubeClient.Core().Events(namespace).Search(legacyscheme.Scheme, pod); podEvents != nil {
			events.Items = append(events.Items, podEvents.Items...)
		}
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, build.ObjectMeta)

		fmt.Fprintln(out, "")

		status := bold(build.Status.Phase)
		if build.Status.Message != "" {
			status += " (" + build.Status.Message + ")"
		}
		formatString(out, "Status", status)

		if build.Status.StartTimestamp != nil && !build.Status.StartTimestamp.IsZero() {
			formatString(out, "Started", build.Status.StartTimestamp.Time.Format(time.RFC1123))
		}

		// Create the time object with second-level precision so we don't get
		// output like "duration: 1.2724395728934s"
		formatString(out, "Duration", describeBuildDuration(build))

		for _, stage := range build.Status.Stages {
			duration := stage.StartTime.Time.Add(time.Duration(stage.DurationMilliseconds * int64(time.Millisecond))).Round(time.Second).Sub(stage.StartTime.Time.Round(time.Second))
			formatString(out, fmt.Sprintf("  %v", stage.Name), fmt.Sprintf("  %v", duration))
		}

		fmt.Fprintln(out, "")

		if build.Status.Config != nil {
			formatString(out, "Build Config", build.Status.Config.Name)
		}
		formatString(out, "Build Pod", buildapi.GetBuildPodName(build))

		if build.Status.Output.To != nil && len(build.Status.Output.To.ImageDigest) > 0 {
			formatString(out, "Image Digest", build.Status.Output.To.ImageDigest)
		}

		describeCommonSpec(build.Spec.CommonSpec, out)
		describeBuildTriggerCauses(build.Spec.TriggeredBy, out)
		if len(build.Status.LogSnippet) != 0 {
			formatString(out, "Log Tail", build.Status.LogSnippet)
		}
		if settings.ShowEvents {
			kinternalprinters.DescribeEvents(events, kinternalprinters.NewPrefixWriter(out))
		}

		return nil
	})
}

func describeBuildDuration(build *buildapi.Build) string {
	t := metav1.Now().Rfc3339Copy()
	if build.Status.StartTimestamp == nil &&
		build.Status.CompletionTimestamp != nil &&
		(build.Status.Phase == buildapi.BuildPhaseCancelled ||
			build.Status.Phase == buildapi.BuildPhaseFailed ||
			build.Status.Phase == buildapi.BuildPhaseError) {
		// time a build waited for its pod before ultimately being cancelled before that pod was created
		return fmt.Sprintf("waited for %s", build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(build.CreationTimestamp.Rfc3339Copy().Time))
	} else if build.Status.StartTimestamp == nil && build.Status.Phase != buildapi.BuildPhaseCancelled {
		// time a new build has been waiting for its pod to be created so it can run
		return fmt.Sprintf("waiting for %v", t.Sub(build.CreationTimestamp.Rfc3339Copy().Time))
	} else if build.Status.StartTimestamp != nil && build.Status.CompletionTimestamp == nil {
		// time a still running build has been running in a pod
		duration := metav1.Now().Rfc3339Copy().Time.Sub(build.Status.StartTimestamp.Rfc3339Copy().Time)
		return fmt.Sprintf("running for %v", duration)
	} else if build.Status.CompletionTimestamp == nil &&
		build.Status.StartTimestamp == nil &&
		build.Status.Phase == buildapi.BuildPhaseCancelled {
		return "<none>"
	}

	duration := build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(build.Status.StartTimestamp.Rfc3339Copy().Time)
	return fmt.Sprintf("%v", duration)
}

// BuildConfigDescriber generates information about a buildConfig
type BuildConfigDescriber struct {
	buildClient buildclient.BuildInterface
	kubeClient  kclientset.Interface
	host        string
}

func nameAndNamespace(ns, name string) string {
	if len(ns) != 0 {
		return fmt.Sprintf("%s/%s", ns, name)
	}
	return name
}

func describeCommonSpec(p buildapi.CommonSpec, out *tabwriter.Writer) {
	formatString(out, "\nStrategy", buildapi.StrategyType(p.Strategy))
	noneType := true
	if p.Source.Git != nil {
		noneType = false
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
		squashGitInfo(p.Revision, out)
	}
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
	switch {
	case p.Strategy.DockerStrategy != nil:
		describeDockerStrategy(p.Strategy.DockerStrategy, out)
	case p.Strategy.SourceStrategy != nil:
		describeSourceStrategy(p.Strategy.SourceStrategy, out)
	case p.Strategy.CustomStrategy != nil:
		describeCustomStrategy(p.Strategy.CustomStrategy, out)
	case p.Strategy.JenkinsPipelineStrategy != nil:
		describeJenkinsPipelineStrategy(p.Strategy.JenkinsPipelineStrategy, out)
	}

	if p.Output.To != nil {
		formatString(out, "Output to", fmt.Sprintf("%s %s", p.Output.To.Kind, nameAndNamespace(p.Output.To.Namespace, p.Output.To.Name)))
	}

	if p.Source.Binary != nil {
		noneType = false
		if len(p.Source.Binary.AsFile) > 0 {
			formatString(out, "Binary", fmt.Sprintf("provided as file %q on build", p.Source.Binary.AsFile))
		} else {
			formatString(out, "Binary", "provided on build")
		}
	}

	if len(p.Source.Secrets) > 0 {
		result := []string{}
		for _, s := range p.Source.Secrets {
			result = append(result, fmt.Sprintf("%s->%s", s.Secret.Name, filepath.Clean(s.DestinationDir)))
		}
		formatString(out, "Build Secrets", strings.Join(result, ","))
	}
	if len(p.Source.Images) == 1 && len(p.Source.Images[0].Paths) == 1 {
		noneType = false
		image := p.Source.Images[0]
		path := image.Paths[0]
		formatString(out, "Image Source", fmt.Sprintf("copies %s from %s to %s", path.SourcePath, nameAndNamespace(image.From.Namespace, image.From.Name), path.DestinationDir))
	} else {
		for _, image := range p.Source.Images {
			noneType = false
			formatString(out, "Image Source", fmt.Sprintf("%s", nameAndNamespace(image.From.Namespace, image.From.Name)))
			for _, path := range image.Paths {
				fmt.Fprintf(out, "\t- %s -> %s\n", path.SourcePath, path.DestinationDir)
			}
			for _, name := range image.As {
				fmt.Fprintf(out, "\t- as %s\n", name)
			}
		}
	}

	if noneType {
		formatString(out, "Empty Source", "no input source provided")
	}

	describePostCommitHook(p.PostCommit, out)

	if p.Output.PushSecret != nil {
		formatString(out, "Push Secret", p.Output.PushSecret.Name)
	}

	if p.CompletionDeadlineSeconds != nil {
		formatString(out, "Fail Build After", time.Duration(*p.CompletionDeadlineSeconds)*time.Second)
	}
}

func describePostCommitHook(hook buildapi.BuildPostCommitSpec, out *tabwriter.Writer) {
	command := hook.Command
	args := hook.Args
	script := hook.Script
	if len(command) == 0 && len(args) == 0 && len(script) == 0 {
		// Post commit hook is not set, nothing to do.
		return
	}
	if len(script) != 0 {
		command = []string{"/bin/sh", "-ic"}
		if len(args) > 0 {
			args = append([]string{script, command[0]}, args...)
		} else {
			args = []string{script}
		}
	}
	if len(command) == 0 {
		command = []string{"<image-entrypoint>"}
	}
	all := append(command, args...)
	for i, v := range all {
		all[i] = fmt.Sprintf("%q", v)
	}
	formatString(out, "Post Commit Hook", fmt.Sprintf("[%s]", strings.Join(all, ", ")))
}

func describeSourceStrategy(s *buildapi.SourceBuildStrategy, out *tabwriter.Writer) {
	if len(s.From.Name) != 0 {
		formatString(out, "From Image", fmt.Sprintf("%s %s", s.From.Kind, nameAndNamespace(s.From.Namespace, s.From.Name)))
	}
	if len(s.Scripts) != 0 {
		formatString(out, "Scripts", s.Scripts)
	}
	if s.PullSecret != nil {
		formatString(out, "Pull Secret Name", s.PullSecret.Name)
	}
	if s.Incremental != nil && *s.Incremental {
		formatString(out, "Incremental Build", "yes")
	}
	if s.ForcePull {
		formatString(out, "Force Pull", "yes")
	}
}

func describeDockerStrategy(s *buildapi.DockerBuildStrategy, out *tabwriter.Writer) {
	if s.From != nil && len(s.From.Name) != 0 {
		formatString(out, "From Image", fmt.Sprintf("%s %s", s.From.Kind, nameAndNamespace(s.From.Namespace, s.From.Name)))
	}
	if len(s.DockerfilePath) != 0 {
		formatString(out, "Dockerfile Path", s.DockerfilePath)
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
		formatString(out, "Image Reference", fmt.Sprintf("%s %s", s.From.Kind, nameAndNamespace(s.From.Namespace, s.From.Name)))
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

func describeJenkinsPipelineStrategy(s *buildapi.JenkinsPipelineBuildStrategy, out *tabwriter.Writer) {
	if len(s.JenkinsfilePath) != 0 {
		formatString(out, "Jenkinsfile path", s.JenkinsfilePath)
	}
	if len(s.Jenkinsfile) != 0 {
		fmt.Fprintf(out, "Jenkinsfile contents:\n")
		for _, s := range strings.Split(s.Jenkinsfile, "\n") {
			fmt.Fprintf(out, "  %s\n", s)
		}
	}
	if len(s.Jenkinsfile) == 0 && len(s.JenkinsfilePath) == 0 {
		formatString(out, "Jenkinsfile", "from source repository root")
	}
}

// DescribeTriggers generates information about the triggers associated with a
// buildconfig
func (d *BuildConfigDescriber) DescribeTriggers(bc *buildapi.BuildConfig, out *tabwriter.Writer) {
	describeBuildTriggers(bc.Spec.Triggers, bc.Name, bc.Namespace, out, d)
}

func describeBuildTriggers(triggers []buildapi.BuildTriggerPolicy, name, namespace string, w *tabwriter.Writer, d *BuildConfigDescriber) {
	if len(triggers) == 0 {
		formatString(w, "Triggered by", "<none>")
		return
	}

	labels := []string{}

	for _, t := range triggers {
		switch t.Type {
		case buildapi.GitHubWebHookBuildTriggerType, buildapi.GenericWebHookBuildTriggerType, buildapi.GitLabWebHookBuildTriggerType, buildapi.BitbucketWebHookBuildTriggerType:
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

	webHooks := webHooksDescribe(triggers, name, namespace, d.buildClient.RESTClient())
	for webHookType, webHookDesc := range webHooks {
		fmt.Fprintf(w, "Webhook %s:\n", strings.Title(webHookType))
		for _, trigger := range webHookDesc {
			fmt.Fprintf(w, "\tURL:\t%s\n", trigger.URL)
			if webHookType == string(buildapi.GenericWebHookBuildTriggerType) && trigger.AllowEnv != nil {
				fmt.Fprintf(w, fmt.Sprintf("\t%s:\t%v\n", "AllowEnv", *trigger.AllowEnv))
			}
		}
	}
}

// Describe returns the description of a buildConfig
func (d *BuildConfigDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.buildClient.BuildConfigs(namespace)
	buildConfig, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	buildList, err := d.buildClient.Builds(namespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	buildList.Items = buildapi.FilterBuilds(buildList.Items, buildapi.ByBuildConfigPredicate(name))

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, buildConfig.ObjectMeta)
		if buildConfig.Status.LastVersion == 0 {
			formatString(out, "Latest Version", "Never built")
		} else {
			formatString(out, "Latest Version", strconv.FormatInt(buildConfig.Status.LastVersion, 10))
		}
		describeCommonSpec(buildConfig.Spec.CommonSpec, out)
		formatString(out, "\nBuild Run Policy", string(buildConfig.Spec.RunPolicy))
		d.DescribeTriggers(buildConfig, out)

		if len(buildList.Items) > 0 {
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
		}

		if settings.ShowEvents {
			events, _ := d.kubeClient.Core().Events(namespace).Search(legacyscheme.Scheme, buildConfig)
			if events != nil {
				fmt.Fprint(out, "\n")
				kinternalprinters.DescribeEvents(events, kinternalprinters.NewPrefixWriter(out))
			}
		}
		return nil
	})
}

// OAuthAccessTokenDescriber generates information about an OAuth Acess Token (OAuth)
type OAuthAccessTokenDescriber struct {
	client oauthclient.OauthInterface
}

func (d *OAuthAccessTokenDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.client.OAuthAccessTokens()
	oAuthAccessToken, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var timeCreated time.Time = oAuthAccessToken.ObjectMeta.CreationTimestamp.Time
	expires := "never"
	if oAuthAccessToken.ExpiresIn > 0 {
		var timeExpired time.Time = timeCreated.Add(time.Duration(oAuthAccessToken.ExpiresIn) * time.Second)
		expires = formatToHumanDuration(timeExpired.Sub(time.Now()))
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, oAuthAccessToken.ObjectMeta)
		formatString(out, "Scopes", oAuthAccessToken.Scopes)
		formatString(out, "Expires In", expires)
		formatString(out, "User Name", oAuthAccessToken.UserName)
		formatString(out, "User UID", oAuthAccessToken.UserUID)
		formatString(out, "Client Name", oAuthAccessToken.ClientName)

		return nil
	})
}

// ImageDescriber generates information about a Image
type ImageDescriber struct {
	c imageclient.ImageInterface
}

// Describe returns the description of an image
func (d *ImageDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.Images()
	image, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return DescribeImage(image, "")
}

func describeImageSignature(s imageapi.ImageSignature, out *tabwriter.Writer) error {
	formatString(out, "\tName", s.Name)
	formatString(out, "\tType", s.Type)
	if s.IssuedBy == nil {
		// FIXME: Make this constant
		formatString(out, "\tStatus", "Unverified")
	} else {
		formatString(out, "\tStatus", "Verified")
		formatString(out, "\tIssued By", s.IssuedBy.CommonName)
		if len(s.Conditions) > 0 {
			for _, c := range s.Conditions {
				formatString(out, "\t", fmt.Sprintf("Signature is %s (%s on %s)", string(c.Type), c.Message, fmt.Sprintf("%s", c.LastProbeTime)))
			}
		}
	}
	return nil
}

func DescribeImage(image *imageapi.Image, imageName string) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		if len(imageName) > 0 {
			formatString(out, "Image Name", imageName)
		}
		formatString(out, "Docker Image", image.DockerImageReference)
		formatString(out, "Name", image.Name)
		if !image.CreationTimestamp.IsZero() {
			formatTime(out, "Created", image.CreationTimestamp.Time)
		}
		if len(image.Labels) > 0 {
			formatMapStringString(out, "Labels", image.Labels)
		}
		if len(image.Annotations) > 0 {
			formatAnnotations(out, image.ObjectMeta, "")
		}

		switch l := len(image.DockerImageLayers); l {
		case 0:
			// legacy case, server does not know individual layers
			formatString(out, "Layer Size", units.HumanSize(float64(image.DockerImageMetadata.Size)))
		case 1:
			formatString(out, "Image Size", units.HumanSize(float64(image.DockerImageMetadata.Size)))
		default:
			info := []string{}
			if image.DockerImageLayers[0].LayerSize > 0 {
				info = append(info, fmt.Sprintf("first layer %s", units.HumanSize(float64(image.DockerImageLayers[0].LayerSize))))
			}
			for i := l - 1; i > 0; i-- {
				if image.DockerImageLayers[i].LayerSize == 0 {
					continue
				}
				info = append(info, fmt.Sprintf("last binary layer %s", units.HumanSize(float64(image.DockerImageLayers[i].LayerSize))))
				break
			}
			if len(info) > 0 {
				formatString(out, "Image Size", fmt.Sprintf("%s (%s)", units.HumanSize(float64(image.DockerImageMetadata.Size)), strings.Join(info, ", ")))
			} else {
				formatString(out, "Image Size", units.HumanSize(float64(image.DockerImageMetadata.Size)))
			}
		}
		if len(image.Signatures) > 0 {
			for _, s := range image.Signatures {
				formatString(out, "Image Signatures", " ")
				if err := describeImageSignature(s, out); err != nil {
					return err
				}
			}
		}
		//formatString(out, "Parent Image", image.DockerImageMetadata.Parent)
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
	formatMapStringString(out, "Docker Labels", image.Labels)
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
	c imageclient.ImageInterface
}

// Describe returns the description of an imageStreamTag
func (d *ImageStreamTagDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.ImageStreamTags(namespace)
	repo, tag, err := imageapi.ParseImageStreamTagName(name)
	if err != nil {
		return "", err
	}
	if len(tag) == 0 {
		// TODO use repo's preferred default, when that's coded
		tag = imageapi.DefaultImageTag
	}
	imageStreamTag, err := c.Get(repo+":"+tag, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return DescribeImage(&imageStreamTag.Image, imageStreamTag.Image.Name)
}

// ImageStreamImageDescriber generates information about a ImageStreamImage (Image).
type ImageStreamImageDescriber struct {
	c imageclient.ImageInterface
}

// Describe returns the description of an imageStreamImage
func (d *ImageStreamImageDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.ImageStreamImages(namespace)
	imageStreamImage, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return DescribeImage(&imageStreamImage.Image, imageStreamImage.Image.Name)
}

// ImageStreamDescriber generates information about a ImageStream (Image).
type ImageStreamDescriber struct {
	ImageClient imageclient.ImageInterface
}

// Describe returns the description of an imageStream
func (d *ImageStreamDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.ImageClient.ImageStreams(namespace)
	imageStream, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return DescribeImageStream(imageStream)
}

func DescribeImageStream(imageStream *imageapi.ImageStream) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, imageStream.ObjectMeta)
		if len(imageStream.Status.PublicDockerImageRepository) > 0 {
			formatString(out, "Docker Pull Spec", imageStream.Status.PublicDockerImageRepository)
		} else {
			formatString(out, "Docker Pull Spec", imageStream.Status.DockerImageRepository)
		}
		formatString(out, "Image Lookup", fmt.Sprintf("local=%t", imageStream.Spec.LookupPolicy.Local))
		formatImageStreamTags(out, imageStream)
		return nil
	})
}

// RouteDescriber generates information about a Route
type RouteDescriber struct {
	routeClient routeclient.RouteInterface
	kubeClient  kclientset.Interface
}

type routeEndpointInfo struct {
	*kapi.Endpoints
	Err error
}

// Describe returns the description of a route
func (d *RouteDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.routeClient.Routes(namespace)
	route, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	backends := append([]routeapi.RouteTargetReference{route.Spec.To}, route.Spec.AlternateBackends...)
	totalWeight := int32(0)
	endpoints := make(map[string]routeEndpointInfo)
	for _, backend := range backends {
		if backend.Weight != nil {
			totalWeight += *backend.Weight
		}
		ep, endpointsErr := d.kubeClient.Core().Endpoints(namespace).Get(backend.Name, metav1.GetOptions{})
		endpoints[backend.Name] = routeEndpointInfo{ep, endpointsErr}
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		var hostName string
		formatMeta(out, route.ObjectMeta)
		if len(route.Spec.Host) > 0 {
			formatString(out, "Requested Host", route.Spec.Host)
			for _, ingress := range route.Status.Ingress {
				if route.Spec.Host != ingress.Host {
					continue
				}
				hostName = ""
				if len(ingress.RouterCanonicalHostname) > 0 {
					hostName = fmt.Sprintf(" (host %s)", ingress.RouterCanonicalHostname)
				}
				switch status, condition := routeapi.IngressConditionStatus(&ingress, routeapi.RouteAdmitted); status {
				case kapi.ConditionTrue:
					fmt.Fprintf(out, "\t  exposed on router %s%s %s ago\n", ingress.RouterName, hostName, strings.ToLower(formatRelativeTime(condition.LastTransitionTime.Time)))
				case kapi.ConditionFalse:
					fmt.Fprintf(out, "\t  rejected by router %s: %s%s (%s ago)\n", ingress.RouterName, hostName, condition.Reason, strings.ToLower(formatRelativeTime(condition.LastTransitionTime.Time)))
					if len(condition.Message) > 0 {
						fmt.Fprintf(out, "\t    %s\n", condition.Message)
					}
				}
			}
		} else {
			formatString(out, "Requested Host", "<auto>")
		}

		for _, ingress := range route.Status.Ingress {
			if route.Spec.Host == ingress.Host {
				continue
			}
			hostName = ""
			if len(ingress.RouterCanonicalHostname) > 0 {
				hostName = fmt.Sprintf(" (host %s)", ingress.RouterCanonicalHostname)
			}
			switch status, condition := routeapi.IngressConditionStatus(&ingress, routeapi.RouteAdmitted); status {
			case kapi.ConditionTrue:
				fmt.Fprintf(out, "\t%s exposed on router %s %s%s ago\n", ingress.Host, ingress.RouterName, hostName, strings.ToLower(formatRelativeTime(condition.LastTransitionTime.Time)))
			case kapi.ConditionFalse:
				fmt.Fprintf(out, "\trejected by router %s: %s%s (%s ago)\n", ingress.RouterName, hostName, condition.Reason, strings.ToLower(formatRelativeTime(condition.LastTransitionTime.Time)))
				if len(condition.Message) > 0 {
					fmt.Fprintf(out, "\t  %s\n", condition.Message)
				}
			}
		}
		formatString(out, "Path", route.Spec.Path)

		tlsTerm := ""
		insecurePolicy := ""
		if route.Spec.TLS != nil {
			tlsTerm = string(route.Spec.TLS.Termination)
			insecurePolicy = string(route.Spec.TLS.InsecureEdgeTerminationPolicy)
		}
		formatString(out, "TLS Termination", tlsTerm)
		formatString(out, "Insecure Policy", insecurePolicy)
		if route.Spec.Port != nil {
			formatString(out, "Endpoint Port", route.Spec.Port.TargetPort.String())
		} else {
			formatString(out, "Endpoint Port", "<all endpoint ports>")
		}

		for _, backend := range backends {
			fmt.Fprintln(out)
			formatString(out, "Service", backend.Name)
			weight := int32(0)
			if backend.Weight != nil {
				weight = *backend.Weight
			}
			if weight > 0 {
				fmt.Fprintf(out, "Weight:\t%d (%d%%)\n", weight, weight*100/totalWeight)
			} else {
				formatString(out, "Weight", "0")
			}

			info := endpoints[backend.Name]
			if info.Err != nil {
				formatString(out, "Endpoints", fmt.Sprintf("<error: %v>", info.Err))
				continue
			}
			endpoints := info.Endpoints
			if len(endpoints.Subsets) == 0 {
				formatString(out, "Endpoints", "<none>")
				continue
			}

			list := []string{}
			max := 3
			count := 0
			for i := range endpoints.Subsets {
				ss := &endpoints.Subsets[i]
				for p := range ss.Ports {
					for a := range ss.Addresses {
						if len(list) < max {
							list = append(list, fmt.Sprintf("%s:%d", ss.Addresses[a].IP, ss.Ports[p].Port))
						}
						count++
					}
				}
			}
			ends := strings.Join(list, ", ")
			if count > max {
				ends += fmt.Sprintf(" + %d more...", count-max)
			}
			formatString(out, "Endpoints", ends)
		}
		return nil
	})
}

// ProjectDescriber generates information about a Project
type ProjectDescriber struct {
	projectClient projectclient.ProjectInterface
	kubeClient    kclientset.Interface
}

// Describe returns the description of a project
func (d *ProjectDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	projectsClient := d.projectClient.Projects()
	project, err := projectsClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	resourceQuotasClient := d.kubeClient.Core().ResourceQuotas(name)
	resourceQuotaList, err := resourceQuotasClient.List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	limitRangesClient := d.kubeClient.Core().LimitRanges(name)
	limitRangeList, err := limitRangesClient.List(metav1.ListOptions{})
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
		formatString(out, "Display Name", project.Annotations[oapi.OpenShiftDisplayName])
		formatString(out, "Description", project.Annotations[oapi.OpenShiftDescription])
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
				sort.Sort(kinternalprinters.SortableResourceNames(resources))

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
				fmt.Fprintf(out, "\tType\tResource\tMin\tMax\tDefault\tLimit\tLimit/Request\n")
				fmt.Fprintf(out, "\t----\t--------\t---\t---\t---\t-----\t-------------\n")
				for i := range limitRange.Spec.Limits {
					item := limitRange.Spec.Limits[i]
					maxResources := item.Max
					minResources := item.Min
					defaultResources := item.Default
					defaultRequestResources := item.DefaultRequest
					ratio := item.MaxLimitRequestRatio

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
					for k := range defaultRequestResources {
						set[k] = true
					}
					for k := range ratio {
						set[k] = true
					}

					for k := range set {
						// if no value is set, we output -
						maxValue := "-"
						minValue := "-"
						defaultValue := "-"
						defaultLimitValue := "-"
						ratioValue := "-"

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

						defaultLimitQuantity, defaultLimitQuantityFound := defaultResources[k]
						if defaultLimitQuantityFound {
							defaultLimitValue = defaultLimitQuantity.String()
						}

						ratioQuantity, ratioQuantityFound := ratio[k]
						if ratioQuantityFound {
							ratioValue = ratioQuantity.String()
						}

						msg := "\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n"
						fmt.Fprintf(out, msg, item.Type, k, minValue, maxValue, defaultValue, defaultLimitValue, ratioValue)
					}
				}
			}
		}
		return nil
	})
}

// TemplateDescriber generates information about a template
type TemplateDescriber struct {
	templateClient templateclient.TemplateInterface
	meta.MetadataAccessor
	runtime.ObjectTyper
	kprinters.ObjectDescriber
}

// DescribeMessage prints the message that will be parameter substituted and displayed to the
// user when this template is processed.
func (d *TemplateDescriber) DescribeMessage(msg string, out *tabwriter.Writer) {
	if len(msg) == 0 {
		msg = "<none>"
	}
	formatString(out, "Message", msg)
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
			out.Write([]byte("\n"))
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

		name, _ := d.MetadataAccessor.Name(obj)
		groupKind := "<unknown>"
		if gvk, _, err := d.ObjectTyper.ObjectKinds(obj); err == nil {
			gk := gvk[0].GroupKind()
			groupKind = gk.String()
		} else {
			if unstructured, ok := obj.(*unstructured.Unstructured); ok {
				gvk := unstructured.GroupVersionKind()
				gk := gvk.GroupKind()
				groupKind = gk.String()
			}
		}
		fmt.Fprintf(out, fmt.Sprintf("%s%s\t%s\n", indent, groupKind, name))
		//meta.Annotations, _ = d.MetadataAccessor.Annotations(obj)
		//meta.Labels, _ = d.MetadataAccessor.Labels(obj)
		/*if len(meta.Labels) > 0 {
			formatString(out, indent+"Labels", formatLabels(meta.Labels))
		}
		formatAnnotations(out, meta, indent)*/
	}
}

// Describe returns the description of a template
func (d *TemplateDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.templateClient.Templates(namespace)
	template, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return d.DescribeTemplate(template)
}

func (d *TemplateDescriber) DescribeTemplate(template *templateapi.Template) (string, error) {
	// TODO: write error?
	_ = runtime.DecodeList(template.Objects, unstructured.UnstructuredJSONScheme)

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, template.ObjectMeta)
		out.Write([]byte("\n"))
		out.Flush()
		d.DescribeParameters(template.Parameters, out)
		out.Write([]byte("\n"))
		formatString(out, "Object Labels", formatLabels(template.ObjectLabels))
		out.Write([]byte("\n"))
		d.DescribeMessage(template.Message, out)
		out.Write([]byte("\n"))
		out.Flush()
		d.describeObjects(template.Objects, out)
		return nil
	})
}

// TemplateInstanceDescriber generates information about a template instance
type TemplateInstanceDescriber struct {
	kubeClient     kclientset.Interface
	templateClient templateclient.TemplateInterface
	kprinters.ObjectDescriber
}

// Describe returns the description of a template instance
func (d *TemplateInstanceDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.templateClient.TemplateInstances(namespace)
	templateInstance, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return d.DescribeTemplateInstance(templateInstance, namespace, settings)
}

// DescribeTemplateInstance prints out information about the template instance
func (d *TemplateInstanceDescriber) DescribeTemplateInstance(templateInstance *templateapi.TemplateInstance, namespace string, settings kprinters.DescriberSettings) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, templateInstance.ObjectMeta)
		out.Write([]byte("\n"))
		out.Flush()
		d.DescribeConditions(templateInstance.Status.Conditions, out)
		out.Write([]byte("\n"))
		out.Flush()
		d.DescribeObjects(templateInstance.Status.Objects, out)
		out.Write([]byte("\n"))
		out.Flush()
		d.DescribeParameters(templateInstance.Spec.Template, namespace, templateInstance.Spec.Secret.Name, out)
		out.Write([]byte("\n"))
		out.Flush()
		return nil
	})
}

// DescribeConditions prints out information about the conditions of a template instance
func (d *TemplateInstanceDescriber) DescribeConditions(conditions []templateapi.TemplateInstanceCondition, out *tabwriter.Writer) {
	formatString(out, "Conditions", " ")
	indent := "    "
	for _, c := range conditions {
		formatString(out, indent+"Type", c.Type)
		formatString(out, indent+"Status", c.Status)
		formatString(out, indent+"LastTransitionTime", c.LastTransitionTime)
		formatString(out, indent+"Reason", c.Reason)
		formatString(out, indent+"Message", c.Message)
		out.Write([]byte("\n"))
	}
}

// DescribeObjects prints out information about the objects that a template instance creates
func (d *TemplateInstanceDescriber) DescribeObjects(objects []templateapi.TemplateInstanceObject, out *tabwriter.Writer) {
	formatString(out, "Objects", " ")
	indent := "    "
	for _, o := range objects {
		formatString(out, indent+o.Ref.Kind, fmt.Sprintf("%s/%s", o.Ref.Namespace, o.Ref.Name))
	}
}

// DescribeParameters prints out information about the secret that holds the template instance parameters
// kinternalprinter.SecretDescriber#Describe could have been used here, but the formatting
// is off when it prints the information and seems to not be easily fixable
func (d *TemplateInstanceDescriber) DescribeParameters(template templateapi.Template, namespace, name string, out *tabwriter.Writer) {
	secret, err := d.kubeClient.Core().Secrets(namespace).Get(name, metav1.GetOptions{})

	formatString(out, "Parameters", " ")

	if kerrs.IsForbidden(err) || kerrs.IsUnauthorized(err) {
		fmt.Fprintf(out, "Unable to access parameters, insufficient permissions.")
		return
	} else if kerrs.IsNotFound(err) {
		fmt.Fprintf(out, "Unable to access parameters, secret not found: %s", secret.Name)
		return
	} else if err != nil {
		fmt.Fprintf(out, "Unknown error occurred, please rerun with loglevel > 4 for more information")
		glog.V(4).Infof("%v", err)
		return
	}

	indent := "    "
	if len(template.Parameters) == 0 {
		fmt.Fprintf(out, indent+"No parameters found.")
	} else {
		for _, p := range template.Parameters {
			if val, ok := secret.Data[p.Name]; ok {
				formatString(out, indent+p.Name, fmt.Sprintf("%d bytes", len(val)))
			}
		}
	}
}

// IdentityDescriber generates information about a user
type IdentityDescriber struct {
	c userclient.UserInterface
}

// Describe returns the description of an identity
func (d *IdentityDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	userClient := d.c.Users()
	identityClient := d.c.Identities()

	identity, err := identityClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, identity.ObjectMeta)

		if len(identity.User.Name) == 0 {
			formatString(out, "User Name", identity.User.Name)
			formatString(out, "User UID", identity.User.UID)
		} else {
			resolvedUser, err := userClient.Get(identity.User.Name, metav1.GetOptions{})

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
	c userclient.UserInterface
}

// Describe returns the description of a userIdentity
func (d *UserIdentityMappingDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.UserIdentityMappings()

	mapping, err := c.Get(name, metav1.GetOptions{})
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
	c userclient.UserInterface
}

// Describe returns the description of a user
func (d *UserDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	userClient := d.c.Users()
	identityClient := d.c.Identities()

	user, err := userClient.Get(name, metav1.GetOptions{})
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
				resolvedIdentity, err := identityClient.Get(identity, metav1.GetOptions{})

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
	c userclient.UserInterface
}

// Describe returns the description of a group
func (d *GroupDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	group, err := d.c.Groups().Get(name, metav1.GetOptions{})
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

const PolicyRuleHeadings = "Verbs\tNon-Resource URLs\tResource Names\tAPI Groups\tResources"

func DescribePolicyRule(out *tabwriter.Writer, rule authorizationapi.PolicyRule, indent string) {
	if rule.AttributeRestrictions != nil {
		// We are not supporting attribute restrictions going forward
		return
	}

	fmt.Fprintf(out, indent+"%v\t%v\t%v\t%v\t%v\n",
		rule.Verbs.List(),
		rule.NonResourceURLs.List(),
		rule.ResourceNames.List(),
		rule.APIGroups,
		rule.Resources.List(),
	)
}

// RoleDescriber generates information about a Project
type RoleDescriber struct {
	c oauthorizationclient.AuthorizationInterface
}

// Describe returns the description of a role
func (d *RoleDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.Roles(namespace)
	role, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return DescribeRole(role)
}

func DescribeRole(role *authorizationapi.Role) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, role.ObjectMeta)

		fmt.Fprint(out, PolicyRuleHeadings+"\n")
		for _, rule := range role.Rules {
			DescribePolicyRule(out, rule, "")

		}

		return nil
	})
}

// RoleBindingDescriber generates information about a Project
type RoleBindingDescriber struct {
	c oauthorizationclient.AuthorizationInterface
}

// Describe returns the description of a roleBinding
func (d *RoleBindingDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.RoleBindings(namespace)
	roleBinding, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var role *authorizationapi.Role
	if len(roleBinding.RoleRef.Namespace) == 0 {
		var clusterRole *authorizationapi.ClusterRole
		clusterRole, err = d.c.ClusterRoles().Get(roleBinding.RoleRef.Name, metav1.GetOptions{})
		role = authorizationapi.ToRole(clusterRole)
	} else {
		role, err = d.c.Roles(roleBinding.RoleRef.Namespace).Get(roleBinding.RoleRef.Name, metav1.GetOptions{})
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
			fmt.Fprint(out, PolicyRuleHeadings+"\n")
			for _, rule := range role.Rules {
				DescribePolicyRule(out, rule, "")
			}

		default:
			formatString(out, "Policy Rules", "<none>")
		}

		return nil
	})
}

type ClusterRoleDescriber struct {
	c oauthorizationclient.AuthorizationInterface
}

// Describe returns the description of a role
func (d *ClusterRoleDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.ClusterRoles()
	role, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return DescribeRole(authorizationapi.ToRole(role))
}

// ClusterRoleBindingDescriber generates information about a Project
type ClusterRoleBindingDescriber struct {
	c oauthorizationclient.AuthorizationInterface
}

// Describe returns the description of a roleBinding
func (d *ClusterRoleBindingDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.ClusterRoleBindings()
	roleBinding, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	role, err := d.c.ClusterRoles().Get(roleBinding.RoleRef.Name, metav1.GetOptions{})
	return DescribeRoleBinding(authorizationapi.ToRoleBinding(roleBinding), authorizationapi.ToRole(role), err)
}

func describeBuildTriggerCauses(causes []buildapi.BuildTriggerCause, out *tabwriter.Writer) {
	if causes == nil {
		formatString(out, "\nBuild trigger cause", "<unknown>")
	}

	for _, cause := range causes {
		formatString(out, "\nBuild trigger cause", cause.Message)

		switch {
		case cause.GitHubWebHook != nil:
			squashGitInfo(cause.GitHubWebHook.Revision, out)
			formatString(out, "Secret", cause.GitHubWebHook.Secret)

		case cause.GitLabWebHook != nil:
			squashGitInfo(cause.GitLabWebHook.Revision, out)
			formatString(out, "Secret", cause.GitLabWebHook.Secret)

		case cause.BitbucketWebHook != nil:
			squashGitInfo(cause.BitbucketWebHook.Revision, out)
			formatString(out, "Secret", cause.BitbucketWebHook.Secret)

		case cause.GenericWebHook != nil:
			squashGitInfo(cause.GenericWebHook.Revision, out)
			formatString(out, "Secret", cause.GenericWebHook.Secret)

		case cause.ImageChangeBuild != nil:
			formatString(out, "Image ID", cause.ImageChangeBuild.ImageID)
			formatString(out, "Image Name/Kind", fmt.Sprintf("%s / %s", cause.ImageChangeBuild.FromRef.Name, cause.ImageChangeBuild.FromRef.Kind))
		}
	}
	fmt.Fprintf(out, "\n")
}

func squashGitInfo(sourceRevision *buildapi.SourceRevision, out *tabwriter.Writer) {
	if sourceRevision != nil && sourceRevision.Git != nil {
		rev := sourceRevision.Git
		var commit string
		if len(rev.Commit) > 7 {
			commit = rev.Commit[:7]
		} else {
			commit = rev.Commit
		}
		formatString(out, "Commit", fmt.Sprintf("%s (%s)", commit, rev.Message))
		hasAuthor := len(rev.Author.Name) != 0
		hasCommitter := len(rev.Committer.Name) != 0
		if hasAuthor && hasCommitter {
			if rev.Author.Name == rev.Committer.Name {
				formatString(out, "Author/Committer", rev.Author.Name)
			} else {
				formatString(out, "Author/Committer", fmt.Sprintf("%s / %s", rev.Author.Name, rev.Committer.Name))
			}
		} else if hasAuthor {
			formatString(out, "Author", rev.Author.Name)
		} else if hasCommitter {
			formatString(out, "Committer", rev.Committer.Name)
		}
	}
}

type ClusterQuotaDescriber struct {
	c quotaclient.QuotaInterface
}

func (d *ClusterQuotaDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	quota, err := d.c.ClusterResourceQuotas().Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return DescribeClusterQuota(quota)
}

func DescribeClusterQuota(quota *quotaapi.ClusterResourceQuota) (string, error) {
	labelSelector, err := metav1.LabelSelectorAsSelector(quota.Spec.Selector.LabelSelector)
	if err != nil {
		return "", err
	}

	nsSelector := make([]interface{}, 0, quota.Status.Namespaces.OrderedKeys().Len())
	ns := quota.Status.Namespaces.OrderedKeys().Front()
	for ns != nil {
		nsSelector = append(nsSelector, ns.Value)
		ns = ns.Next()
	}

	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, quota.ObjectMeta)
		fmt.Fprintf(out, "Namespace Selector: %q\n", nsSelector)
		fmt.Fprintf(out, "Label Selector: %s\n", labelSelector)
		fmt.Fprintf(out, "AnnotationSelector: %s\n", quota.Spec.Selector.AnnotationSelector)
		if len(quota.Spec.Quota.Scopes) > 0 {
			scopes := []string{}
			for _, scope := range quota.Spec.Quota.Scopes {
				scopes = append(scopes, string(scope))
			}
			sort.Strings(scopes)
			fmt.Fprintf(out, "Scopes:\t%s\n", strings.Join(scopes, ", "))
		}
		fmt.Fprintf(out, "Resource\tUsed\tHard\n")
		fmt.Fprintf(out, "--------\t----\t----\n")

		resources := []kapi.ResourceName{}
		for resource := range quota.Status.Total.Hard {
			resources = append(resources, resource)
		}
		sort.Sort(kinternalprinters.SortableResourceNames(resources))

		msg := "%v\t%v\t%v\n"
		for i := range resources {
			resource := resources[i]
			hardQuantity := quota.Status.Total.Hard[resource]
			usedQuantity := quota.Status.Total.Used[resource]
			fmt.Fprintf(out, msg, resource, usedQuantity.String(), hardQuantity.String())
		}
		return nil
	})
}

type AppliedClusterQuotaDescriber struct {
	c quotaclient.QuotaInterface
}

func (d *AppliedClusterQuotaDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	quota, err := d.c.AppliedClusterResourceQuotas(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return DescribeClusterQuota(quotaapi.ConvertAppliedClusterResourceQuotaToClusterResourceQuota(quota))
}

type ClusterNetworkDescriber struct {
	c onetworkclient.NetworkInterface
}

// Describe returns the description of a ClusterNetwork
func (d *ClusterNetworkDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	cn, err := d.c.ClusterNetworks().Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, cn.ObjectMeta)
		formatString(out, "Service Network", cn.ServiceNetwork)
		formatString(out, "Plugin Name", cn.PluginName)
		fmt.Fprintf(out, "ClusterNetworks:\n")
		fmt.Fprintf(out, "CIDR\tHost Subnet Length\n")
		fmt.Fprintf(out, "----\t------------------\n")
		for _, clusterNetwork := range cn.ClusterNetworks {
			fmt.Fprintf(out, "%s\t%d\n", clusterNetwork.CIDR, clusterNetwork.HostSubnetLength)
		}
		return nil
	})
}

type HostSubnetDescriber struct {
	c onetworkclient.NetworkInterface
}

// Describe returns the description of a HostSubnet
func (d *HostSubnetDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	hs, err := d.c.HostSubnets().Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, hs.ObjectMeta)
		formatString(out, "Node", hs.Host)
		formatString(out, "Node IP", hs.HostIP)
		formatString(out, "Pod Subnet", hs.Subnet)
		formatString(out, "Egress IPs", strings.Join(hs.EgressIPs, ", "))
		return nil
	})
}

type NetNamespaceDescriber struct {
	c onetworkclient.NetworkInterface
}

// Describe returns the description of a NetNamespace
func (d *NetNamespaceDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	netns, err := d.c.NetNamespaces().Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, netns.ObjectMeta)
		formatString(out, "Name", netns.NetName)
		formatString(out, "ID", netns.NetID)
		formatString(out, "Egress IPs", strings.Join(netns.EgressIPs, ", "))
		return nil
	})
}

type EgressNetworkPolicyDescriber struct {
	c onetworkclient.NetworkInterface
}

// Describe returns the description of an EgressNetworkPolicy
func (d *EgressNetworkPolicyDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	c := d.c.EgressNetworkPolicies(namespace)
	policy, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, policy.ObjectMeta)
		for _, rule := range policy.Spec.Egress {
			if len(rule.To.CIDRSelector) > 0 {
				fmt.Fprintf(out, "Rule:\t%s to %s\n", rule.Type, rule.To.CIDRSelector)
			} else {
				fmt.Fprintf(out, "Rule:\t%s to %s\n", rule.Type, rule.To.DNSName)
			}
		}
		return nil
	})
}

type RoleBindingRestrictionDescriber struct {
	c oauthorizationclient.AuthorizationInterface
}

// Describe returns the description of a RoleBindingRestriction.
func (d *RoleBindingRestrictionDescriber) Describe(namespace, name string, settings kprinters.DescriberSettings) (string, error) {
	rbr, err := d.c.RoleBindingRestrictions(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return tabbedString(func(out *tabwriter.Writer) error {
		formatMeta(out, rbr.ObjectMeta)

		subjectType := roleBindingRestrictionType(rbr)
		if subjectType == "" {
			subjectType = "<none>"
		}
		formatString(out, "Subject type", subjectType)

		var labelSelectors []metav1.LabelSelector

		switch {
		case rbr.Spec.UserRestriction != nil:
			formatString(out, "Users",
				strings.Join(rbr.Spec.UserRestriction.Users, ", "))
			formatString(out, "Users in groups",
				strings.Join(rbr.Spec.UserRestriction.Groups, ", "))
			labelSelectors = rbr.Spec.UserRestriction.Selectors
		case rbr.Spec.GroupRestriction != nil:
			formatString(out, "Groups",
				strings.Join(rbr.Spec.GroupRestriction.Groups, ", "))
			labelSelectors = rbr.Spec.GroupRestriction.Selectors
		case rbr.Spec.ServiceAccountRestriction != nil:
			serviceaccounts := []string{}
			for _, sa := range rbr.Spec.ServiceAccountRestriction.ServiceAccounts {
				serviceaccounts = append(serviceaccounts, sa.Name)
			}
			formatString(out, "ServiceAccounts", strings.Join(serviceaccounts, ", "))
			formatString(out, "Namespaces",
				strings.Join(rbr.Spec.ServiceAccountRestriction.Namespaces, ", "))
		}

		if rbr.Spec.UserRestriction != nil || rbr.Spec.GroupRestriction != nil {
			if len(labelSelectors) == 0 {
				formatString(out, "Label selectors", "")
			} else {
				fmt.Fprintf(out, "Label selectors:\n")
				for _, labelSelector := range labelSelectors {
					selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
					if err != nil {
						return err
					}
					fmt.Fprintf(out, "\t%s\n", selector)
				}
			}
		}

		return nil
	})
}

// SecurityContextConstraintsDescriber generates information about an SCC
type SecurityContextConstraintsDescriber struct {
	c securityclient.SecurityContextConstraintsGetter
}

func (d *SecurityContextConstraintsDescriber) Describe(namespace, name string, s kprinters.DescriberSettings) (string, error) {
	scc, err := d.c.SecurityContextConstraints().Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return describeSecurityContextConstraints(scc)
}

func describeSecurityContextConstraints(scc *securityapi.SecurityContextConstraints) (string, error) {
	return tabbedString(func(out *tabwriter.Writer) error {
		fmt.Fprintf(out, "Name:\t%s\n", scc.Name)

		priority := ""
		if scc.Priority != nil {
			priority = fmt.Sprintf("%d", *scc.Priority)
		}
		fmt.Fprintf(out, "Priority:\t%s\n", stringOrNone(priority))

		fmt.Fprintf(out, "Access:\t\n")
		fmt.Fprintf(out, "  Users:\t%s\n", stringOrNone(strings.Join(scc.Users, ",")))
		fmt.Fprintf(out, "  Groups:\t%s\n", stringOrNone(strings.Join(scc.Groups, ",")))

		fmt.Fprintf(out, "Settings:\t\n")
		fmt.Fprintf(out, "  Allow Privileged:\t%t\n", scc.AllowPrivilegedContainer)
		fmt.Fprintf(out, "  Default Add Capabilities:\t%s\n", capsToString(scc.DefaultAddCapabilities))
		fmt.Fprintf(out, "  Required Drop Capabilities:\t%s\n", capsToString(scc.RequiredDropCapabilities))
		fmt.Fprintf(out, "  Allowed Capabilities:\t%s\n", capsToString(scc.AllowedCapabilities))
		fmt.Fprintf(out, "  Allowed Seccomp Profiles:\t%s\n", stringOrNone(strings.Join(scc.SeccompProfiles, ",")))
		fmt.Fprintf(out, "  Allowed Volume Types:\t%s\n", fsTypeToString(scc.Volumes))
		fmt.Fprintf(out, "  Allowed Flexvolumes:\t%s\n", flexVolumesToString(scc.AllowedFlexVolumes))
		fmt.Fprintf(out, "  Allow Host Network:\t%t\n", scc.AllowHostNetwork)
		fmt.Fprintf(out, "  Allow Host Ports:\t%t\n", scc.AllowHostPorts)
		fmt.Fprintf(out, "  Allow Host PID:\t%t\n", scc.AllowHostPID)
		fmt.Fprintf(out, "  Allow Host IPC:\t%t\n", scc.AllowHostIPC)
		fmt.Fprintf(out, "  Read Only Root Filesystem:\t%t\n", scc.ReadOnlyRootFilesystem)

		fmt.Fprintf(out, "  Run As User Strategy: %s\t\n", string(scc.RunAsUser.Type))
		uid := ""
		if scc.RunAsUser.UID != nil {
			uid = strconv.FormatInt(*scc.RunAsUser.UID, 10)
		}
		fmt.Fprintf(out, "    UID:\t%s\n", stringOrNone(uid))

		uidRangeMin := ""
		if scc.RunAsUser.UIDRangeMin != nil {
			uidRangeMin = strconv.FormatInt(*scc.RunAsUser.UIDRangeMin, 10)
		}
		fmt.Fprintf(out, "    UID Range Min:\t%s\n", stringOrNone(uidRangeMin))

		uidRangeMax := ""
		if scc.RunAsUser.UIDRangeMax != nil {
			uidRangeMax = strconv.FormatInt(*scc.RunAsUser.UIDRangeMax, 10)
		}
		fmt.Fprintf(out, "    UID Range Max:\t%s\n", stringOrNone(uidRangeMax))

		fmt.Fprintf(out, "  SELinux Context Strategy: %s\t\n", string(scc.SELinuxContext.Type))
		var user, role, seLinuxType, level string
		if scc.SELinuxContext.SELinuxOptions != nil {
			user = scc.SELinuxContext.SELinuxOptions.User
			role = scc.SELinuxContext.SELinuxOptions.Role
			seLinuxType = scc.SELinuxContext.SELinuxOptions.Type
			level = scc.SELinuxContext.SELinuxOptions.Level
		}
		fmt.Fprintf(out, "    User:\t%s\n", stringOrNone(user))
		fmt.Fprintf(out, "    Role:\t%s\n", stringOrNone(role))
		fmt.Fprintf(out, "    Type:\t%s\n", stringOrNone(seLinuxType))
		fmt.Fprintf(out, "    Level:\t%s\n", stringOrNone(level))

		fmt.Fprintf(out, "  FSGroup Strategy: %s\t\n", string(scc.FSGroup.Type))
		fmt.Fprintf(out, "    Ranges:\t%s\n", idRangeToString(scc.FSGroup.Ranges))

		fmt.Fprintf(out, "  Supplemental Groups Strategy: %s\t\n", string(scc.SupplementalGroups.Type))
		fmt.Fprintf(out, "    Ranges:\t%s\n", idRangeToString(scc.SupplementalGroups.Ranges))

		return nil
	})
}

func stringOrNone(s string) string {
	return stringOrDefaultValue(s, "<none>")
}

func stringOrDefaultValue(s, defaultValue string) string {
	if len(s) > 0 {
		return s
	}
	return defaultValue
}

func fsTypeToString(volumes []securityapi.FSType) string {
	strVolumes := []string{}
	for _, v := range volumes {
		strVolumes = append(strVolumes, string(v))
	}
	return stringOrNone(strings.Join(strVolumes, ","))
}

func flexVolumesToString(flexVolumes []securityapi.AllowedFlexVolume) string {
	volumes := []string{}
	for _, flexVolume := range flexVolumes {
		volumes = append(volumes, "driver="+flexVolume.Driver)
	}
	return stringOrDefaultValue(strings.Join(volumes, ","), "<all>")
}

func idRangeToString(ranges []securityapi.IDRange) string {
	formattedString := ""
	if ranges != nil {
		strRanges := []string{}
		for _, r := range ranges {
			strRanges = append(strRanges, fmt.Sprintf("%d-%d", r.Min, r.Max))
		}
		formattedString = strings.Join(strRanges, ",")
	}
	return stringOrNone(formattedString)
}

func capsToString(caps []kapi.Capability) string {
	formattedString := ""
	if caps != nil {
		strCaps := []string{}
		for _, c := range caps {
			strCaps = append(strCaps, string(c))
		}
		formattedString = strings.Join(strCaps, ",")
	}
	return stringOrNone(formattedString)
}
