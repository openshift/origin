package describe

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kctl "k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

var (
	buildColumns            = []string{"NAME", "TYPE", "FROM", "STATUS", "STARTED", "DURATION"}
	buildConfigColumns      = []string{"NAME", "TYPE", "FROM", "LATEST"}
	imageColumns            = []string{"NAME", "DOCKER REF"}
	imageStreamTagColumns   = []string{"NAME", "DOCKER REF", "UPDATED", "IMAGENAME"}
	imageStreamImageColumns = []string{"NAME", "DOCKER REF", "UPDATED", "IMAGENAME"}
	imageStreamColumns      = []string{"NAME", "DOCKER REPO", "TAGS", "UPDATED"}
	projectColumns          = []string{"NAME", "DISPLAY NAME", "STATUS"}
	routeColumns            = []string{"NAME", "HOST/PORT", "PATH", "SERVICE", "TERMINATION", "LABELS"}
	deploymentColumns       = []string{"NAME", "STATUS", "CAUSE"}
	deploymentConfigColumns = []string{"NAME", "REVISION", "REPLICAS", "TRIGGERED BY"}
	templateColumns         = []string{"NAME", "DESCRIPTION", "PARAMETERS", "OBJECTS"}
	policyColumns           = []string{"NAME", "ROLES", "LAST MODIFIED"}
	policyBindingColumns    = []string{"NAME", "ROLE BINDINGS", "LAST MODIFIED"}
	roleBindingColumns      = []string{"NAME", "ROLE", "USERS", "GROUPS", "SERVICE ACCOUNTS", "SUBJECTS"}
	roleColumns             = []string{"NAME"}

	oauthClientColumns              = []string{"NAME", "SECRET", "WWW-CHALLENGE", "REDIRECT URIS"}
	oauthClientAuthorizationColumns = []string{"NAME", "USER NAME", "CLIENT NAME", "SCOPES"}
	oauthAccessTokenColumns         = []string{"NAME", "USER NAME", "CLIENT NAME", "CREATED", "EXPIRES", "REDIRECT URI", "SCOPES"}
	oauthAuthorizeTokenColumns      = []string{"NAME", "USER NAME", "CLIENT NAME", "CREATED", "EXPIRES", "REDIRECT URI", "SCOPES"}

	userColumns                = []string{"NAME", "UID", "FULL NAME", "IDENTITIES"}
	identityColumns            = []string{"NAME", "IDP NAME", "IDP USER NAME", "USER NAME", "USER UID"}
	userIdentityMappingColumns = []string{"NAME", "IDENTITY", "USER NAME", "USER UID"}
	groupColumns               = []string{"NAME", "USERS"}

	// IsPersonalSubjectAccessReviewColumns contains known custom role extensions
	IsPersonalSubjectAccessReviewColumns = []string{"NAME"}

	hostSubnetColumns     = []string{"NAME", "HOST", "HOST IP", "SUBNET"}
	netNamespaceColumns   = []string{"NAME", "NETID"}
	clusterNetworkColumns = []string{"NAME", "NETWORK", "HOST SUBNET LENGTH", "SERVICE NETWORK", "PLUGIN NAME"}
)

// NewHumanReadablePrinter returns a new HumanReadablePrinter
func NewHumanReadablePrinter(noHeaders, withNamespace, wide bool, showAll bool, showLabels bool, absoluteTimestamps bool, columnLabels []string) *kctl.HumanReadablePrinter {
	// TODO: support cross namespace listing
	p := kctl.NewHumanReadablePrinter(noHeaders, withNamespace, wide, showAll, showLabels, absoluteTimestamps, columnLabels)
	p.Handler(buildColumns, printBuild)
	p.Handler(buildColumns, printBuildList)
	p.Handler(buildConfigColumns, printBuildConfig)
	p.Handler(buildConfigColumns, printBuildConfigList)
	p.Handler(imageColumns, printImage)
	p.Handler(imageStreamTagColumns, printImageStreamTag)
	p.Handler(imageStreamTagColumns, printImageStreamTagList)
	p.Handler(imageStreamImageColumns, printImageStreamImage)
	p.Handler(imageColumns, printImageList)
	p.Handler(imageStreamColumns, printImageStream)
	p.Handler(imageStreamColumns, printImageStreamList)
	p.Handler(projectColumns, printProject)
	p.Handler(projectColumns, printProjectList)
	p.Handler(routeColumns, printRoute)
	p.Handler(routeColumns, printRouteList)
	p.Handler(deploymentConfigColumns, printDeploymentConfig)
	p.Handler(deploymentConfigColumns, printDeploymentConfigList)
	p.Handler(templateColumns, printTemplate)
	p.Handler(templateColumns, printTemplateList)

	p.Handler(policyColumns, printPolicy)
	p.Handler(policyColumns, printPolicyList)
	p.Handler(policyBindingColumns, printPolicyBinding)
	p.Handler(policyBindingColumns, printPolicyBindingList)
	p.Handler(roleBindingColumns, printRoleBinding)
	p.Handler(roleBindingColumns, printRoleBindingList)
	p.Handler(roleColumns, printRole)
	p.Handler(roleColumns, printRoleList)

	p.Handler(policyColumns, printClusterPolicy)
	p.Handler(policyColumns, printClusterPolicyList)
	p.Handler(policyBindingColumns, printClusterPolicyBinding)
	p.Handler(policyBindingColumns, printClusterPolicyBindingList)
	p.Handler(roleColumns, printClusterRole)
	p.Handler(roleColumns, printClusterRoleList)
	p.Handler(roleBindingColumns, printClusterRoleBinding)
	p.Handler(roleBindingColumns, printClusterRoleBindingList)

	p.Handler(oauthClientColumns, printOAuthClient)
	p.Handler(oauthClientColumns, printOAuthClientList)
	p.Handler(oauthClientAuthorizationColumns, printOAuthClientAuthorization)
	p.Handler(oauthClientAuthorizationColumns, printOAuthClientAuthorizationList)
	p.Handler(oauthAccessTokenColumns, printOAuthAccessToken)
	p.Handler(oauthAccessTokenColumns, printOAuthAccessTokenList)
	p.Handler(oauthAuthorizeTokenColumns, printOAuthAuthorizeToken)
	p.Handler(oauthAuthorizeTokenColumns, printOAuthAuthorizeTokenList)

	p.Handler(userColumns, printUser)
	p.Handler(userColumns, printUserList)
	p.Handler(identityColumns, printIdentity)
	p.Handler(identityColumns, printIdentityList)
	p.Handler(userIdentityMappingColumns, printUserIdentityMapping)
	p.Handler(groupColumns, printGroup)
	p.Handler(groupColumns, printGroupList)

	p.Handler(IsPersonalSubjectAccessReviewColumns, printIsPersonalSubjectAccessReview)

	p.Handler(hostSubnetColumns, printHostSubnet)
	p.Handler(hostSubnetColumns, printHostSubnetList)
	p.Handler(netNamespaceColumns, printNetNamespaceList)
	p.Handler(netNamespaceColumns, printNetNamespace)
	p.Handler(clusterNetworkColumns, printClusterNetwork)
	p.Handler(clusterNetworkColumns, printClusterNetworkList)

	return p
}

const templateDescriptionLen = 80

// PrintTemplateParameters the Template parameters with their default values
func PrintTemplateParameters(params []templateapi.Parameter, output io.Writer) error {
	w := tabwriter.NewWriter(output, 20, 5, 3, ' ', 0)
	defer w.Flush()
	parameterColumns := []string{"NAME", "DESCRIPTION", "GENERATOR", "VALUE"}
	fmt.Fprintf(w, "%s\n", strings.Join(parameterColumns, "\t"))
	for _, p := range params {
		value := p.Value
		if len(p.Generate) != 0 {
			value = p.From
		}
		_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Description, p.Generate, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func printTemplate(t *templateapi.Template, w io.Writer, opts kctl.PrintOptions) error {
	description := ""
	if t.Annotations != nil {
		description = t.Annotations["description"]
	}
	if len(description) > templateDescriptionLen {
		description = strings.TrimSpace(description[:templateDescriptionLen-3]) + "..."
	}
	empty, generated, total := 0, 0, len(t.Parameters)
	for _, p := range t.Parameters {
		if len(p.Value) > 0 {
			continue
		}
		if len(p.Generate) > 0 {
			generated++
			continue
		}
		empty++
	}
	params := ""
	switch {
	case empty > 0:
		params = fmt.Sprintf("%d (%d blank)", total, empty)
	case generated > 0:
		params = fmt.Sprintf("%d (%d generated)", total, generated)
	default:
		params = fmt.Sprintf("%d (all set)", total)
	}
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", t.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d", t.Name, description, params, len(t.Objects)); err != nil {
		return err
	}
	if err := appendItemLabels(t.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printTemplateList(list *templateapi.TemplateList, w io.Writer, opts kctl.PrintOptions) error {
	for _, t := range list.Items {
		if err := printTemplate(&t, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printBuild(build *buildapi.Build, w io.Writer, opts kctl.PrintOptions) error {
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", build.Namespace); err != nil {
			return err
		}
	}
	var created string
	if build.Status.StartTimestamp != nil {
		created = fmt.Sprintf("%s ago", formatRelativeTime(build.Status.StartTimestamp.Time))
	}
	var duration string
	if build.Status.Duration > 0 {
		duration = build.Status.Duration.String()
	}
	from := describeSourceShort(build.Spec.CommonSpec)
	status := string(build.Status.Phase)
	if len(build.Status.Reason) > 0 {
		status = fmt.Sprintf("%s (%s)", status, build.Status.Reason)
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s", build.Name, buildapi.StrategyType(build.Spec.Strategy), from, status, created, duration); err != nil {
		return err
	}
	if err := appendItemLabels(build.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func describeSourceShort(spec buildapi.CommonSpec) string {
	var from string
	switch source := spec.Source; {
	case source.Binary != nil:
		from = "Binary"
		if rev := describeSourceGitRevision(spec); len(rev) != 0 {
			from = fmt.Sprintf("%s@%s", from, rev)
		}
	case source.Dockerfile != nil && source.Git != nil:
		from = "Dockerfile,Git"
		if rev := describeSourceGitRevision(spec); len(rev) != 0 {
			from = fmt.Sprintf("%s@%s", from, rev)
		}
	case source.Dockerfile != nil:
		from = "Dockerfile"
	case source.Git != nil:
		from = "Git"
		if rev := describeSourceGitRevision(spec); len(rev) != 0 {
			from = fmt.Sprintf("%s@%s", from, rev)
		}
	default:
		from = buildapi.SourceType(source)
	}
	return from
}

var nonCommitRev = regexp.MustCompile("[^a-fA-F0-9]")

func describeSourceGitRevision(spec buildapi.CommonSpec) string {
	var rev string
	if spec.Revision != nil && spec.Revision.Git != nil {
		rev = spec.Revision.Git.Commit
	}
	if len(rev) == 0 && spec.Source.Git != nil {
		rev = spec.Source.Git.Ref
	}
	// if this appears to be a full Git commit hash, shorten it to 7 characters for brevity
	if !nonCommitRev.MatchString(rev) && len(rev) > 20 {
		rev = rev[:7]
	}
	return rev
}

func printBuildList(buildList *buildapi.BuildList, w io.Writer, opts kctl.PrintOptions) error {
	builds := buildList.Items
	sort.Sort(buildapi.BuildSliceByCreationTimestamp(builds))
	for _, build := range builds {
		if err := printBuild(&build, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printBuildConfig(bc *buildapi.BuildConfig, w io.Writer, opts kctl.PrintOptions) error {
	if bc.Spec.Strategy.CustomStrategy != nil {
		_, err := fmt.Fprintf(w, "%s\t%v\t%s\t%d\n", bc.Name, buildapi.StrategyType(bc.Spec.Strategy), bc.Spec.Strategy.CustomStrategy.From.Name, bc.Status.LastVersion)
		return err
	}

	from := describeSourceShort(bc.Spec.CommonSpec)

	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", bc.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s\t%v\t%s\t%d", bc.Name, buildapi.StrategyType(bc.Spec.Strategy), from, bc.Status.LastVersion); err != nil {
		return err
	}
	if err := appendItemLabels(bc.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printBuildConfigList(buildList *buildapi.BuildConfigList, w io.Writer, opts kctl.PrintOptions) error {
	for _, buildConfig := range buildList.Items {
		if err := printBuildConfig(&buildConfig, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printImage(image *imageapi.Image, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", image.Name, image.DockerImageReference)
	return err
}

func printImageStreamTag(ist *imageapi.ImageStreamTag, w io.Writer, opts kctl.PrintOptions) error {
	created := fmt.Sprintf("%s ago", formatRelativeTime(ist.CreationTimestamp.Time))
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", ist.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s", ist.Name, ist.Image.DockerImageReference, created, ist.Image.Name); err != nil {
		return err
	}
	if err := appendItemLabels(ist.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printImageStreamTagList(list *imageapi.ImageStreamTagList, w io.Writer, opts kctl.PrintOptions) error {
	for _, ist := range list.Items {
		if err := printImageStreamTag(&ist, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printImageStreamImage(isi *imageapi.ImageStreamImage, w io.Writer, opts kctl.PrintOptions) error {
	created := fmt.Sprintf("%s ago", formatRelativeTime(isi.CreationTimestamp.Time))
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", isi.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s", isi.Name, isi.Image.DockerImageReference, created, isi.Image.Name); err != nil {
		return err
	}
	if err := appendItemLabels(isi.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printImageList(images *imageapi.ImageList, w io.Writer, opts kctl.PrintOptions) error {
	for _, image := range images.Items {
		if err := printImage(&image, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printImageStream(stream *imageapi.ImageStream, w io.Writer, opts kctl.PrintOptions) error {
	tags := ""
	const numOfTagsShown = 3

	var latest unversioned.Time
	for _, list := range stream.Status.Tags {
		if len(list.Items) > 0 {
			if list.Items[0].Created.After(latest.Time) {
				latest = list.Items[0].Created
			}
		}
	}
	latestTime := ""
	if !latest.IsZero() {
		latestTime = fmt.Sprintf("%s ago", formatRelativeTime(latest.Time))
	}
	list := imageapi.SortStatusTags(stream.Status.Tags)
	more := false
	if len(list) > numOfTagsShown {
		list = list[:numOfTagsShown]
		more = true
	}
	tags = strings.Join(list, ",")
	if more {
		tags = fmt.Sprintf("%s + %d more...", tags, len(stream.Status.Tags)-numOfTagsShown)
	}
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", stream.Namespace); err != nil {
			return err
		}
	}
	repo := stream.Spec.DockerImageRepository
	if len(repo) == 0 {
		repo = stream.Status.DockerImageRepository
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s", stream.Name, repo, tags, latestTime); err != nil {
		return err
	}
	if err := appendItemLabels(stream.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printImageStreamList(streams *imageapi.ImageStreamList, w io.Writer, opts kctl.PrintOptions) error {
	for _, stream := range streams.Items {
		if err := printImageStream(&stream, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printProject(project *projectapi.Project, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", project.Name, project.Annotations[projectapi.ProjectDisplayName], project.Status.Phase)
	return err
}

// SortableProjects is a list of projects that can be sorted
type SortableProjects []projectapi.Project

func (list SortableProjects) Len() int {
	return len(list)
}

func (list SortableProjects) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list SortableProjects) Less(i, j int) bool {
	return list[i].ObjectMeta.Name < list[j].ObjectMeta.Name
}

func printProjectList(projects *projectapi.ProjectList, w io.Writer, opts kctl.PrintOptions) error {
	sort.Sort(SortableProjects(projects.Items))
	for _, project := range projects.Items {
		if err := printProject(&project, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printRoute(route *routeapi.Route, w io.Writer, opts kctl.PrintOptions) error {
	tlsTerm := ""
	insecurePolicy := ""
	if route.Spec.TLS != nil {
		tlsTerm = string(route.Spec.TLS.Termination)
		insecurePolicy = string(route.Spec.TLS.InsecureEdgeTerminationPolicy)
	}
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", route.Namespace); err != nil {
			return err
		}
	}
	var (
		matchedHost bool
		reason      string
		host        = route.Spec.Host

		admitted, errors = 0, 0
	)
	for _, ingress := range route.Status.Ingress {
		switch status, condition := routeapi.IngressConditionStatus(&ingress, routeapi.RouteAdmitted); status {
		case kapi.ConditionTrue:
			admitted++
			if !matchedHost {
				matchedHost = ingress.Host == route.Spec.Host
				host = ingress.Host
			}
		case kapi.ConditionFalse:
			reason = condition.Reason
			errors++
		}
	}
	switch {
	case route.Status.Ingress == nil:
		// this is the legacy case, we should continue to show the host when talking to servers
		// that have not set status ingress, since we can't distinguish this condition from there
		// being no routers.
	case admitted == 0 && errors > 0:
		host = reason
	case errors > 0:
		host = fmt.Sprintf("%s ... %d rejected", host, errors)
	case admitted == 0:
		host = "Pending"
	case admitted > 1:
		host = fmt.Sprintf("%s ... %d more", host, admitted-1)
	}
	var policy string
	switch {
	case len(tlsTerm) != 0 && len(insecurePolicy) != 0:
		policy = fmt.Sprintf("%s/%s", tlsTerm, insecurePolicy)
	case len(tlsTerm) != 0:
		policy = tlsTerm
	case len(insecurePolicy) != 0:
		policy = fmt.Sprintf("default/%s", insecurePolicy)
	default:
		policy = ""
	}
	svc := route.Spec.To.Name
	if route.Spec.Port != nil {
		svc = fmt.Sprintf("%s:%s", svc, route.Spec.Port.TargetPort.String())
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s", route.Name, host, route.Spec.Path, svc, policy, labels.Set(route.Labels)); err != nil {
		return err
	}
	return nil
}

func printRouteList(routeList *routeapi.RouteList, w io.Writer, opts kctl.PrintOptions) error {
	for _, route := range routeList.Items {
		if err := printRoute(&route, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printDeploymentConfig(dc *deployapi.DeploymentConfig, w io.Writer, opts kctl.PrintOptions) error {
	var scale string
	if dc.Spec.Test {
		scale = fmt.Sprintf("%d (during test)", dc.Spec.Replicas)
	} else {
		scale = fmt.Sprintf("%d", dc.Spec.Replicas)
	}

	containers := sets.NewString()
	if dc.Spec.Template != nil {
		for _, c := range dc.Spec.Template.Spec.Containers {
			containers.Insert(c.Name)
		}
	}
	//names := containers.List()
	referencedContainers := sets.NewString()

	triggers := sets.String{}
	for _, trigger := range dc.Spec.Triggers {
		switch t := trigger.Type; t {
		case deployapi.DeploymentTriggerOnConfigChange:
			triggers.Insert("config")
		case deployapi.DeploymentTriggerOnImageChange:
			if p := trigger.ImageChangeParams; p != nil && p.Automatic {
				var prefix string
				if len(containers) != 1 && !containers.HasAll(p.ContainerNames...) {
					sort.Sort(sort.StringSlice(p.ContainerNames))
					prefix = strings.Join(p.ContainerNames, ",") + ":"
				}
				referencedContainers.Insert(p.ContainerNames...)
				switch p.From.Kind {
				case "ImageStreamTag":
					triggers.Insert(fmt.Sprintf("image(%s%s)", prefix, p.From.Name))
				default:
					triggers.Insert(fmt.Sprintf("%s(%s%s)", p.From.Kind, prefix, p.From.Name))
				}
			}
		default:
			triggers.Insert(string(t))
		}
	}
	trigger := strings.Join(triggers.List(), ",")

	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", dc.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s\t%v\t%s\t%s", dc.Name, dc.Status.LatestVersion, scale, trigger); err != nil {
		return err
	}
	if err := appendItemLabels(dc.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}

	return nil
}

func printDeploymentConfigList(list *deployapi.DeploymentConfigList, w io.Writer, opts kctl.PrintOptions) error {
	for _, dc := range list.Items {
		if err := printDeploymentConfig(&dc, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printPolicy(policy *authorizationapi.Policy, w io.Writer, opts kctl.PrintOptions) error {
	roleNames := sets.String{}
	for key := range policy.Roles {
		roleNames.Insert(key)
	}
	rolesString := strings.Join(roleNames.List(), ", ")

	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", policy.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%v", policy.Name, rolesString, policy.LastModified); err != nil {
		return err
	}
	if err := appendItemLabels(policy.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printPolicyList(list *authorizationapi.PolicyList, w io.Writer, opts kctl.PrintOptions) error {
	for _, policy := range list.Items {
		if err := printPolicy(&policy, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printPolicyBinding(policyBinding *authorizationapi.PolicyBinding, w io.Writer, opts kctl.PrintOptions) error {
	roleBindingNames := sets.String{}
	for key := range policyBinding.RoleBindings {
		roleBindingNames.Insert(key)
	}
	roleBindingsString := strings.Join(roleBindingNames.List(), ", ")

	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", policyBinding.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%v", policyBinding.Name, roleBindingsString, policyBinding.LastModified); err != nil {
		return err
	}
	if err := appendItemLabels(policyBinding.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printPolicyBindingList(list *authorizationapi.PolicyBindingList, w io.Writer, opts kctl.PrintOptions) error {
	for _, policyBinding := range list.Items {
		if err := printPolicyBinding(&policyBinding, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printClusterPolicy(policy *authorizationapi.ClusterPolicy, w io.Writer, opts kctl.PrintOptions) error {
	return printPolicy(authorizationapi.ToPolicy(policy), w, opts)
}

func printClusterPolicyList(list *authorizationapi.ClusterPolicyList, w io.Writer, opts kctl.PrintOptions) error {
	return printPolicyList(authorizationapi.ToPolicyList(list), w, opts)
}

func printClusterPolicyBinding(policyBinding *authorizationapi.ClusterPolicyBinding, w io.Writer, opts kctl.PrintOptions) error {
	return printPolicyBinding(authorizationapi.ToPolicyBinding(policyBinding), w, opts)
}

func printClusterPolicyBindingList(list *authorizationapi.ClusterPolicyBindingList, w io.Writer, opts kctl.PrintOptions) error {
	return printPolicyBindingList(authorizationapi.ToPolicyBindingList(list), w, opts)
}

func printClusterRole(role *authorizationapi.ClusterRole, w io.Writer, opts kctl.PrintOptions) error {
	return printRole(authorizationapi.ToRole(role), w, opts)
}

func printClusterRoleList(list *authorizationapi.ClusterRoleList, w io.Writer, opts kctl.PrintOptions) error {
	return printRoleList(authorizationapi.ToRoleList(list), w, opts)
}

func printClusterRoleBinding(roleBinding *authorizationapi.ClusterRoleBinding, w io.Writer, opts kctl.PrintOptions) error {
	return printRoleBinding(authorizationapi.ToRoleBinding(roleBinding), w, opts)
}

func printClusterRoleBindingList(list *authorizationapi.ClusterRoleBindingList, w io.Writer, opts kctl.PrintOptions) error {
	return printRoleBindingList(authorizationapi.ToRoleBindingList(list), w, opts)
}

func printIsPersonalSubjectAccessReview(a *authorizationapi.IsPersonalSubjectAccessReview, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "IsPersonalSubjectAccessReview\n")
	return err
}

func printRole(role *authorizationapi.Role, w io.Writer, opts kctl.PrintOptions) error {
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", role.Namespace); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s", role.Name); err != nil {
		return err
	}
	if err := appendItemLabels(role.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printRoleList(list *authorizationapi.RoleList, w io.Writer, opts kctl.PrintOptions) error {
	for _, role := range list.Items {
		if err := printRole(&role, w, opts); err != nil {
			return err
		}
	}

	return nil
}

func printRoleBinding(roleBinding *authorizationapi.RoleBinding, w io.Writer, opts kctl.PrintOptions) error {
	if opts.WithNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", roleBinding.Namespace); err != nil {
			return err
		}
	}
	users, groups, sas, others := authorizationapi.SubjectsStrings(roleBinding.Namespace, roleBinding.Subjects)

	if _, err := fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%v\t%v", roleBinding.Name, roleBinding.RoleRef.Namespace+"/"+roleBinding.RoleRef.Name, strings.Join(users, ", "), strings.Join(groups, ", "), strings.Join(sas, ", "), strings.Join(others, ", ")); err != nil {
		return err
	}
	if err := appendItemLabels(roleBinding.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printRoleBindingList(list *authorizationapi.RoleBindingList, w io.Writer, opts kctl.PrintOptions) error {
	for _, roleBinding := range list.Items {
		if err := printRoleBinding(&roleBinding, w, opts); err != nil {
			return err
		}
	}

	return nil
}

func printOAuthClient(client *oauthapi.OAuthClient, w io.Writer, opts kctl.PrintOptions) error {
	challenge := "FALSE"
	if client.RespondWithChallenges {
		challenge = "TRUE"
	}
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v", client.Name, client.Secret, challenge, strings.Join(client.RedirectURIs, ",")); err != nil {
		return err
	}
	if err := appendItemLabels(client.Labels, w, opts.ColumnLabels, opts.ShowLabels); err != nil {
		return err
	}
	return nil
}

func printOAuthClientList(list *oauthapi.OAuthClientList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printOAuthClient(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthClientAuthorization(auth *oauthapi.OAuthClientAuthorization, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", auth.Name, auth.UserName, auth.ClientName, strings.Join(auth.Scopes, ","))
	return err
}

func printOAuthClientAuthorizationList(list *oauthapi.OAuthClientAuthorizationList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printOAuthClientAuthorization(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthAccessToken(token *oauthapi.OAuthAccessToken, w io.Writer, opts kctl.PrintOptions) error {
	created := token.CreationTimestamp
	expires := created.Add(time.Duration(token.ExpiresIn) * time.Second)
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", token.Name, token.UserName, token.ClientName, created, expires, token.RedirectURI, strings.Join(token.Scopes, ","))
	return err
}

func printOAuthAccessTokenList(list *oauthapi.OAuthAccessTokenList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printOAuthAccessToken(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthAuthorizeToken(token *oauthapi.OAuthAuthorizeToken, w io.Writer, opts kctl.PrintOptions) error {
	created := token.CreationTimestamp
	expires := created.Add(time.Duration(token.ExpiresIn) * time.Second)
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", token.Name, token.UserName, token.ClientName, created, expires, token.RedirectURI, strings.Join(token.Scopes, ","))
	return err
}

func printOAuthAuthorizeTokenList(list *oauthapi.OAuthAuthorizeTokenList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printOAuthAuthorizeToken(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printUser(user *userapi.User, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", user.Name, user.UID, user.FullName, strings.Join(user.Identities, ", "))
	return err
}

func printUserList(list *userapi.UserList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printUser(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printIdentity(identity *userapi.Identity, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", identity.Name, identity.ProviderName, identity.ProviderUserName, identity.User.Name, identity.User.UID)
	return err
}

func printIdentityList(list *userapi.IdentityList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printIdentity(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printUserIdentityMapping(mapping *userapi.UserIdentityMapping, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", mapping.Name, mapping.Identity.Name, mapping.User.Name, mapping.User.UID)
	return err
}

func printGroup(group *userapi.Group, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", group.Name, strings.Join(group.Users, ", "))
	return err
}

func printGroupList(list *userapi.GroupList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printGroup(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printHostSubnet(h *sdnapi.HostSubnet, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Name, h.Host, h.HostIP, h.Subnet)
	return err
}

func printHostSubnetList(list *sdnapi.HostSubnetList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printHostSubnet(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printNetNamespace(h *sdnapi.NetNamespace, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%d\n", h.NetName, h.NetID)
	return err
}

func printNetNamespaceList(list *sdnapi.NetNamespaceList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printNetNamespace(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func printClusterNetwork(n *sdnapi.ClusterNetwork, w io.Writer, opts kctl.PrintOptions) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", n.Name, n.Network, n.HostSubnetLength, n.ServiceNetwork, n.PluginName)
	return err
}

func printClusterNetworkList(list *sdnapi.ClusterNetworkList, w io.Writer, opts kctl.PrintOptions) error {
	for _, item := range list.Items {
		if err := printClusterNetwork(&item, w, opts); err != nil {
			return err
		}
	}
	return nil
}

func appendItemLabels(itemLabels map[string]string, w io.Writer, columnLabels []string, showLabels bool) error {
	if _, err := fmt.Fprint(w, kctl.AppendLabels(itemLabels, columnLabels)); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, kctl.AppendAllLabels(showLabels, itemLabels)); err != nil {
		return err
	}
	return nil
}
