package instantiate

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/api/validation"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func NewREST(store registry.Store, oc client.Interface, kc kclient.Interface, decoder runtime.Decoder, admission admission.Interface) *REST {
	store.UpdateStrategy = Strategy
	return &REST{store: &store, isn: oc, rn: kc, decoder: decoder, admit: admission}
}

// REST implements the Creater interface.
var _ = rest.Creater(&REST{})

type REST struct {
	store   *registry.Store
	isn     client.ImageStreamsNamespacer
	rn      kclient.ReplicationControllersNamespacer
	decoder runtime.Decoder
	admit   admission.Interface
}

func (s *REST) New() runtime.Object {
	return &deployapi.DeploymentRequest{}
}

// Create instantiates a deployment config
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	req, ok := obj.(*deployapi.DeploymentRequest)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("wrong object passed for requesting a new rollout: %#v", obj))
	}

	configObj, err := r.store.Get(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	config := configObj.(*deployapi.DeploymentConfig)
	old := config

	if errs := validation.ValidateRequestForDeploymentConfig(req, config); len(errs) > 0 {
		return nil, errors.NewInvalid(deployapi.Kind("DeploymentRequest"), req.Name, errs)
	}

	// We need to process the deployment config before we can determine if it is possible to trigger
	// a deployment.
	if req.Latest {
		if err := processTriggers(config, r.isn, req.Force); err != nil {
			return nil, err
		}
	}

	canTrigger, causes, err := canTrigger(config, r.rn, r.decoder, req.Force)
	if err != nil {
		return nil, err
	}
	// If we cannot trigger then there is nothing to do here.
	if !canTrigger {
		return &unversioned.Status{
			Message: fmt.Sprintf("deployment config %q cannot be instantiated", config.Name),
			Code:    int32(204),
		}, nil
	}
	glog.V(4).Infof("New deployment for %q caused by %#v", config.Name, causes)

	config.Status.Details = new(deployapi.DeploymentDetails)
	config.Status.Details.Causes = causes
	switch causes[0].Type {
	case deployapi.DeploymentTriggerOnConfigChange:
		config.Status.Details.Message = "config change"
	case deployapi.DeploymentTriggerOnImageChange:
		config.Status.Details.Message = "image change"
	case deployapi.DeploymentTriggerManual:
		config.Status.Details.Message = "manual change"
	}
	config.Status.LatestVersion++

	userInfo, _ := kapi.UserFrom(ctx)
	attrs := admission.NewAttributesRecord(config, old, deployapi.Kind("DeploymentConfig").WithVersion(""), config.Namespace, config.Name, deployapi.Resource("DeploymentConfig").WithVersion(""), "", admission.Update, userInfo)
	if err := r.admit.Admit(attrs); err != nil {
		return nil, err
	}

	updated, _, err := r.store.Update(ctx, config.Name, rest.DefaultUpdatedObjectInfo(config, kapi.Scheme))
	return updated, err
}

// processTriggers will go over all deployment triggers that require processing and update
// the deployment config accordingly. This contains the work that the image change controller
// had been doing up to the point we got the /instantiate endpoint.
func processTriggers(config *deployapi.DeploymentConfig, isn client.ImageStreamsNamespacer, force bool) error {
	errs := []error{}

	// Process any image change triggers.
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		params := trigger.ImageChangeParams

		// Forced deployments should always try to resolve the images in the template.
		// On the other hand, paused deployments or non-automatic triggers shouldn't.
		if !force && (config.Spec.Paused || !params.Automatic) {
			continue
		}

		// Tag references are already validated
		name, tag, _ := imageapi.SplitImageStreamTag(params.From.Name)
		stream, err := isn.ImageStreams(params.From.Namespace).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				errs = append(errs, err)
			}
			continue
		}

		// Find the latest tag event for the trigger reference.
		latestEvent := imageapi.LatestTaggedImage(stream, tag)
		if latestEvent == nil {
			continue
		}

		// Ensure a change occurred
		latestRef := latestEvent.DockerImageReference
		if len(latestRef) == 0 || latestRef == params.LastTriggeredImage {
			continue
		}

		// Update containers
		names := sets.NewString(params.ContainerNames...)
		for i := range config.Spec.Template.Spec.Containers {
			container := &config.Spec.Template.Spec.Containers[i]
			if !names.Has(container.Name) {
				continue
			}

			if container.Image != latestRef {
				// Update the image
				container.Image = latestRef
				// Log the last triggered image ID
				params.LastTriggeredImage = latestRef
			}
		}
	}

	if err := utilerrors.NewAggregate(errs); err != nil {
		return errors.NewInternalError(err)
	}

	return nil
}

// canTrigger determines if we can trigger a new deployment for config based on the various deployment triggers.
func canTrigger(
	config *deployapi.DeploymentConfig,
	rn kclient.ReplicationControllersNamespacer,
	decoder runtime.Decoder,
	force bool,
) (bool, []deployapi.DeploymentCause, error) {

	decoded, err := decodeFromLatestDeployment(config, rn, decoder)
	if err != nil {
		return false, nil, err
	}

	ictCount, resolved, canTriggerByImageChange := 0, 0, false
	var causes []deployapi.DeploymentCause

	for _, t := range config.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}
		ictCount++

		// If the image is yet to be resolved then we cannot process this trigger.
		lastTriggered := t.ImageChangeParams.LastTriggeredImage
		if len(lastTriggered) == 0 {
			continue
		}
		resolved++

		// Non-automatic triggers should not be able to trigger deployments.
		if !t.ImageChangeParams.Automatic {
			continue
		}

		// We need stronger checks in order to validate that this template
		// change is an image change. Look at the deserialized config's
		// triggers and compare with the present trigger. Initial deployments
		// should always trigger - there is no previous config to use for the
		// comparison.
		if config.Status.LatestVersion > 0 && !triggeredByDifferentImage(*t.ImageChangeParams, *decoded) {
			continue
		}

		canTriggerByImageChange = true
		causes = append(causes, deployapi.DeploymentCause{
			Type: deployapi.DeploymentTriggerOnImageChange,
			ImageTrigger: &deployapi.DeploymentCauseImageTrigger{
				From: kapi.ObjectReference{
					Name:      t.ImageChangeParams.From.Name,
					Namespace: t.ImageChangeParams.From.Namespace,
					Kind:      "ImageStreamTag",
				},
			},
		})
	}

	if ictCount != resolved {
		err = errors.NewBadRequest(fmt.Sprintf("cannot trigger a deployment for %q because it contains unresolved images", config.Name))
		return false, nil, err
	}

	if force {
		return true, []deployapi.DeploymentCause{{Type: deployapi.DeploymentTriggerManual}}, nil
	}

	canTriggerByConfigChange := false
	if deployutil.HasChangeTrigger(config) && // Our deployment config has a config change trigger
		len(causes) == 0 && // and no other trigger has triggered.
		(config.Status.LatestVersion == 0 || // Either it's the initial deployment
			!kapi.Semantic.DeepEqual(config.Spec.Template, decoded.Spec.Template)) /* or a config change happened so we need to trigger */ {

		canTriggerByConfigChange = true
		causes = []deployapi.DeploymentCause{{Type: deployapi.DeploymentTriggerOnConfigChange}}
	}

	return canTriggerByConfigChange || canTriggerByImageChange, causes, nil
}

// decodeFromLatestDeployment will try to return the decoded version of the current deploymentconfig
// found in the annotations of its latest deployment. If there is no previous deploymentconfig (ie.
// latestVersion == 0), the returned deploymentconfig will be the same.
func decodeFromLatestDeployment(config *deployapi.DeploymentConfig, rn kclient.ReplicationControllersNamespacer, decoder runtime.Decoder) (*deployapi.DeploymentConfig, error) {
	if config.Status.LatestVersion == 0 {
		return config, nil
	}

	latestDeploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := rn.ReplicationControllers(config.Namespace).Get(latestDeploymentName)
	if err != nil {
		// If there's no deployment for the latest config, we have no basis of
		// comparison. It's the responsibility of the deployment config controller
		// to make the deployment for the config, so return early.
		return nil, err
	}
	decoded, err := deployutil.DecodeDeploymentConfig(deployment, decoder)
	if err != nil {
		return nil, errors.NewInternalError(err)
	}
	return decoded, nil
}

// triggeredByDifferentImage compares the provided image change parameters with those found in the
// previous deployment config (the one we decoded from the annotations of its latest deployment)
// and returns whether the two deployment configs have been triggered by a different image change.
func triggeredByDifferentImage(ictParams deployapi.DeploymentTriggerImageChangeParams, previous deployapi.DeploymentConfig) bool {
	for _, t := range previous.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		if t.ImageChangeParams.From.Name != ictParams.From.Name ||
			t.ImageChangeParams.From.Namespace != ictParams.From.Namespace {
			continue
		}

		if t.ImageChangeParams.LastTriggeredImage != ictParams.LastTriggeredImage {
			glog.V(4).Infof("Deployment config %q triggered by different image: %s -> %s", previous.Name, t.ImageChangeParams.LastTriggeredImage, ictParams.LastTriggeredImage)
			return true
		}
		return false
	}
	return false
}
