package appjson

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/openshift/api/apps/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/openshift/oc/pkg/helpers/newapp"
	"github.com/openshift/oc/pkg/helpers/newapp/app"
	"github.com/openshift/oc/pkg/helpers/newapp/docker/dockerfile"
)

type EnvVarOrString struct {
	Value  string
	EnvVar *EnvVar
}

type EnvVar struct {
	Description string
	Generator   string
	Value       string
	Required    bool
	Default     interface{}
}

func (e *EnvVarOrString) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return nil
	}
	if data[0] == '"' {
		e.Value = string(data[1 : len(data)-1])
		return nil
	}
	e.EnvVar = &EnvVar{}
	return json.Unmarshal(data, e.EnvVar)
}

type Formation struct {
	Quantity int32
	Size     string
	Command  string
}

type Buildpack struct {
	URL string `json:"url"`
}

type AppJSON struct {
	Name        string
	Description string
	Keywords    []string
	Repository  string
	Website     string
	Logo        string
	SuccessURL  string `json:"success_url"`
	Scripts     map[string]string
	Env         map[string]EnvVarOrString
	Formation   map[string]Formation
	Image       string
	Addons      []string
	Buildpacks  []Buildpack
}

type Generator struct {
	LocalPath string
	Name      string
	BaseImage string
}

// Generate accepts a path to an app.json file and generates a template from it
func (g *Generator) Generate(body []byte) (*templatev1.Template, error) {
	appJSON := &AppJSON{}
	if err := json.Unmarshal(body, appJSON); err != nil {
		return nil, err
	}

	klog.V(4).Infof("app.json: %#v", appJSON)

	name := g.Name
	if len(name) == 0 && len(g.LocalPath) > 0 {
		name = filepath.Base(g.LocalPath)
	}

	template := &templatev1.Template{}
	template.Name = name
	template.Annotations = make(map[string]string)
	template.Annotations["openshift.io/website"] = appJSON.Website
	template.Annotations["k8s.io/display-name"] = appJSON.Name
	template.Annotations["k8s.io/description"] = appJSON.Description
	template.Annotations["tags"] = strings.Join(appJSON.Keywords, ",")
	template.Annotations["iconURL"] = appJSON.Logo

	// create parameters and environment for containers
	allEnv := make(app.Environment)
	for k, v := range appJSON.Env {
		if v.EnvVar != nil {
			allEnv[k] = fmt.Sprintf("${%s}", k)
		}
	}
	envVars := allEnv.List()
	for _, v := range envVars {
		env := appJSON.Env[v.Name]
		if env.EnvVar == nil {
			continue
		}
		e := env.EnvVar
		displayName := v.Name
		displayName = strings.Join(strings.Split(strings.ToLower(displayName), "_"), " ")
		displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		param := templatev1.Parameter{
			Name:        v.Name,
			DisplayName: displayName,
			Description: e.Description,
			Value:       e.Value,
		}
		switch e.Generator {
		case "secret":
			param.Generate = "expression"
			param.From = "[a-zA-Z0-9]{14}"
		}
		if len(param.Value) == 0 && e.Default != nil {
			switch t := e.Default.(type) {
			case string:
				param.Value = t
			case float64, float32:
				out, _ := json.Marshal(t)
				param.Value = string(out)
			}
		}
		template.Parameters = append(template.Parameters, param)
	}

	warnings := make(map[string][]string)

	if len(appJSON.Formation) == 0 {
		klog.V(4).Infof("No formation in app.json, adding a default web")
		// TODO: read Procfile for command?
		appJSON.Formation = map[string]Formation{
			"web": {
				Quantity: 1,
			},
		}
		msg := "adding a default formation 'web' with scale 1"
		warnings[msg] = append(warnings[msg], "app.json")
	}

	formations := sets.NewString()
	for k := range appJSON.Formation {
		formations.Insert(k)
	}

	var primaryFormation = "web"
	if _, ok := appJSON.Formation["web"]; !ok || len(appJSON.Formation) == 1 {
		for k := range appJSON.Formation {
			primaryFormation = k
			break
		}
	}

	imageGen := app.NewImageRefGenerator()

	buildPath := appJSON.Repository
	if len(buildPath) == 0 && len(g.LocalPath) > 0 {
		buildPath = g.LocalPath
	}
	if len(buildPath) == 0 {
		return nil, fmt.Errorf("app.json did not contain a repository URL and no local path was specified")
	}

	repo, err := app.NewSourceRepository(buildPath, newapp.StrategyDocker)
	if err != nil {
		return nil, err
	}

	var ports []string

	var pipelines app.PipelineGroup
	baseImage := g.BaseImage
	if len(baseImage) == 0 {
		baseImage = appJSON.Image
	}
	if len(baseImage) == 0 {
		return nil, fmt.Errorf("Docker image required: provide an --image flag or 'image' key in app.json")
	}

	fakeDockerfile := heredoc.Docf(`
      # Generated from app.json
      FROM %s
    `, baseImage)

	dockerfilePath := filepath.Join(buildPath, "Dockerfile")
	if df, err := app.NewDockerfileFromFile(dockerfilePath); err == nil {
		repo.Info().Dockerfile = df
		repo.Info().Path = dockerfilePath
		ports = dockerfile.LastExposedPorts(df.AST())
	}
	// TODO: look for procfile for more info?

	image, err := imageGen.FromNameAndPorts(baseImage, ports)
	if err != nil {
		return nil, err
	}
	image.AsImageStream = true
	image.TagDirectly = true
	image.ObjectName = name
	image.Tag = "from"

	pipeline, err := app.NewPipelineBuilder(name, nil, nil, false).To(name).NewBuildPipeline(name, image, repo, false)
	if err != nil {
		return nil, err
	}

	// TODO: this should not be necessary
	pipeline.Build.Source.Name = name
	pipeline.Build.Source.DockerfileContents = fakeDockerfile
	pipeline.Name = name
	pipeline.Image.ObjectName = name
	klog.V(4).Infof("created pipeline %+v", pipeline)

	pipelines = append(pipelines, pipeline)

	var errs []error

	// create deployments for each formation
	var group app.PipelineGroup
	for _, component := range formations.List() {
		componentName := fmt.Sprintf("%s-%s", name, component)
		if formations.Len() == 1 {
			componentName = name
		}
		formationName := component
		formation := appJSON.Formation[component]

		inputImage := pipelines[0].Image

		inputImage.ContainerFn = func(c *corev1.Container) {
			for _, s := range ports {
				if port, err := strconv.Atoi(s); err == nil {
					c.Ports = append(c.Ports, corev1.ContainerPort{ContainerPort: int32(port)})
				}
			}
			if len(formation.Command) > 0 {
				c.Args = []string{formation.Command}
			} else {
				msg := "no command defined, defaulting to command in the Procfile"
				warnings[msg] = append(warnings[msg], formationName)
				c.Args = []string{"/bin/sh", "-c", fmt.Sprintf("$(grep %s Procfile | cut -f 2 -d :)", formationName)}
			}
			c.Env = append(c.Env, envVars...)

			c.Resources = resourcesForProfile(formation.Size)
		}

		pipeline, err := app.NewPipelineBuilder(componentName, nil, nil, true).To(componentName).NewImagePipeline(componentName, inputImage)
		if err != nil {
			errs = append(errs, err)
			break
		}

		if err := pipeline.NeedsDeployment(nil, nil, false); err != nil {
			return nil, err
		}

		if cmd, ok := appJSON.Scripts["postdeploy"]; ok && primaryFormation == component {
			pipeline.Deployment.PostHook = &app.DeploymentHook{Shell: cmd}
			delete(appJSON.Scripts, "postdeploy")
		}

		group = append(group, pipeline)
	}
	if err := group.Reduce(); err != nil {
		return nil, err
	}
	pipelines = append(pipelines, group...)

	if len(errs) > 0 {
		return nil, utilerrs.NewAggregate(errs)
	}

	acceptors := app.Acceptors{app.NewAcceptUnique(), app.AcceptNew}
	objects := app.Objects{}
	accept := app.NewAcceptFirst()
	for _, p := range pipelines {
		accepted, err := p.Objects(accept, acceptors)
		if err != nil {
			return nil, fmt.Errorf("can't setup %q: %v", p.From, err)
		}
		objects = append(objects, accepted...)
	}

	// create services for each object with a name based on alias.
	var services []*corev1.Service
	for _, obj := range objects {
		switch t := obj.(type) {
		case *appsv1.DeploymentConfig:
			ports := app.UniqueContainerToServicePorts(app.AllContainerPorts(t.Spec.Template.Spec.Containers...))
			if len(ports) == 0 {
				continue
			}
			svc := app.GenerateService(t.ObjectMeta, t.Spec.Selector)
			svc.Spec.Ports = ports
			services = append(services, svc)
		}
	}
	for _, svc := range services {
		objects = append(objects, svc)
	}

	template.Objects = appObjectsToTemplateObjects(objects)

	// generate warnings
	warnUnusableAppJSONElements("app.json", appJSON, warnings)
	if len(warnings) > 0 {
		allWarnings := sets.NewString()
		for msg, services := range warnings {
			allWarnings.Insert(fmt.Sprintf("%s: %s", strings.Join(services, ","), msg))
		}
		if template.Annotations == nil {
			template.Annotations = make(map[string]string)
		}
		template.Annotations[app.GenerationWarningAnnotation] = fmt.Sprintf("not all app.json fields were honored:\n* %s", strings.Join(allWarnings.List(), "\n* "))
	}

	return template, nil
}

func appObjectsToTemplateObjects(objs []runtime.Object) []runtime.RawExtension {
	rawObjs := []runtime.RawExtension{}
	for _, o := range objs {
		raw, err := json.Marshal(o)
		if err != nil {
			klog.V(1).Infof("error marshaling app object to raw bytes: %v", err)
			continue
		}

		rawObjs = append(rawObjs, runtime.RawExtension{
			Raw:    raw,
			Object: o,
		})
	}

	return rawObjs
}

// warnUnusableAppJSONElements add warnings for unsupported elements in the provided service config
func warnUnusableAppJSONElements(k string, v *AppJSON, warnings map[string][]string) {
	fn := func(msg string) {
		warnings[msg] = append(warnings[msg], k)
	}
	if len(v.Buildpacks) > 0 {
		fn("buildpacks are not handled")
	}
	for _, s := range v.Addons {
		fn(fmt.Sprintf("addon %q is not supported and must be added separately", s))
	}
	if len(v.SuccessURL) > 0 {
		fn("success_url is not handled")
	}
	for k, v := range v.Scripts {
		fn(fmt.Sprintf("script directive %q for %q is not handled", v, k))
	}
}

func checkForPorts(repo *app.SourceRepository) []string {
	info := repo.Info()
	if info == nil || info.Dockerfile == nil {
		return nil
	}
	node := info.Dockerfile.AST()
	return dockerfile.LastExposedPorts(node)
}

// resourcesForProfile takes standard Heroku sizes described here:
// https://devcenter.heroku.com/articles/dyno-types#available-dyno-types and turns them into
// Kubernetes resource requests.
func resourcesForProfile(profile string) corev1.ResourceRequirements {
	profile = strings.ToLower(profile)
	switch profile {
	case "standard-2x":
		return corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
	case "performance-m":
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("2.5Gi"),
			},
		}
	case "performance-l":
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2G"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("14Gi"),
			},
		}
	case "free", "hobby", "standard":
		fallthrough
	default:
		return corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		}
	}
}
