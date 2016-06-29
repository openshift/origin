package dockercompose

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	utilerrs "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/git"
	templateapi "github.com/openshift/origin/pkg/template/api"
	dockerfileutil "github.com/openshift/origin/pkg/util/docker/dockerfile"
	"github.com/openshift/origin/third_party/github.com/docker/libcompose/project"
)

func IsPossibleDockerCompose(path string) bool {
	switch base := filepath.Base(path); {
	case base == "docker-compose.yaml", base == "docker-compose.yml":
		return true
	default:
		return false
	}
}

// Generate accepts a set of Docker compose project paths and converts them in an
// OpenShift template definition.
func Generate(paths ...string) (*templateapi.Template, error) {
	for i := range paths {
		path, err := filepath.Abs(paths[i])
		if err != nil {
			return nil, err
		}
		paths[i] = path
	}
	var bases []string
	for _, s := range paths {
		bases = append(bases, filepath.Dir(s))
	}

	context := &project.Context{
		ComposeFiles: paths,
	}
	p := project.NewProject(context)
	if err := project.AddEnvironmentLookUp(context); err != nil {
		return nil, err
	}
	if err := p.Parse(); err != nil {
		return nil, err
	}
	template := &templateapi.Template{}
	template.Name = p.Name

	serviceOrder := sets.NewString()
	warnings := make(map[string][]string)
	for k, v := range p.Configs {
		serviceOrder.Insert(k)
		warnUnusableComposeElements(k, v, warnings)
	}

	g := app.NewImageRefGenerator()

	var errs []error
	var pipelines app.PipelineGroup
	builds := make(map[string]*app.Pipeline)

	// identify colocated components due to shared volumes
	joins := make(map[string]sets.String)
	volumesFrom := make(map[string][]string)
	for _, k := range serviceOrder.List() {
		if joins[k] == nil {
			joins[k] = sets.NewString(k)
		}
		v := p.Configs[k]
		for _, from := range v.VolumesFrom {
			switch parts := strings.Split(from, ":"); len(parts) {
			case 1:
				joins[k].Insert(parts[0])
				volumesFrom[k] = append(volumesFrom[k], parts[0])
			case 2:
				target := parts[1]
				if parts[1] == "ro" || parts[1] == "rw" {
					target = parts[0]
				}
				joins[k].Insert(target)
				volumesFrom[k] = append(volumesFrom[k], target)
			case 3:
				joins[k].Insert(parts[1])
				volumesFrom[k] = append(volumesFrom[k], parts[1])
			}
		}
	}
	joinOrder := sets.NewString()
	for k := range joins {
		joinOrder.Insert(k)
	}
	var colocated []sets.String
	for _, k := range joinOrder.List() {
		set := joins[k]
		matched := -1
		for i, existing := range colocated {
			if set.Intersection(existing).Len() == 0 {
				continue
			}
			if matched != -1 {
				return nil, fmt.Errorf("%q belongs with %v, but %v also contains some overlapping elements", k, set, colocated[matched])
			}
			existing.Insert(set.List()...)
			matched = i
			continue
		}
		if matched == -1 {
			colocated = append(colocated, set)
		}
	}

	// identify service aliases
	aliases := make(map[string]sets.String)
	for _, v := range p.Configs {
		for _, s := range v.Links.Slice() {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) != 2 || parts[0] == parts[1] {
				continue
			}
			set := aliases[parts[0]]
			if set == nil {
				set = sets.NewString()
				aliases[parts[0]] = set
			}
			set.Insert(parts[1])
		}
	}

	// find and define build pipelines
	for _, k := range serviceOrder.List() {
		v := p.Configs[k]
		if len(v.Build) == 0 {
			continue
		}
		if _, ok := builds[v.Build]; ok {
			continue
		}
		var base, relative string
		for _, s := range bases {
			if !strings.HasPrefix(v.Build, s) {
				continue
			}
			base = s
			path, err := filepath.Rel(base, v.Build)
			if err != nil {
				return nil, fmt.Errorf("path is not relative to base: %v", err)
			}
			relative = path
			break
		}
		if len(base) == 0 {
			return nil, fmt.Errorf("build path outside of the compose file: %s", v.Build)
		}

		// if this is a Git repository, make the path relative
		if root, err := git.NewRepository().GetRootDir(base); err == nil {
			if relative, err = filepath.Rel(root, filepath.Join(base, relative)); err != nil {
				return nil, fmt.Errorf("unable to find relative path for Git repository: %v", err)
			}
			base = root
		}
		buildPath := filepath.Join(base, relative)

		// TODO: what if there is no origin for this repo?

		glog.V(4).Infof("compose service: %#v", v)
		repo, err := app.NewSourceRepositoryWithDockerfile(buildPath, "")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		repo.BuildWithDocker()

		info := repo.Info()
		if info == nil || info.Dockerfile == nil {
			errs = append(errs, fmt.Errorf("unable to locate a Dockerfile in %s", v.Build))
			continue
		}
		node := info.Dockerfile.AST()
		baseImage := dockerfileutil.LastBaseImage(node)
		if len(baseImage) == 0 {
			errs = append(errs, fmt.Errorf("the Dockerfile in the repository %q has no FROM instruction", info.Path))
			continue
		}

		var ports []string
		for _, s := range v.Ports {
			container, _ := extractFirstPorts(s)
			ports = append(ports, container)
		}

		image, err := g.FromNameAndPorts(baseImage, ports)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		image.AsImageStream = true
		image.TagDirectly = true
		image.ObjectName = k
		image.Tag = "from"

		pipeline, err := app.NewPipelineBuilder(k, nil, false).To(k).NewBuildPipeline(k, image, repo)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(relative) > 0 {
			pipeline.Build.Source.ContextDir = relative
		}
		// TODO: this should not be necessary
		pipeline.Build.Source.Name = k
		pipeline.Name = k
		pipeline.Image.ObjectName = k
		glog.V(4).Infof("created pipeline %+v", pipeline)

		builds[v.Build] = pipeline
		pipelines = append(pipelines, pipeline)
	}

	if len(errs) > 0 {
		return nil, utilerrs.NewAggregate(errs)
	}

	// create deployment groups
	for _, pod := range colocated {
		var group app.PipelineGroup
		commonMounts := make(map[string]string)
		for _, k := range pod.List() {
			v := p.Configs[k]
			glog.V(4).Infof("compose service: %#v", v)
			var inputImage *app.ImageRef
			if len(v.Image) != 0 {
				image, err := g.FromName(v.Image)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				image.AsImageStream = true
				image.TagDirectly = true
				image.ObjectName = k

				inputImage = image
			}
			if inputImage == nil {
				if previous, ok := builds[v.Build]; ok {
					inputImage = previous.Image
				}
			}
			if inputImage == nil {
				errs = append(errs, fmt.Errorf("could not find an input image for %q", k))
				continue
			}

			inputImage.ContainerFn = func(c *kapi.Container) {
				if len(v.ContainerName) > 0 {
					c.Name = v.ContainerName
				}
				for _, s := range v.Ports {
					container, _ := extractFirstPorts(s)
					if port, err := strconv.Atoi(container); err == nil {
						c.Ports = append(c.Ports, kapi.ContainerPort{ContainerPort: int32(port)})
					}
				}
				c.Args = v.Command.Slice()
				if len(v.Entrypoint.Slice()) > 0 {
					c.Command = v.Entrypoint.Slice()
				}
				if len(v.WorkingDir) > 0 {
					c.WorkingDir = v.WorkingDir
				}
				c.Env = append(c.Env, app.ParseEnvironment(v.Environment.Slice()...).List()...)
				if uid, err := strconv.Atoi(v.User); err == nil {
					uid64 := int64(uid)
					if c.SecurityContext == nil {
						c.SecurityContext = &kapi.SecurityContext{}
					}
					c.SecurityContext.RunAsUser = &uid64
				}
				c.TTY = v.Tty
				if v.StdinOpen {
					c.StdinOnce = true
					c.Stdin = true
				}
				if v.Privileged {
					if c.SecurityContext == nil {
						c.SecurityContext = &kapi.SecurityContext{}
					}
					t := true
					c.SecurityContext.Privileged = &t
				}
				if v.ReadOnly {
					if c.SecurityContext == nil {
						c.SecurityContext = &kapi.SecurityContext{}
					}
					t := true
					c.SecurityContext.ReadOnlyRootFilesystem = &t
				}
				if v.MemLimit > 0 {
					q := resource.NewQuantity(v.MemLimit, resource.DecimalSI)
					if c.Resources.Limits == nil {
						c.Resources.Limits = make(kapi.ResourceList)
					}
					c.Resources.Limits[kapi.ResourceMemory] = *q
				}

				if quota := v.CPUQuota; quota > 0 {
					if quota < 1000 {
						quota = 1000 // minQuotaPeriod
					}
					milliCPU := quota * 1000     // milliCPUtoCPU
					milliCPU = milliCPU / 100000 // quotaPeriod
					q := resource.NewMilliQuantity(milliCPU, resource.DecimalSI)
					if c.Resources.Limits == nil {
						c.Resources.Limits = make(kapi.ResourceList)
					}
					c.Resources.Limits[kapi.ResourceCPU] = *q
				}
				if shares := v.CPUShares; shares > 0 {
					if shares < 2 {
						shares = 2 // minShares
					}
					milliCPU := shares * 1000  // milliCPUtoCPU
					milliCPU = milliCPU / 1024 // sharesPerCPU
					q := resource.NewMilliQuantity(milliCPU, resource.DecimalSI)
					if c.Resources.Requests == nil {
						c.Resources.Requests = make(kapi.ResourceList)
					}
					c.Resources.Requests[kapi.ResourceCPU] = *q
				}

				mountPoints := make(map[string][]string)
				for _, s := range v.Volumes {
					switch parts := strings.SplitN(s, ":", 3); len(parts) {
					case 1:
						mountPoints[""] = append(mountPoints[""], parts[0])

					case 2:
						fallthrough
					default:
						mountPoints[parts[0]] = append(mountPoints[parts[0]], parts[1])
					}
				}
				for from, at := range mountPoints {
					name, ok := commonMounts[from]
					if !ok {
						name = fmt.Sprintf("dir-%d", len(commonMounts)+1)
						commonMounts[from] = name
					}
					for _, path := range at {
						c.VolumeMounts = append(c.VolumeMounts, kapi.VolumeMount{Name: name, MountPath: path})
					}
				}
			}

			pipeline, err := app.NewPipelineBuilder(k, nil, true).To(k).NewImagePipeline(k, inputImage)
			if err != nil {
				errs = append(errs, err)
				break
			}

			if err := pipeline.NeedsDeployment(nil, nil, false); err != nil {
				return nil, err
			}

			group = append(group, pipeline)
		}
		if err := group.Reduce(); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, group...)
	}

	if len(errs) > 0 {
		return nil, utilerrs.NewAggregate(errs)
	}

	acceptors := app.Acceptors{app.NewAcceptUnique(kapi.Scheme), app.AcceptNew}
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
	containers := make(map[string]*kapi.Container)
	var services []*kapi.Service
	for _, obj := range objects {
		switch t := obj.(type) {
		case *deployapi.DeploymentConfig:
			ports := app.UniqueContainerToServicePorts(app.AllContainerPorts(t.Spec.Template.Spec.Containers...))
			if len(ports) == 0 {
				msg := "no ports defined to send traffic to - no OpenShift service was created"
				warnings[msg] = append(warnings[msg], t.Name)
				continue
			}
			svc := app.GenerateService(t.ObjectMeta, t.Spec.Selector)
			if aliases[svc.Name].Len() == 1 {
				svc.Name = aliases[svc.Name].List()[0]
			}
			svc.Spec.Ports = ports
			services = append(services, svc)

			// take a reference to each container
			for i := range t.Spec.Template.Spec.Containers {
				c := &t.Spec.Template.Spec.Containers[i]
				containers[c.Name] = c
			}
		}
	}
	for _, svc := range services {
		objects = append(objects, svc)
	}

	// for each container that defines VolumesFrom, copy equivalent mounts.
	// TODO: ensure mount names are unique?
	for target, otherContainers := range volumesFrom {
		for _, from := range otherContainers {
			for _, volume := range containers[from].VolumeMounts {
				containers[target].VolumeMounts = append(containers[target].VolumeMounts, volume)
			}
		}
	}

	template.Objects = objects

	// generate warnings
	if len(warnings) > 0 {
		allWarnings := sets.NewString()
		for msg, services := range warnings {
			allWarnings.Insert(fmt.Sprintf("%s: %s", strings.Join(services, ","), msg))
		}
		if template.Annotations == nil {
			template.Annotations = make(map[string]string)
		}
		template.Annotations[app.GenerationWarningAnnotation] = fmt.Sprintf("not all docker-compose fields were honored:\n* %s", strings.Join(allWarnings.List(), "\n* "))
	}

	return template, nil
}

// extractFirstPorts converts a Docker compose port spec (CONTAINER, HOST:CONTAINER, or
// IP:HOST:CONTAINER) to the first container and host port in the range.  Host port will
// default to container port.
func extractFirstPorts(port string) (container, host string) {
	segments := strings.Split(port, ":")
	container = segments[len(segments)-1]
	container = rangeToPort(container)
	switch {
	case len(segments) == 3:
		host = rangeToPort(segments[1])
	case len(segments) == 2 && net.ParseIP(segments[0]) == nil:
		host = rangeToPort(segments[0])
	default:
		host = container
	}
	return container, host
}

func rangeToPort(s string) string {
	parts := strings.SplitN(s, "-", 2)
	return parts[0]
}

// warnUnusableComposeElements add warnings for unsupported elements in the provided service config
func warnUnusableComposeElements(k string, v *project.ServiceConfig, warnings map[string][]string) {
	fn := func(msg string) {
		warnings[msg] = append(warnings[msg], k)
	}
	if len(v.CapAdd) > 0 || len(v.CapDrop) > 0 {
		// TODO: we can support this
		fn("cap_add and cap_drop are not supported")
	}
	if len(v.CgroupParent) > 0 {
		fn("cgroup_parent is not supported")
	}
	if len(v.CPUSet) > 0 {
		fn("cpuset is not supported")
	}
	if len(v.Devices) > 0 {
		fn("devices are not supported")
	}
	if v.DNS.Len() > 0 || v.DNSSearch.Len() > 0 {
		fn("dns and dns_search are not supported")
	}
	if len(v.DomainName) > 0 {
		fn("domainname is not supported")
	}
	if len(v.Hostname) > 0 {
		fn("hostname is not supported")
	}
	if len(v.Labels.MapParts()) > 0 {
		fn("labels is ignored")
	}
	if len(v.Links.Slice()) > 0 {
		//fn("links are not supported, use services to talk to other pods")
		// TODO: display some sort of warning when linking will be inconsistent
	}
	if len(v.LogDriver) > 0 {
		fn("log_driver is not supported")
	}
	if len(v.MacAddress) > 0 {
		fn("mac_address is not supported")
	}
	if len(v.Net) > 0 {
		fn("net is not supported")
	}
	if len(v.Pid) > 0 {
		fn("pid is not supported")
	}
	if len(v.Uts) > 0 {
		fn("uts is not supported")
	}
	if len(v.Ipc) > 0 {
		fn("ipc is not supported")
	}
	if v.MemSwapLimit > 0 {
		fn("mem_swap_limit is not supported")
	}
	if len(v.Restart) > 0 {
		fn("restart is ignored - all pods are automatically restarted")
	}
	if len(v.SecurityOpt) > 0 {
		fn("security_opt is not supported")
	}
	if len(v.User) > 0 {
		if _, err := strconv.Atoi(v.User); err != nil {
			fn("setting user to a string is not supported - use numeric user value")
		}
	}
	if len(v.VolumeDriver) > 0 {
		fn("volume_driver is not supported")
	}
	if len(v.VolumesFrom) > 0 {
		fn("volumes_from is not supported")
		// TODO: use volumes from for colocated containers to automount volumes
	}
	if len(v.ExternalLinks) > 0 {
		fn("external_links are not supported - use services")
	}
	if len(v.LogOpt) > 0 {
		fn("log_opt is not supported")
	}
	if len(v.ExtraHosts) > 0 {
		fn("extra_hosts is not supported")
	}
	if len(v.Ulimits.Elements) > 0 {
		fn("ulimits is not supported")
	}
	// TODO: fields to handle
	// EnvFile       Stringorslice     `yaml:"env_file,omitempty"`
}
