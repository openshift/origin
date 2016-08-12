package imagepolicy

import (
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"
	lru "github.com/hashicorp/golang-lru"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api/validation"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/rules"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	admission.RegisterPlugin(api.PluginName, func(client clientset.Interface, input io.Reader) (admission.Interface, error) {
		obj, err := configlatest.ReadYAML(input)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, nil
		}
		config, ok := obj.(*api.ImagePolicyConfig)
		if !ok {
			return nil, fmt.Errorf("unexpected config object: %#v", obj)
		}
		if errs := validation.Validate(config); len(errs) > 0 {
			return nil, errs.ToAggregate()
		}
		glog.V(5).Infof("%s admission controller loaded with config: %#v", api.PluginName, config)
		return newImagePolicyPlugin(client, config)
	})
}

type imagePolicyPlugin struct {
	*admission.Handler
	config *api.ImagePolicyConfig
	client client.Interface

	accepter rules.Accepter
	adjuster rules.Adjuster

	integratedRegistryMatcher integratedRegistryMatcher

	resolveGroupResources []unversioned.GroupResource

	resolver imageResolver
}

// TODO: add wants the image registry resolver
var _ = oadmission.WantsOpenshiftClient(&imagePolicyPlugin{})
var _ = oadmission.Validator(&imagePolicyPlugin{})

type integratedRegistryMatcher struct {
	rules.RegistryMatcher
}

type imageResolver interface {
	Resolve(ref imageapi.DockerImageReference) (*imageapi.Image, error)
}

// imagePolicyPlugin returns an admission controller for pods that controls what images are allowed to run on the
// cluster.
func newImagePolicyPlugin(client clientset.Interface, parsed *api.ImagePolicyConfig) (*imagePolicyPlugin, error) {
	m := integratedRegistryMatcher{
		RegistryMatcher: rules.NewRegistryMatcher(nil),
	}
	accepter := rules.NewExecutionRulesAccepter(parsed.ExecutionRules, m)
	resourceAdjuster := rules.NewConsumptionRulesAdjuster(parsed.ConsumptionRules, m)
	placementAdjuster := rules.NewPlacementRulesAdjuster(parsed.PlacementRules, m)

	return &imagePolicyPlugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		config:  parsed,

		accepter: accepter,
		adjuster: rules.Adjusters{resourceAdjuster, placementAdjuster},

		integratedRegistryMatcher: m,
	}, nil
}

func (a *imagePolicyPlugin) SetDefaultRegistryFunc(fn imageapi.DefaultRegistryFunc) {
	a.integratedRegistryMatcher.RegistryMatcher = rules.RegistryNameMatcher(fn)
}

func (a *imagePolicyPlugin) SetOpenshiftClient(c client.Interface) {
	a.client = c
}

// Validate ensures that all required interfaces have been provided, or returns an error.
func (a *imagePolicyPlugin) Validate() error {
	if a.client == nil {
		return fmt.Errorf("%s needs an Openshift client", api.PluginName)
	}
	imageResolver, err := newImageResolutionCache(a.client.Images(), a.client, a.integratedRegistryMatcher)
	if err != nil {
		return fmt.Errorf("unable to create image policy controller: %v", err)
	}
	a.resolver = imageResolver
	return nil
}

// Admit attempts to apply the image policy to the incoming resource.
func (a *imagePolicyPlugin) Admit(attr admission.Attributes) error {
	switch attr.GetOperation() {
	case admission.Create, admission.Update:
		if len(attr.GetSubresource()) > 0 {
			return nil
		}
		// only create and update are tested, and only on core resources
		// TODO: scan all resources
		// TODO: Create a general equivalence map for admission - operation X on subresource Y is equivalent to reduced operation
	default:
		return nil
	}

	gr := attr.GetResource().GroupResource()
	accept, adjust := a.accepter.Covers(gr), a.adjuster.Covers(gr)
	if !accept && !adjust {
		return nil
	}

	podSpec, basePath, err := meta.GetPodSpec(attr.GetObject())
	if err != nil {
		return apierrs.NewForbidden(gr, attr.GetName(), fmt.Errorf("unable to apply image policy against objects of type %T, because they do not have a pod spec", attr.GetObject()))
	}

	covered := make(policyAttributes)

	if accept {
		if err := a.accept(podSpec, basePath, attr, covered); err != nil {
			return err
		}
	}
	if adjust {
		if err := a.adjust(podSpec, attr, covered); err != nil {
			return err
		}
	}

	return nil
}

func (a *imagePolicyPlugin) accept(podSpec *kapi.PodSpec, basePath *field.Path, attr admission.Attributes, covered policyAttributes) error {
	gr := attr.GetResource().GroupResource()
	requiresImage := a.accepter.RequiresImage(gr)
	resolvesImage := a.accepter.ResolvesImage(gr)

	var errs field.ErrorList
	path := basePath.Child("containers")
	for i := range podSpec.Containers {
		if ok, err := a.acceptContainer(&podSpec.Containers[i], gr, covered, requiresImage, resolvesImage); !ok {
			if err != nil {
				errs = append(errs, field.Forbidden(path.Index(i).Child("image"), fmt.Sprintf("this image is prohibited by policy: %v", err.Error())))
				continue
			}
			errs = append(errs, field.Forbidden(path.Index(i).Child("image"), "this image is prohibited by policy"))
		}
	}
	path = basePath.Child("initContainers")
	for i := range podSpec.InitContainers {
		if ok, err := a.acceptContainer(&podSpec.InitContainers[i], gr, covered, requiresImage, resolvesImage); !ok {
			if err != nil {
				errs = append(errs, field.Forbidden(path.Index(i).Child("image"), fmt.Sprintf("this image is prohibited by policy: %v", err.Error())))
				continue
			}
			errs = append(errs, field.Forbidden(path.Index(i).Child("image"), "this image is prohibited by policy"))
		}
	}
	if len(errs) > 0 {
		glog.V(5).Infof("failed to create: %v", errs)
		return apierrs.NewInvalid(attr.GetKind().GroupKind(), attr.GetName(), errs)
	}
	glog.V(5).Infof("allowed: %#v", attr)
	return nil
}

func (a *imagePolicyPlugin) acceptContainer(container *kapi.Container, gr unversioned.GroupResource, covered policyAttributes, requiresImage, resolvesImage bool) (bool, error) {
	name := container.Image

	// only resolve the image once per pod
	if covered.Has(name) {
		return true, nil
	}
	// explicitly allow empty strings to bypass checks
	if len(name) == 0 {
		return true, nil
	}

	ref, err := imageapi.ParseDockerImageReference(name)
	if err != nil {
		return false, err
	}

	// check the decision cache if we've already made a decision on the image
	// TODO: time bound the decision cache so that namespace overrides can take effect
	isDigest := len(ref.ID) > 0

	var image *imageapi.Image
	if requiresImage || (!isDigest && resolvesImage) {
		image, err = a.resolver.Resolve(ref)
		switch {
		case apierrs.IsNotFound(err) && !resolvesImage:
			// if the referenced image does not exist, and resolution is not required, we'll let
			// the policy rule decide whether to fail
		case err != nil:
			return false, err
		default:
			glog.V(5).Infof("Resolved image %v to %s (%s)", ref, image.Name, image.DockerImageReference)
			// resolve the reference to a digest, rather than a tag
			ref.Tag = ""
			ref.ID = image.Name

			// if we should resolve the image, update the image and also check the cache again
			if resolvesImage {
				container.Image = ref.Exact()
				isDigest = true
			}
		}
	}

	attr := &rules.ImagePolicyAttributes{
		Name:         ref,
		OriginalName: name,
		Resource:     gr,
		Image:        image,
	}
	// remember under both names
	covered.Remember(name, attr)
	covered.Remember(container.Image, attr)

	accepted := a.accepter.Accepts(attr)

	glog.V(5).Infof("Made decision for %v: %t", ref, accepted)
	return accepted, nil
}

func (a *imagePolicyPlugin) adjust(podSpec *kapi.PodSpec, attr admission.Attributes, covered policyAttributes) error {
	gr := attr.GetResource().GroupResource()
	requiresImage := a.adjuster.RequiresImage(gr)

	for i := range podSpec.Containers {
		if err := a.adjustContainer(podSpec, &podSpec.Containers[i], gr, covered, requiresImage); err != nil {
			return err
		}
	}
	for i := range podSpec.InitContainers {
		if err := a.adjustContainer(podSpec, &podSpec.InitContainers[i], gr, covered, requiresImage); err != nil {
			return err
		}
	}
	return nil
}

func (a *imagePolicyPlugin) adjustContainer(podSpec *kapi.PodSpec, container *kapi.Container, gr unversioned.GroupResource, covered policyAttributes, requiresImage bool) error {
	name := container.Image

	// ignore containers without images
	if len(name) == 0 {
		return nil
	}

	// resolve the image if we have not done so - this logic is simpler than accept because we only need
	// the image
	attr, ok := covered.Get(name)
	if !ok {
		ref, err := imageapi.ParseDockerImageReference(name)
		if err != nil {
			return err
		}

		var image *imageapi.Image
		if requiresImage {
			image, err = a.resolver.Resolve(ref)
			if err != nil {
				return err
			}
			glog.V(5).Infof("Resolved image %v to %s (%s)", ref, image.Name, image.DockerImageReference)
		}

		attr = &rules.ImagePolicyAttributes{
			Name:         ref,
			OriginalName: name,
			Resource:     gr,
			Image:        image,
		}
		covered.Remember(name, attr)
	}

	a.adjuster.Adjust(attr, podSpec)
	return nil
}

type policyAttributes map[string]*rules.ImagePolicyAttributes

func (d policyAttributes) Remember(image string, attr *rules.ImagePolicyAttributes) {
	d[image] = attr
}

func (d policyAttributes) Has(image string) bool {
	_, ok := d[image]
	return ok
}

func (d policyAttributes) Get(image string) (*rules.ImagePolicyAttributes, bool) {
	attr, ok := d[image]
	return attr, ok
}

type imageResolutionCache struct {
	images     client.ImageInterface
	tags       client.ImageStreamTagsNamespacer
	integrated rules.RegistryMatcher
	expiration time.Duration

	cache *lru.Cache
}

type imageCacheEntry struct {
	expires time.Time
	image   *imageapi.Image
}

func newImageResolutionCache(images client.ImageInterface, tags client.ImageStreamTagsNamespacer, integratedRegistry rules.RegistryMatcher) (*imageResolutionCache, error) {
	imageCache, err := lru.New(128)
	if err != nil {
		return nil, err
	}
	return &imageResolutionCache{
		images:     images,
		tags:       tags,
		integrated: integratedRegistry,
		cache:      imageCache,
		expiration: time.Minute,
	}, nil
}

var errNotRegistryImage = fmt.Errorf("only images imported into the registry can be used")

var now = time.Now

func (c *imageResolutionCache) Resolve(ref imageapi.DockerImageReference) (*imageapi.Image, error) {
	// images by ID can be checked for policy
	if len(ref.ID) > 0 {
		now := now()
		if value, ok := c.cache.Get(ref.ID); ok {
			cached := value.(imageCacheEntry)
			if now.Before(cached.expires) {
				return cached.image, nil
			}
		}
		image, err := c.images.Get(ref.ID)
		if err != nil {
			return nil, err
		}
		c.cache.Add(ref.ID, imageCacheEntry{expires: now.Add(c.expiration), image: image})
		return image, nil
	}

	if !c.integrated.Matches(ref.Registry) {
		return nil, errNotRegistryImage
	}

	tag := ref.Tag
	if len(tag) == 0 {
		tag = imageapi.DefaultImageTag
	}
	resolved, err := c.tags.ImageStreamTags(ref.Namespace).Get(ref.Name, tag)
	if err != nil {
		fmt.Printf("failed to resolve %s: %v\n", ref, err)
		return nil, err
	}
	fmt.Printf("resolved %s: %#v\n", ref, resolved)
	now := now()
	c.cache.Add(resolved.Image.Name, imageCacheEntry{expires: now.Add(c.expiration), image: &resolved.Image})
	return &resolved.Image, nil
}
