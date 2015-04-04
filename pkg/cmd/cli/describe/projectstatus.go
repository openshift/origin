package describe

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
)

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
