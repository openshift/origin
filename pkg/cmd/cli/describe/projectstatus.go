package describe

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/graph"
	graphveneers "github.com/openshift/origin/pkg/api/graph/veneers"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployedges "github.com/openshift/origin/pkg/deploy/graph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

// ProjectStatusDescriber generates extended information about a Project
type ProjectStatusDescriber struct {
	K kclient.Interface
	C client.Interface
}

// Describe returns the description of a project
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

	builds := &buildapi.BuildList{}
	if len(bcs.Items) > 0 {
		if b, err := d.C.Builds(namespace).List(labels.Everything(), fields.Everything()); err == nil {
			builds = b
		}
	}

	rcs, err := d.K.ReplicationControllers(namespace).List(labels.Everything())
	if err != nil {
		rcs = &kapi.ReplicationControllerList{}
	}

	g := graph.New()
	for i := range bcs.Items {
		build := buildgraph.EnsureBuildConfigNode(g, &bcs.Items[i])
		buildedges.AddInputOutputEdges(g, build)
		buildedges.JoinBuilds(build, builds.Items)
	}
	for i := range dcs.Items {
		deploy := deploygraph.EnsureDeploymentConfigNode(g, &dcs.Items[i])
		deployedges.AddTriggerEdges(g, deploy)
		deployedges.JoinDeployments(deploy, rcs.Items)
	}
	for i := range svcs.Items {
		service := kubegraph.EnsureServiceNode(g, &svcs.Items[i])
		kubeedges.AddExposedPodTemplateSpecEdges(g, service)
	}
	groups := graphveneers.ServiceAndDeploymentGroups(g)

	return tabbedString(func(out *tabwriter.Writer) error {
		indent := "  "
		fmt.Fprintf(out, "In project %s\n", projectapi.DisplayNameAndNameForProject(project))

		for _, group := range groups {
			if len(group.Builds) != 0 {
				for _, build := range group.Builds {
					fmt.Fprintln(out)
					printLines(out, indent, 0, describeStandaloneBuildGroup(build, namespace)...)
					printLines(out, indent, 1, describeAdditionalBuildDetail(build.Build, true)...)
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
			fmt.Fprintln(out, "\nYou have no Services, DeploymentConfigs, or BuildConfigs. 'oc new-app' can be used to create applications from scratch from existing Docker images and templates.")
		} else {
			fmt.Fprintln(out, "\nTo see more information about a Service or DeploymentConfig, use 'oc describe service <name>' or 'oc describe dc <name>'.")
			fmt.Fprintln(out, "You can use 'oc get all' to see lists of each of the types described above.")
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

func describeDeploymentInServiceGroup(deploy graphveneers.DeploymentFlow) []string {
	includeLastPass := deploy.Deployment.ActiveDeployment == nil
	if len(deploy.Images) == 1 {
		lines := []string{fmt.Sprintf("%s deploys %s %s", deploy.Deployment.Name, describeImageInPipeline(deploy.Images[0], deploy.Deployment.Namespace), describeDeploymentConfigTrigger(deploy.Deployment.DeploymentConfig))}
		if len(lines[0]) > 120 && strings.Contains(lines[0], " <- ") {
			segments := strings.SplitN(lines[0], " <- ", 2)
			lines[0] = segments[0] + " <-"
			lines = append(lines, segments[1])
		}
		lines = append(lines, describeAdditionalBuildDetail(deploy.Images[0].Build, includeLastPass)...)
		lines = append(lines, describeDeployments(deploy.Deployment, 3)...)
		return lines
	}

	lines := []string{fmt.Sprintf("%s deploys: %s", deploy.Deployment.Name, describeDeploymentConfigTrigger(deploy.Deployment.DeploymentConfig))}
	for _, image := range deploy.Images {
		lines = append(lines, describeImageInPipeline(image, deploy.Deployment.Namespace))
		lines = append(lines, describeAdditionalBuildDetail(image.Build, includeLastPass)...)
		lines = append(lines, describeDeployments(deploy.Deployment, 3)...)
	}
	return lines
}

func describeDeploymentConfigTrigger(dc *deployapi.DeploymentConfig) string {
	if len(dc.Triggers) == 0 {
		return "(manual)"
	}

	return ""
}

func describeStandaloneBuildGroup(pipeline graphveneers.ImagePipeline, namespace string) []string {
	switch {
	case pipeline.Build != nil:
		lines := []string{fmt.Sprintf("%s %s", pipeline.Build.BuildConfig.Name, describeBuildInPipeline(pipeline.Build.BuildConfig, pipeline.BaseImage))}
		if pipeline.Image != nil {
			lines = append(lines, fmt.Sprintf("pushes to %s", describeImageTagInPipeline(pipeline.Image, namespace)))
		}
		return lines
	case pipeline.Image != nil:
		return []string{describeImageTagInPipeline(pipeline.Image, namespace)}
	default:
		return []string{"<unknown>"}
	}
}

func describeImageInPipeline(pipeline graphveneers.ImagePipeline, namespace string) string {
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

func describeImageTagInPipeline(image graphveneers.ImageTagLocation, namespace string) string {
	switch t := image.(type) {
	case *imagegraph.ImageStreamTagNode:
		if t.ImageStream.Namespace != namespace {
			return image.ImageSpec()
		}
		return fmt.Sprintf("%s:%s", t.ImageStream.Name, image.ImageTag())
	default:
		return image.ImageSpec()
	}
}

func describeBuildInPipeline(build *buildapi.BuildConfig, baseImage graphveneers.ImageTagLocation) string {
	switch build.Parameters.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		// TODO: handle case where no source repo
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return "docker build; no source set"
		}
		return fmt.Sprintf("docker build of %s", source)
	case buildapi.SourceBuildStrategyType:
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return fmt.Sprintf("unconfigured source build %s", build.Name)
		}
		if baseImage == nil {
			return fmt.Sprintf("%s; no image set", source)
		}
		return fmt.Sprintf("builds %s with %s", source, baseImage.ImageSpec())
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

func describeAdditionalBuildDetail(build *buildgraph.BuildConfigNode, includeSuccess bool) []string {
	if build == nil {
		return nil
	}
	out := []string{}

	pass := build.LastSuccessfulBuild
	passTime := buildTimestamp(pass)
	fail := build.LastUnsuccessfulBuild
	failTime := buildTimestamp(fail)

	last := failTime
	if passTime.After(failTime.Time) {
		last = passTime
		fail = nil
	}

	if pass != nil && includeSuccess {
		out = append(out, describeBuildStatus(pass, &passTime, build.BuildConfig.Name))
	}
	if fail != nil {
		out = append(out, describeBuildStatus(fail, &failTime, build.BuildConfig.Name))
	}

	active := build.ActiveBuilds
	if len(active) > 0 {
		activeOut := []string{}
		for i := range active {
			activeOut = append(activeOut, describeBuildStatus(&active[i], nil, build.BuildConfig.Name))
		}

		if buildTimestamp(&active[0]).Before(last) {
			out = append(out, activeOut...)
		} else {
			out = append(activeOut, out...)
		}
	}
	if len(out) == 0 && pass == nil {
		out = append(out, "not built yet")
	}
	return out
}

func describeBuildStatus(build *buildapi.Build, t *util.Time, parentName string) string {
	if t == nil {
		ts := buildTimestamp(build)
		t = &ts
	}
	var time string
	if t.IsZero() {
		time = "<unknown>"
	} else {
		time = strings.ToLower(formatRelativeTime(t.Time))
	}
	name := build.Name
	prefix := parentName + "-"
	if strings.HasPrefix(name, prefix) {
		name = name[len(prefix):]
	}
	revision := describeSourceRevision(build.Parameters.Revision)
	if len(revision) != 0 {
		revision = fmt.Sprintf(" - %s", revision)
	}
	switch build.Status {
	case buildapi.BuildStatusComplete:
		return fmt.Sprintf("build %s succeeded %s ago%s", name, time, revision)
	case buildapi.BuildStatusError:
		return fmt.Sprintf("build %s stopped with an error %s ago%s", name, time, revision)
	case buildapi.BuildStatusFailed:
		return fmt.Sprintf("build %s failed %s ago%s", name, time, revision)
	default:
		status := strings.ToLower(string(build.Status))
		return fmt.Sprintf("build %s %s for %s%s", name, status, time, revision)
	}
}

func describeSourceRevision(rev *buildapi.SourceRevision) string {
	if rev == nil {
		return ""
	}
	switch {
	case rev.Git != nil:
		author := describeSourceControlUser(rev.Git.Author)
		if len(author) == 0 {
			author = describeSourceControlUser(rev.Git.Committer)
		}
		if len(author) != 0 {
			author = fmt.Sprintf(" (%s)", author)
		}
		commit := rev.Git.Commit
		if len(commit) > 7 {
			commit = commit[:7]
		}
		return fmt.Sprintf("%s: %s%s", commit, rev.Git.Message, author)
	default:
		return ""
	}
}

func describeSourceControlUser(user buildapi.SourceControlUser) string {
	if len(user.Name) == 0 {
		return user.Email
	}
	if len(user.Email) == 0 {
		return user.Name
	}
	return fmt.Sprintf("%s <%s>", user.Name, user.Email)
}

func buildTimestamp(build *buildapi.Build) util.Time {
	if build == nil {
		return util.Time{}
	}
	if !build.CompletionTimestamp.IsZero() {
		return *build.CompletionTimestamp
	}
	if !build.StartTimestamp.IsZero() {
		return *build.StartTimestamp
	}
	return build.CreationTimestamp
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

func describeDeployments(node *deploygraph.DeploymentConfigNode, count int) []string {
	if node == nil {
		return nil
	}
	out := []string{}
	deployments := node.Deployments

	if node.ActiveDeployment == nil {
		on, auto := describeDeploymentConfigTriggers(node.DeploymentConfig)
		if node.DeploymentConfig.LatestVersion == 0 {
			out = append(out, fmt.Sprintf("#1 deployment waiting %s", on))
		} else if auto {
			out = append(out, fmt.Sprintf("#%d deployment pending %s", node.DeploymentConfig.LatestVersion, on))
		}
		// TODO: detect new image available?
	} else {
		deployments = append([]*kapi.ReplicationController{node.ActiveDeployment}, deployments...)
	}

	for i, deployment := range deployments {
		out = append(out, describeDeploymentStatus(deployment, i == 0))

		switch {
		case count == -1:
			if deployutil.DeploymentStatusFor(deployment) == deployapi.DeploymentStatusComplete {
				return out
			}
		default:
			if i+1 >= count {
				return out
			}
		}
	}
	return out
}

func describeDeploymentStatus(deploy *kapi.ReplicationController, first bool) string {
	timeAt := strings.ToLower(formatRelativeTime(deploy.CreationTimestamp.Time))
	status := deployutil.DeploymentStatusFor(deploy)
	version := deployutil.DeploymentVersionFor(deploy)
	switch status {
	case deployapi.DeploymentStatusFailed:
		reason := deployutil.DeploymentStatusReasonFor(deploy)
		if len(reason) > 0 {
			reason = fmt.Sprintf(": %s", reason)
		}
		// TODO: encode fail time in the rc
		return fmt.Sprintf("#%d deployment failed %s ago%s%s", version, timeAt, reason, describeDeploymentPodSummaryInline(deploy, false))
	case deployapi.DeploymentStatusComplete:
		// TODO: pod status output
		return fmt.Sprintf("#%d deployed %s ago%s", version, timeAt, describeDeploymentPodSummaryInline(deploy, first))
	case deployapi.DeploymentStatusRunning:
		return fmt.Sprintf("#%d deployment running for %s%s", version, timeAt, describeDeploymentPodSummaryInline(deploy, false))
	default:
		return fmt.Sprintf("#%d deployment %s %s ago%s", version, strings.ToLower(string(status)), timeAt, describeDeploymentPodSummaryInline(deploy, false))
	}
}

func describeDeploymentPodSummaryInline(deploy *kapi.ReplicationController, includeEmpty bool) string {
	s := describeDeploymentPodSummary(deploy, includeEmpty)
	if len(s) == 0 {
		return s
	}
	change := ""
	if changing, ok := deployutil.DeploymentDesiredReplicas(deploy); ok {
		switch {
		case changing < deploy.Spec.Replicas:
			change = fmt.Sprintf(" reducing to %d", changing)
		case changing > deploy.Spec.Replicas:
			change = fmt.Sprintf(" growing to %d", changing)
		}
	}
	return fmt.Sprintf(" - %s%s", s, change)
}

func describeDeploymentPodSummary(deploy *kapi.ReplicationController, includeEmpty bool) string {
	actual, requested := deploy.Status.Replicas, deploy.Spec.Replicas
	if actual == requested {
		switch {
		case actual == 0:
			if !includeEmpty {
				return ""
			}
			return "0 pods"
		case actual > 1:
			return fmt.Sprintf("%d pods", actual)
		default:
			return "1 pod"
		}
	}
	return fmt.Sprintf("%d/%d pods", actual, requested)
}

func describeDeploymentConfigTriggers(config *deployapi.DeploymentConfig) (string, bool) {
	hasConfig, hasImage := false, false
	for _, t := range config.Triggers {
		switch t.Type {
		case deployapi.DeploymentTriggerOnConfigChange:
			hasConfig = true
		case deployapi.DeploymentTriggerOnImageChange:
			hasImage = true
		}
	}
	switch {
	case hasConfig && hasImage:
		return "on image or update", true
	case hasConfig:
		return "on update", true
	case hasImage:
		return "on image", true
	default:
		return "for manual", false
	}
}

func describeServiceInServiceGroup(svc graphveneers.ServiceReference) []string {
	spec := svc.Service.Spec
	ip := spec.PortalIP
	port := describeServicePorts(spec)
	switch {
	case ip == "None":
		return []string{fmt.Sprintf("service %s (headless%s)", svc.Service.Name, port)}
	case len(ip) == 0:
		return []string{fmt.Sprintf("service %s (<initializing>%s)", svc.Service.Name, port)}
	default:
		return []string{fmt.Sprintf("service %s (%s%s)", svc.Service.Name, ip, port)}
	}
}

func describeServicePorts(spec kapi.ServiceSpec) string {
	switch len(spec.Ports) {
	case 0:
		return " no ports"
	case 1:
		if spec.Ports[0].TargetPort.String() == "0" || spec.PortalIP == kapi.PortalIPNone || spec.Ports[0].Port == spec.Ports[0].TargetPort.IntVal {
			return fmt.Sprintf(":%d", spec.Ports[0].Port)
		}
		return fmt.Sprintf(":%d -> %s", spec.Ports[0].Port, spec.Ports[0].TargetPort.String())
	default:
		pairs := []string{}
		for _, port := range spec.Ports {
			if port.TargetPort.String() == "0" || spec.PortalIP == kapi.PortalIPNone {
				pairs = append(pairs, fmt.Sprintf("%d", port.Port))
				continue
			}
			pairs = append(pairs, fmt.Sprintf("%d->%s", port.Port, port.TargetPort.String()))
		}
		return " " + strings.Join(pairs, ", ")
	}
}
