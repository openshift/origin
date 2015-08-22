package describe

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	kctl "k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
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
	buildColumns            = []string{"NAME", "TYPE", "STATUS", "POD"}
	buildConfigColumns      = []string{"NAME", "TYPE", "SOURCE"}
	imageColumns            = []string{"NAME", "DOCKER REF"}
	imageStreamTagColumns   = []string{"NAME", "DOCKER REF", "UPDATED", "IMAGENAME"}
	imageStreamImageColumns = []string{"NAME", "DOCKER REF", "UPDATED", "IMAGENAME"}
	imageStreamColumns      = []string{"NAME", "DOCKER REPO", "TAGS", "UPDATED"}
	projectColumns          = []string{"NAME", "DISPLAY NAME", "STATUS"}
	routeColumns            = []string{"NAME", "HOST/PORT", "PATH", "SERVICE", "LABELS", "TLS TERMINATION"}
	deploymentColumns       = []string{"NAME", "STATUS", "CAUSE"}
	deploymentConfigColumns = []string{"NAME", "TRIGGERS", "LATEST VERSION"}
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
	clusterNetworkColumns = []string{"NAME", "NETWORK", "HOST SUBNET LENGTH"}
)

// NewHumanReadablePrinter returns a new HumanReadablePrinter
func NewHumanReadablePrinter(noHeaders, withNamespace, wide bool, columnLabels []string) *kctl.HumanReadablePrinter {
	// TODO: support cross namespace listing
	p := kctl.NewHumanReadablePrinter(noHeaders, withNamespace, wide, columnLabels)
	p.Handler(buildColumns, printBuild)
	p.Handler(buildColumns, printBuildList)
	p.Handler(buildConfigColumns, printBuildConfig)
	p.Handler(buildConfigColumns, printBuildConfigList)
	p.Handler(imageColumns, printImage)
	p.Handler(imageStreamTagColumns, printImageStreamTag)
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

func printTemplate(t *templateapi.Template, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
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
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", t.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", t.Name, description, params, len(t.Objects))
	return err
}

func printTemplateList(list *templateapi.TemplateList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, t := range list.Items {
		if err := printTemplate(&t, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printBuild(build *buildapi.Build, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", build.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", build.Name, describeStrategy(build.Spec.Strategy.Type), build.Status.Phase, buildutil.GetBuildPodName(build))
	return err
}

func printBuildList(buildList *buildapi.BuildList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	builds := buildList.Items
	sort.Sort(buildapi.BuildSliceByCreationTimestamp(builds))
	for _, build := range builds {
		if err := printBuild(&build, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printBuildConfig(bc *buildapi.BuildConfig, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	if bc.Spec.Strategy.Type == buildapi.CustomBuildStrategyType {
		_, err := fmt.Fprintf(w, "%s\t%v\t%s\n", bc.Name, describeStrategy(bc.Spec.Strategy.Type), bc.Spec.Strategy.CustomStrategy.From.Name)
		return err
	}

	uri := "MISSING"
	if bc.Spec.Source.Git != nil {
		uri = bc.Spec.Source.Git.URI
	}

	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", bc.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%v\t%s\n", bc.Name, describeStrategy(bc.Spec.Strategy.Type), uri)
	return err
}

func printBuildConfigList(buildList *buildapi.BuildConfigList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, buildConfig := range buildList.Items {
		if err := printBuildConfig(&buildConfig, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printImage(image *imageapi.Image, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", image.Name, image.DockerImageReference)
	return err
}

func printImageStreamTag(ist *imageapi.ImageStreamTag, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	created := fmt.Sprintf("%s ago", formatRelativeTime(ist.CreationTimestamp.Time))
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", ist.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ist.Name, ist.Image.DockerImageReference, created, ist.Image.Name)
	return err
}

func printImageStreamImage(isi *imageapi.ImageStreamImage, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	created := fmt.Sprintf("%s ago", formatRelativeTime(isi.CreationTimestamp.Time))
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", isi.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", isi.Name, isi.Image.DockerImageReference, created, isi.Image.Name)
	return err
}

func printImageList(images *imageapi.ImageList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, image := range images.Items {
		if err := printImage(&image, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printImageStream(stream *imageapi.ImageStream, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	tags := ""
	set := util.NewStringSet()
	var latest util.Time
	for tag, list := range stream.Status.Tags {
		set.Insert(tag)
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
	tags = strings.Join(set.List(), ",")
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", stream.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", stream.Name, stream.Status.DockerImageRepository, tags, latestTime)
	return err
}

func printImageStreamList(streams *imageapi.ImageStreamList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, stream := range streams.Items {
		if err := printImageStream(&stream, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printProject(project *projectapi.Project, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
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

func printProjectList(projects *projectapi.ProjectList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	sort.Sort(SortableProjects(projects.Items))
	for _, project := range projects.Items {
		if err := printProject(&project, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printRoute(route *routeapi.Route, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	tlsTerm := ""
	if route.TLS != nil {
		tlsTerm = string(route.TLS.Termination)
	}
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", route.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", route.Name, route.Host, route.Path, route.ServiceName, labels.Set(route.Labels), tlsTerm)
	return err
}

func printRouteList(routeList *routeapi.RouteList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, route := range routeList.Items {
		if err := printRoute(&route, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printDeploymentConfig(dc *deployapi.DeploymentConfig, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	triggers := util.StringSet{}
	for _, trigger := range dc.Triggers {
		triggers.Insert(string(trigger.Type))
	}
	tStr := strings.Join(triggers.List(), ", ")

	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", dc.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%v\n", dc.Name, tStr, dc.LatestVersion)
	return err
}

func printDeploymentConfigList(list *deployapi.DeploymentConfigList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, dc := range list.Items {
		if err := printDeploymentConfig(&dc, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}

	return nil
}

func printPolicy(policy *authorizationapi.Policy, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	roleNames := util.StringSet{}
	for key := range policy.Roles {
		roleNames.Insert(key)
	}
	rolesString := strings.Join(roleNames.List(), ", ")

	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", policy.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%v\n", policy.Name, rolesString, policy.LastModified)
	return err
}

func printPolicyList(list *authorizationapi.PolicyList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, policy := range list.Items {
		if err := printPolicy(&policy, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}

	return nil
}

func printPolicyBinding(policyBinding *authorizationapi.PolicyBinding, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	roleBindingNames := util.StringSet{}
	for key := range policyBinding.RoleBindings {
		roleBindingNames.Insert(key)
	}
	roleBindingsString := strings.Join(roleBindingNames.List(), ", ")

	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", policyBinding.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%v\n", policyBinding.Name, roleBindingsString, policyBinding.LastModified)
	return err
}

func printPolicyBindingList(list *authorizationapi.PolicyBindingList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, policyBinding := range list.Items {
		if err := printPolicyBinding(&policyBinding, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}

	return nil
}

func printClusterPolicy(policy *authorizationapi.ClusterPolicy, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printPolicy(authorizationapi.ToPolicy(policy), w, withNamespace, wide, columnLabels)
}

func printClusterPolicyList(list *authorizationapi.ClusterPolicyList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printPolicyList(authorizationapi.ToPolicyList(list), w, withNamespace, wide, columnLabels)
}

func printClusterPolicyBinding(policyBinding *authorizationapi.ClusterPolicyBinding, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printPolicyBinding(authorizationapi.ToPolicyBinding(policyBinding), w, withNamespace, wide, columnLabels)
}

func printClusterPolicyBindingList(list *authorizationapi.ClusterPolicyBindingList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printPolicyBindingList(authorizationapi.ToPolicyBindingList(list), w, withNamespace, wide, columnLabels)
}

func printClusterRole(role *authorizationapi.ClusterRole, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printRole(authorizationapi.ToRole(role), w, withNamespace, wide, columnLabels)
}

func printClusterRoleList(list *authorizationapi.ClusterRoleList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printRoleList(authorizationapi.ToRoleList(list), w, withNamespace, wide, columnLabels)
}

func printClusterRoleBinding(roleBinding *authorizationapi.ClusterRoleBinding, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printRoleBinding(authorizationapi.ToRoleBinding(roleBinding), w, withNamespace, wide, columnLabels)
}

func printClusterRoleBindingList(list *authorizationapi.ClusterRoleBindingList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	return printRoleBindingList(authorizationapi.ToRoleBindingList(list), w, withNamespace, wide, columnLabels)
}

func printIsPersonalSubjectAccessReview(a *authorizationapi.IsPersonalSubjectAccessReview, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "IsPersonalSubjectAccessReview\n")
	return err
}

func printRole(role *authorizationapi.Role, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", role.Namespace); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s\n", role.Name)
	return err
}

func printRoleList(list *authorizationapi.RoleList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, role := range list.Items {
		if err := printRole(&role, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}

	return nil
}

func printRoleBinding(roleBinding *authorizationapi.RoleBinding, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	if withNamespace {
		if _, err := fmt.Fprintf(w, "%s\t", roleBinding.Namespace); err != nil {
			return err
		}
	}
	users, groups, sas, others := authorizationapi.SubjectsStrings(roleBinding.Namespace, roleBinding.Subjects)

	_, err := fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%v\t%v\n", roleBinding.Name, roleBinding.RoleRef.Namespace+"/"+roleBinding.RoleRef.Name, strings.Join(users, ", "), strings.Join(groups, ", "), strings.Join(sas, ", "), strings.Join(others, ", "))
	return err
}

func printRoleBindingList(list *authorizationapi.RoleBindingList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, roleBinding := range list.Items {
		if err := printRoleBinding(&roleBinding, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}

	return nil
}

func printOAuthClient(client *oauthapi.OAuthClient, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	challenge := "FALSE"
	if client.RespondWithChallenges {
		challenge = "TRUE"
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", client.Name, client.Secret, challenge, strings.Join(client.RedirectURIs, ","))
	return err
}
func printOAuthClientList(list *oauthapi.OAuthClientList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printOAuthClient(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthClientAuthorization(auth *oauthapi.OAuthClientAuthorization, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", auth.Name, auth.UserName, auth.ClientName, strings.Join(auth.Scopes, ","))
	return err
}
func printOAuthClientAuthorizationList(list *oauthapi.OAuthClientAuthorizationList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printOAuthClientAuthorization(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthAccessToken(token *oauthapi.OAuthAccessToken, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	created := token.CreationTimestamp
	expires := created.Add(time.Duration(token.ExpiresIn) * time.Second)
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", token.Name, token.UserName, token.ClientName, created, expires, token.RedirectURI, strings.Join(token.Scopes, ","))
	return err
}
func printOAuthAccessTokenList(list *oauthapi.OAuthAccessTokenList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printOAuthAccessToken(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthAuthorizeToken(token *oauthapi.OAuthAuthorizeToken, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	created := token.CreationTimestamp
	expires := created.Add(time.Duration(token.ExpiresIn) * time.Second)
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", token.Name, token.UserName, token.ClientName, created, expires, token.RedirectURI, strings.Join(token.Scopes, ","))
	return err
}
func printOAuthAuthorizeTokenList(list *oauthapi.OAuthAuthorizeTokenList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printOAuthAuthorizeToken(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printUser(user *userapi.User, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", user.Name, user.UID, user.FullName, strings.Join(user.Identities, ", "))
	return err
}
func printUserList(list *userapi.UserList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printUser(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printIdentity(identity *userapi.Identity, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", identity.Name, identity.ProviderName, identity.ProviderUserName, identity.User.Name, identity.User.UID)
	return err
}
func printIdentityList(list *userapi.IdentityList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printIdentity(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printUserIdentityMapping(mapping *userapi.UserIdentityMapping, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", mapping.Name, mapping.Identity.Name, mapping.User.Name, mapping.User.UID)
	return err
}

func printGroup(group *userapi.Group, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", group.Name, strings.Join(group.Users, ", "))
	return err
}
func printGroupList(list *userapi.GroupList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printGroup(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printHostSubnet(h *sdnapi.HostSubnet, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Name, h.Host, h.HostIP, h.Subnet)
	return err
}
func printHostSubnetList(list *sdnapi.HostSubnetList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printHostSubnet(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printNetNamespace(h *sdnapi.NetNamespace, w io.Writer, withNamespace bool, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%d\n", h.NetName, h.NetID)
	return err
}

func printNetNamespaceList(list *sdnapi.NetNamespaceList, w io.Writer, withNamespace bool, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printNetNamespace(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}

func printClusterNetwork(n *sdnapi.ClusterNetwork, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%d\n", n.Name, n.Network, n.HostSubnetLength)
	return err
}

func printClusterNetworkList(list *sdnapi.ClusterNetworkList, w io.Writer, withNamespace, wide bool, columnLabels []string) error {
	for _, item := range list.Items {
		if err := printClusterNetwork(&item, w, withNamespace, wide, columnLabels); err != nil {
			return err
		}
	}
	return nil
}
