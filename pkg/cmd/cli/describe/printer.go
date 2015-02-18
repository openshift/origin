package describe

import (
	"fmt"
	"io"
	"strings"

	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

var (
	buildColumns            = []string{"NAME", "TYPE", "STATUS", "POD"}
	buildConfigColumns      = []string{"NAME", "TYPE", "SOURCE"}
	imageColumns            = []string{"NAME", "DOCKER REF"}
	imageRepositoryColumns  = []string{"NAME", "DOCKER REPO", "TAGS"}
	projectColumns          = []string{"NAME", "DISPLAY NAME"}
	routeColumns            = []string{"NAME", "HOST/PORT", "PATH", "SERVICE", "LABELS"}
	deploymentColumns       = []string{"NAME", "STATUS", "CAUSE"}
	deploymentConfigColumns = []string{"NAME", "TRIGGERS", "LATEST VERSION"}
	templateColumns         = []string{"NAME", "DESCRIPTION", "PARAMETERS", "OBJECTS"}
	parameterColumns        = []string{"NAME", "DESCRIPTION", "GENERATOR", "VALUE"}
	policyColumns           = []string{"NAME", "ROLES", "LAST MODIFIED"}
	policyBindingColumns    = []string{"NAME", "ROLE BINDINGS", "LAST MODIFIED"}

	oauthClientColumns              = []string{"NAME", "SECRET", "WWW-CHALLENGE", "REDIRECT URIS"}
	oauthClientAuthorizationColumns = []string{"NAME", "USER NAME", "CLIENT NAME", "SCOPES"}
	oauthAccessTokenColumns         = []string{"NAME", "USER NAME", "CLIENT NAME", "CREATED", "EXPIRES", "REDIRECT URI", "SCOPES"}
	oauthAuthorizeTokenColumns      = []string{"NAME", "USER NAME", "CLIENT NAME", "CREATED", "EXPIRES", "REDIRECT URI", "SCOPES"}

	userColumns                = []string{"NAME", "UID", "FULL NAME"}
	userIdentityMappingColumns = []string{"NAME", "IDENTITY PROVIDER", "IDENTITY USERNAME", "USER NAME"}
)

func NewHumanReadablePrinter(noHeaders bool) *kctl.HumanReadablePrinter {
	p := kctl.NewHumanReadablePrinter(noHeaders)
	p.Handler(buildColumns, printBuild)
	p.Handler(buildColumns, printBuildList)
	p.Handler(buildConfigColumns, printBuildConfig)
	p.Handler(buildConfigColumns, printBuildConfigList)
	p.Handler(imageColumns, printImage)
	p.Handler(imageColumns, printImageList)
	p.Handler(imageRepositoryColumns, printImageRepository)
	p.Handler(imageRepositoryColumns, printImageRepositoryList)
	p.Handler(projectColumns, printProject)
	p.Handler(projectColumns, printProjectList)
	p.Handler(routeColumns, printRoute)
	p.Handler(routeColumns, printRouteList)
	p.Handler(deploymentColumns, printDeployment)
	p.Handler(deploymentColumns, printDeploymentList)
	p.Handler(deploymentConfigColumns, printDeploymentConfig)
	p.Handler(deploymentConfigColumns, printDeploymentConfigList)
	p.Handler(templateColumns, printTemplate)
	p.Handler(templateColumns, printTemplateList)
	p.Handler(policyColumns, printPolicy)
	p.Handler(policyColumns, printPolicyList)
	p.Handler(policyBindingColumns, printPolicyBinding)
	p.Handler(policyBindingColumns, printPolicyBindingList)

	p.Handler(oauthClientColumns, printOAuthClient)
	p.Handler(oauthClientColumns, printOAuthClientList)
	p.Handler(oauthClientAuthorizationColumns, printOAuthClientAuthorization)
	p.Handler(oauthClientAuthorizationColumns, printOAuthClientAuthorizationList)
	p.Handler(oauthAccessTokenColumns, printOAuthAccessToken)
	p.Handler(oauthAccessTokenColumns, printOAuthAccessTokenList)
	p.Handler(oauthAuthorizeTokenColumns, printOAuthAuthorizeToken)
	p.Handler(oauthAuthorizeTokenColumns, printOAuthAuthorizeTokenList)

	p.Handler(userColumns, printUser)
	p.Handler(userIdentityMappingColumns, printUserIdentityMapping)
	return p
}

const templateDescriptionLen = 80

func printTemplate(t *templateapi.Template, w io.Writer) error {
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
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", t.Name, description, params, len(t.Objects))
	return err
}

func printTemplateParameters(t *templateapi.Template, w io.Writer) error {
	for _, p := range t.Parameters {
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

func printTemplateList(list *templateapi.TemplateList, w io.Writer) error {
	for _, t := range list.Items {
		if err := printTemplate(&t, w); err != nil {
			return err
		}
	}
	return nil
}

func printBuild(build *buildapi.Build, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", build.Name, build.Parameters.Strategy.Type, build.Status, build.PodName)
	return err
}

func printBuildList(buildList *buildapi.BuildList, w io.Writer) error {
	for _, build := range buildList.Items {
		if err := printBuild(&build, w); err != nil {
			return err
		}
	}
	return nil
}

func printBuildConfig(bc *buildapi.BuildConfig, w io.Writer) error {
	if bc.Parameters.Strategy.Type == buildapi.CustomBuildStrategyType {
		_, err := fmt.Fprintf(w, "%s\t%v\t%s\n", bc.Name, bc.Parameters.Strategy.Type, bc.Parameters.Strategy.CustomStrategy.Image)
		return err
	}
	_, err := fmt.Fprintf(w, "%s\t%v\t%s\n", bc.Name, bc.Parameters.Strategy.Type, bc.Parameters.Source.Git.URI)
	return err
}

func printBuildConfigList(buildList *buildapi.BuildConfigList, w io.Writer) error {
	for _, buildConfig := range buildList.Items {
		if err := printBuildConfig(&buildConfig, w); err != nil {
			return err
		}
	}
	return nil
}

func printImage(image *imageapi.Image, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", image.Name, image.DockerImageReference)
	return err
}

func printImageList(images *imageapi.ImageList, w io.Writer) error {
	for _, image := range images.Items {
		if err := printImage(&image, w); err != nil {
			return err
		}
	}
	return nil
}

func printImageRepository(repo *imageapi.ImageRepository, w io.Writer) error {
	tags := ""
	if len(repo.Tags) > 0 {
		var t []string
		for tag := range repo.Tags {
			t = append(t, tag)
		}
		tags = strings.Join(t, ",")
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", repo.Name, repo.Status.DockerImageRepository, tags)
	return err
}

func printImageRepositoryList(repos *imageapi.ImageRepositoryList, w io.Writer) error {
	for _, repo := range repos.Items {
		if err := printImageRepository(&repo, w); err != nil {
			return err
		}
	}
	return nil
}

func printProject(project *projectapi.Project, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\n", project.Name, project.DisplayName)
	return err
}

func printProjectList(projects *projectapi.ProjectList, w io.Writer) error {
	for _, project := range projects.Items {
		if err := printProject(&project, w); err != nil {
			return err
		}
	}
	return nil
}

func printRoute(route *routeapi.Route, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", route.Name, route.Host, route.Path, route.ServiceName, labels.Set(route.Labels))
	return err
}

func printRouteList(routeList *routeapi.RouteList, w io.Writer) error {
	for _, route := range routeList.Items {
		if err := printRoute(&route, w); err != nil {
			return err
		}
	}
	return nil
}

func printDeployment(d *deployapi.Deployment, w io.Writer) error {
	causes := util.StringSet{}
	if d.Details != nil {
		for _, cause := range d.Details.Causes {
			causes.Insert(string(cause.Type))
		}
	}
	cStr := strings.Join(causes.List(), ", ")
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", d.Name, d.Status, cStr)
	return err
}

func printDeploymentList(list *deployapi.DeploymentList, w io.Writer) error {
	for _, d := range list.Items {
		if err := printDeployment(&d, w); err != nil {
			return err
		}
	}

	return nil
}

func printDeploymentConfig(dc *deployapi.DeploymentConfig, w io.Writer) error {
	triggers := util.StringSet{}
	for _, trigger := range dc.Triggers {
		triggers.Insert(string(trigger.Type))
	}
	tStr := strings.Join(triggers.List(), ", ")

	_, err := fmt.Fprintf(w, "%s\t%s\t%v\n", dc.Name, tStr, dc.LatestVersion)
	return err
}

func printDeploymentConfigList(list *deployapi.DeploymentConfigList, w io.Writer) error {
	for _, dc := range list.Items {
		if err := printDeploymentConfig(&dc, w); err != nil {
			return err
		}
	}

	return nil
}

func printPolicy(policy *authorizationapi.Policy, w io.Writer) error {
	roleNames := util.StringSet{}
	for key := range policy.Roles {
		roleNames.Insert(key)
	}
	rolesString := strings.Join(roleNames.List(), ", ")

	_, err := fmt.Fprintf(w, "%s\t%s\t%v\n", policy.Name, rolesString, policy.LastModified)
	return err
}

func printPolicyList(list *authorizationapi.PolicyList, w io.Writer) error {
	for _, policy := range list.Items {
		if err := printPolicy(&policy, w); err != nil {
			return err
		}
	}

	return nil
}

func printPolicyBinding(policyBinding *authorizationapi.PolicyBinding, w io.Writer) error {
	roleBindingNames := util.StringSet{}
	for key := range policyBinding.RoleBindings {
		roleBindingNames.Insert(key)
	}
	roleBindingsString := strings.Join(roleBindingNames.List(), ", ")

	_, err := fmt.Fprintf(w, "%s\t%s\t%v\n", policyBinding.Name, roleBindingsString, policyBinding.LastModified)
	return err
}

func printPolicyBindingList(list *authorizationapi.PolicyBindingList, w io.Writer) error {
	for _, policyBinding := range list.Items {
		if err := printPolicyBinding(&policyBinding, w); err != nil {
			return err
		}
	}

	return nil
}

func printOAuthClient(client *oauthapi.OAuthClient, w io.Writer) error {
	challenge := "FALSE"
	if client.RespondWithChallenges {
		challenge = "TRUE"
	}
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", client.Name, client.Secret, challenge, strings.Join(client.RedirectURIs, ","))
	return err
}
func printOAuthClientList(list *oauthapi.OAuthClientList, w io.Writer) error {
	for _, item := range list.Items {
		if err := printOAuthClient(&item, w); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthClientAuthorization(auth *oauthapi.OAuthClientAuthorization, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", auth.Name, auth.UserName, auth.ClientName, strings.Join(auth.Scopes, ","))
	return err
}
func printOAuthClientAuthorizationList(list *oauthapi.OAuthClientAuthorizationList, w io.Writer) error {
	for _, item := range list.Items {
		if err := printOAuthClientAuthorization(&item, w); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthAccessToken(token *oauthapi.OAuthAccessToken, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n", token.Name, token.UserName, token.ClientName, token.CreationTimestamp, token.ExpiresIn, token.RedirectURI, strings.Join(token.Scopes, ","))
	return err
}
func printOAuthAccessTokenList(list *oauthapi.OAuthAccessTokenList, w io.Writer) error {
	for _, item := range list.Items {
		if err := printOAuthAccessToken(&item, w); err != nil {
			return err
		}
	}
	return nil
}

func printOAuthAuthorizeToken(token *oauthapi.OAuthAuthorizeToken, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n", token.Name, token.UserName, token.ClientName, token.CreationTimestamp, token.ExpiresIn, token.RedirectURI, strings.Join(token.Scopes, ","))
	return err
}
func printOAuthAuthorizeTokenList(list *oauthapi.OAuthAuthorizeTokenList, w io.Writer) error {
	for _, item := range list.Items {
		if err := printOAuthAuthorizeToken(&item, w); err != nil {
			return err
		}
	}
	return nil
}

func printUser(user *userapi.User, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", user.Name, user.UID, user.FullName)
	return err
}

func printUserIdentityMapping(mapping *userapi.UserIdentityMapping, w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", mapping.Name, mapping.Identity.Provider, mapping.Identity.UserName, mapping.User.Name)
	return err
}
