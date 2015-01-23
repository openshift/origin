package describe

import (
	"fmt"
	"io"
	"strings"

	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

var (
	buildColumns            = []string{"NAME", "TYPE", "STATUS", "POD"}
	buildConfigColumns      = []string{"NAME", "TYPE", "SOURCE"}
	imageColumns            = []string{"NAME", "DOCKER REF"}
	imageRepositoryColumns  = []string{"NAME", "DOCKER REPO", "TAGS"}
	projectColumns          = []string{"NAME", "DISPLAY NAME", "NAMESPACE"}
	routeColumns            = []string{"NAME", "HOST/PORT", "PATH", "SERVICE", "LABELS"}
	deploymentColumns       = []string{"NAME", "STATUS", "CAUSE"}
	deploymentConfigColumns = []string{"NAME", "TRIGGERS", "LATEST VERSION"}
	parameterColumns        = []string{"NAME", "DESCRIPTION", "GENERATOR", "VALUE"}
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
	p.Handler(parameterColumns, printParameters)
	return p
}

func printParameters(t *templateapi.Template, w io.Writer) error {
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
	_, err := fmt.Fprintf(w, "%s\t%s\t%s\n", project.Name, project.DisplayName, project.Namespace)
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
