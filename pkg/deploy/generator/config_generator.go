package generator

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DeploymentConfigGenerator reconciles a DeploymentConfig with other pieces of deployment-related state
// and produces a DeploymentConfig which represents a potential future DeploymentConfig. If the generated
// state differs from the input state, the LatestVersion field of the output is incremented.
type DeploymentConfigGenerator struct {
	Client GeneratorClient
	Codec  runtime.Codec
}

type GeneratorClient interface {
	GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error)
	GetImageRepository(ctx kapi.Context, name string) (*imageapi.ImageRepository, error)
	// LEGACY: used, to scan all repositories for a DockerImageReference.  Will be removed
	// when we drop support for reference by DockerImageReference.
	ListImageRepositories(ctx kapi.Context) (*imageapi.ImageRepositoryList, error)
}

type Client struct {
	DCFn   func(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error)
	IRFn   func(ctx kapi.Context, name string) (*imageapi.ImageRepository, error)
	LIRFn  func(ctx kapi.Context) (*imageapi.ImageRepositoryList, error)
	LIRFn2 func(ctx kapi.Context, label labels.Selector) (*imageapi.ImageRepositoryList, error)
}

func (c Client) GetDeploymentConfig(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
	return c.DCFn(ctx, name)
}
func (c Client) GetImageRepository(ctx kapi.Context, name string) (*imageapi.ImageRepository, error) {
	return c.IRFn(ctx, name)
}
func (c Client) ListImageRepositories(ctx kapi.Context) (*imageapi.ImageRepositoryList, error) {
	if c.LIRFn2 != nil {
		return c.LIRFn2(ctx, labels.Everything())
	}
	return c.LIRFn(ctx)
}

// Generate returns a potential future DeploymentConfig based on the DeploymentConfig specified
// by namespace and name. Returns a RESTful error.
func (g *DeploymentConfigGenerator) Generate(ctx kapi.Context, name string) (*deployapi.DeploymentConfig, error) {
	dc, err := g.Client.GetDeploymentConfig(ctx, name)
	if err != nil {
		return nil, err
	}

	refs, legacy := findReferences(dc)
	if errs := retrieveReferences(g.Client, ctx, refs, legacy); len(errs) > 0 {
		return nil, errors.NewInvalid("DeploymentConfig", name, errs)
	}
	indexed := referencesByIndex(refs, legacy)
	changed, errs := replaceReferences(dc, indexed)
	if len(errs) > 0 {
		return nil, errors.NewInvalid("DeploymentConfig", name, errs)
	}
	if changed || dc.LatestVersion == 0 {
		dc.LatestVersion++
	}

	return dc, nil
}

type refKey struct {
	namespace string
	name      string
}

type triggerEntry struct {
	positions []int
	field     string
	repo      *imageapi.ImageRepository
}

type triggersByRef map[refKey]*triggerEntry
type triggersByName map[string]*triggerEntry

// findReferences looks up triggers with references and maps them back to their position in the trigger array.
func findReferences(dc *deployapi.DeploymentConfig) (refs triggersByRef, legacy triggersByName) {
	refs, legacy = make(triggersByRef), make(triggersByName)

	for i := range dc.Triggers {
		trigger := &dc.Triggers[i]
		if trigger.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		// use the object reference to find the image repository
		if from := &trigger.ImageChangeParams.From; len(from.Name) != 0 {
			k := refKey{
				namespace: from.Namespace,
				name:      from.Name,
			}
			if len(k.namespace) == 0 {
				k.namespace = dc.Namespace
			}
			trigger, ok := refs[k]
			if !ok {
				trigger = &triggerEntry{
					field: fmt.Sprintf("triggers[%d].imageChange.from", i),
				}
				refs[k] = trigger
			}
			trigger.positions = append(trigger.positions, i)
			continue
		}

		// use the old way of looking up the name
		// DEPRECATED: this path will be removed soon
		if k := trigger.ImageChangeParams.RepositoryName; len(k) != 0 {
			trigger, ok := legacy[k]
			if !ok {
				trigger = &triggerEntry{
					field: fmt.Sprintf("triggers[%d].imageChange.repositoryName", i),
				}
				legacy[k] = trigger
			}
			trigger.positions = append(trigger.positions, i)
			continue
		}
	}
	return
}

// retrieveReferences loads the repositories referenced by a deployment config
func retrieveReferences(client GeneratorClient, ctx kapi.Context, refs triggersByRef, legacy triggersByName) errors.ValidationErrorList {
	errs := errors.ValidationErrorList{}

	// fetch repositories directly
	for k, v := range refs {
		repo, err := client.GetImageRepository(kapi.WithNamespace(ctx, k.namespace), k.name)
		if err != nil {
			if errors.IsNotFound(err) {
				errs = append(errs, errors.NewFieldNotFound(v.field, k.name))
			} else {
				errs = append(errs, errors.NewFieldInvalid(v.field, k.name, err.Error()))
			}
			continue
		}
		v.repo = repo
	}

	// look for legacy references that we've already loaded
	// DEPRECATED: remove all code below this line when the reference logic is removed
	missing := make(triggersByName)
	for k, v := range legacy {
		for _, ref := range refs {
			if ref.repo.Status.DockerImageRepository == k {
				v.repo = ref.repo
				break
			}
		}
		if v.repo == nil {
			missing[k] = v
		}
	}

	// if we haven't loaded the references, do the more expensive list all
	if len(missing) != 0 {
		repos, err := client.ListImageRepositories(ctx)
		if err != nil {
			for k, ref := range missing {
				errs = append(errs, errors.NewFieldInvalid(ref.field, k, err.Error()))
			}
			return errs
		}

		for k, ref := range missing {
			for i := range repos.Items {
				repo := &repos.Items[i]
				if repo.DockerImageRepository == k {
					ref.repo = repo
					break
				}
			}
			if ref.repo == nil {
				errs = append(errs, errors.NewFieldNotFound(ref.field, k))
			}
		}
	}
	return errs
}

type reposByIndex map[int]*imageapi.ImageRepository

func referencesByIndex(refs triggersByRef, legacy triggersByName) reposByIndex {
	repos := make(reposByIndex)
	for _, v := range refs {
		for _, i := range v.positions {
			repos[i] = v.repo
		}
	}
	for _, v := range legacy {
		for _, i := range v.positions {
			repos[i] = v.repo
		}
	}
	return repos
}

func replaceReferences(dc *deployapi.DeploymentConfig, repos reposByIndex) (changed bool, errs errors.ValidationErrorList) {
	template := dc.Template.ControllerTemplate.Template
	for i, repo := range repos {
		if len(repo.Status.DockerImageRepository) == 0 {
			errs = append(errs, errors.NewFieldInvalid(fmt.Sprintf("triggers[%d].imageChange.from", i), repo.Name, fmt.Sprintf("image repository %s/%s does not have a Docker image repository reference set and can't be used in a deployment config trigger", repo.Namespace, repo.Name)))
			continue
		}
		params := dc.Triggers[i].ImageChangeParams

		// get the image ref from the repo's tag history
		latest, err := imageapi.LatestTaggedImage(repo, params.Tag)
		if err != nil {
			errs = append(errs, errors.NewFieldInvalid(fmt.Sprintf("triggers[%d].imageChange.from", i), repo.Name, err.Error()))
			continue
		}
		image := latest.DockerImageReference

		// update containers
		names := util.NewStringSet(params.ContainerNames...)
		for i := range template.Spec.Containers {
			container := &template.Spec.Containers[i]
			if !names.Has(container.Name) {
				continue
			}
			if container.Image != image {
				container.Image = image
				changed = true
			}
		}
	}
	return
}
