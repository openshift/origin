package deploymentconfig

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DeploymentConfigController is responsible for creating a new deployment when:
//
//    1. The config version is > 0 and,
//    2. No existing deployment for that version exists.
//
// The responsibility of constructing a new deployment resource from a config
// is delegated. See util.MakeDeployment for more details. The new deployment
// will have DesiredReplicasAnnotation set to the desired replica count for
// the new deployment based on the replica count of the previous/active
// deployment.
//
// Use the DeploymentConfigControllerFactory to create this controller.
type DeploymentConfigController struct {
	// deploymentClient provides access to deployments.
	deploymentClient deploymentClient
	// osClient provides access to Origin resources
	osClient osclient.Interface
	// makeDeployment knows how to make a deployment from a config.
	makeDeployment func(*deployapi.DeploymentConfig) (*kapi.ReplicationController, error)
}

// fatalError is an error which can't be retried.
type fatalError string

// transientError is an error which should always be retried (indefinitely).
type transientError string

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling deployment config: %s", string(e))
}
func (e transientError) Error() string {
	return "transient error handling deployment config: " + string(e)
}

// Handle processes config and creates a new deployment if necessary.
func (c *DeploymentConfigController) Handle(config *deployapi.DeploymentConfig) error {
	// Inspect a deployment configuration every time the controller reconciles it
	details, existingDeployments, latestDeploymentExists, err := c.findDetails(config)
	if err != nil {
		return err
	}
	config, err = c.updateDetails(config, details)
	if err != nil {
		return transientError(err.Error())
	}

	// Only deploy when the version has advanced past 0.
	if config.LatestVersion == 0 {
		glog.V(5).Infof("Waiting for first version of %s", deployutil.LabelForDeploymentConfig(config))
		return nil
	}

	var inflightDeployment *kapi.ReplicationController
	for _, deployment := range existingDeployments.Items {

		deploymentStatus := deployutil.DeploymentStatusFor(&deployment)
		switch deploymentStatus {
		case deployapi.DeploymentStatusFailed,
			deployapi.DeploymentStatusComplete:
			// Previous deployment in terminal state - can ignore
			// Ignoring specific deployment states so that any newly introduced
			// deployment state will not be ignored
		default:
			if inflightDeployment == nil {
				inflightDeployment = &deployment
				continue
			}
			var deploymentForCancellation *kapi.ReplicationController
			if deployutil.DeploymentVersionFor(inflightDeployment) < deployutil.DeploymentVersionFor(&deployment) {
				deploymentForCancellation, inflightDeployment = inflightDeployment, &deployment
			} else {
				deploymentForCancellation = &deployment
			}

			deploymentForCancellation.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
			deploymentForCancellation.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledNewerDeploymentExists
			if _, err := c.deploymentClient.updateDeployment(deploymentForCancellation.Namespace, deploymentForCancellation); err != nil {
				util.HandleError(fmt.Errorf("couldn't cancel deployment %s: %v", deployutil.LabelForDeployment(deploymentForCancellation), err))
			}
			glog.V(4).Infof("Cancelled deployment %s for deployment config %s", deployutil.LabelForDeployment(deploymentForCancellation), deployutil.LabelForDeploymentConfig(config))
		}
	}

	// if the latest deployment exists then nothing else needs to be done
	if latestDeploymentExists {
		return nil
	}

	// check to see if there are inflight deployments
	if inflightDeployment != nil {
		// raise a transientError so that the deployment config can be re-queued
		glog.V(4).Infof("Found previous inflight deployment for %s - will requeue", deployutil.LabelForDeploymentConfig(config))
		return transientError(fmt.Sprintf("found previous inflight deployment for %s - requeuing", deployutil.LabelForDeploymentConfig(config)))
	}

	// Try and build a deployment for the config.
	deployment, err := c.makeDeployment(config)
	if err != nil {
		return fatalError(fmt.Sprintf("couldn't make deployment from (potentially invalid) deployment config %s: %v", deployutil.LabelForDeploymentConfig(config), err))
	}

	// Compute the desired replicas for the deployment. Use the last completed
	// deployment's current replica count, or the config template if there is no
	// prior completed deployment available.
	desiredReplicas := config.Template.ControllerTemplate.Replicas
	if len(existingDeployments.Items) > 0 {
		sort.Sort(deployutil.ByLatestVersionDesc(existingDeployments.Items))
		for _, existing := range existingDeployments.Items {
			if deployutil.DeploymentStatusFor(&existing) == deployapi.DeploymentStatusComplete {
				desiredReplicas = existing.Spec.Replicas
				glog.V(4).Infof("Desired replicas for %s set to %d based on prior completed deployment %s", deployutil.LabelForDeploymentConfig(config), desiredReplicas, existing.Name)
				break
			}
		}
	}
	deployment.Annotations[deployapi.DesiredReplicasAnnotation] = strconv.Itoa(desiredReplicas)

	// Create the deployment.
	if _, err := c.deploymentClient.createDeployment(config.Namespace, deployment); err == nil {
		glog.V(4).Infof("Created deployment for deployment config %s", deployutil.LabelForDeploymentConfig(config))
		return nil
	} else {
		// If the deployment was already created, just move on. The cache could be stale, or another
		// process could have already handled this update.
		if errors.IsAlreadyExists(err) {
			glog.V(4).Infof("Deployment already exists for deployment config %s", deployutil.LabelForDeploymentConfig(config))
			return nil
		}

		glog.Warningf("Cannot create latest deployment for deployment config %q: %v", deployutil.LabelForDeploymentConfig(config), err)
		return fmt.Errorf("couldn't create deployment for deployment config %s: %v", deployutil.LabelForDeploymentConfig(config), err)
	}
}

// deploymentClient abstracts access to deployments.
type deploymentClient interface {
	createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
	// listDeploymentsForConfig should return deployments associated with the
	// provided config.
	listDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error)
	updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// deploymentClientImpl is a pluggable deploymentClient.
type deploymentClientImpl struct {
	createDeploymentFunc         func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
	listDeploymentsForConfigFunc func(namespace, configName string) (*kapi.ReplicationControllerList, error)
	updateDeploymentFunc         func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (i *deploymentClientImpl) createDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.createDeploymentFunc(namespace, deployment)
}

func (i *deploymentClientImpl) listDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error) {
	return i.listDeploymentsForConfigFunc(namespace, configName)
}

func (i *deploymentClientImpl) updateDeployment(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return i.updateDeploymentFunc(namespace, deployment)
}

// findDetails inspects the given deployment configuration for any failure causes
// and returns any found details about it
func (c *DeploymentConfigController) findDetails(config *deployapi.DeploymentConfig) (string, *kapi.ReplicationControllerList, bool, error) {
	// Check if any existing inflight deployments (any non-terminal state).
	existingDeployments, err := c.deploymentClient.listDeploymentsForConfig(config.Namespace, config.Name)
	if err != nil {
		return "", nil, false, fmt.Errorf("couldn't list deployments for deployment config %q: %v", deployutil.LabelForDeploymentConfig(config), err)
	}
	// check if the latest deployment exists
	// we'll return after we've dealt with the multiple-active-deployments case
	latestDeploymentExists, latestDeploymentStatus := deployutil.LatestDeploymentInfo(config, existingDeployments)
	if latestDeploymentExists && latestDeploymentStatus != deployapi.DeploymentStatusFailed {
		// If the latest deployment exists and is not failed, clear the dc message
		return "", existingDeployments, latestDeploymentExists, nil
	}
	bcList, err := c.osClient.BuildConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return "", existingDeployments, latestDeploymentExists, fmt.Errorf("couldn't list build configs: %v", err)
	}
	// Rest of the code will handle non-existing or failed latest deployment causes
	// TODO: Inspect pod logs in case of failed latest
	invalidIsTags := []string{}
	isTagsMissingStreams := map[string]string{}
	isTagBuilds := map[string]string{}
	// Look into image change triggers and find out possible deployment failures such as
	// missing image stream tags with or without build configurations pointing at them
	for _, trigger := range config.Triggers {
		if trigger.Type != deployapi.DeploymentTriggerOnImageChange || trigger.ImageChangeParams == nil || !trigger.ImageChangeParams.Automatic {
			continue
		}
		name := trigger.ImageChangeParams.From.Name
		tag := trigger.ImageChangeParams.Tag
		istag := imageapi.JoinImageStreamTag(name, tag)

		// Check if the image stream tag pointed by the trigger exists
		if _, err := c.osClient.ImageStreamTags(config.Namespace).Get(name, tag); err != nil {
			if !errors.IsNotFound(err) {
				return "", existingDeployments, latestDeploymentExists, fmt.Errorf("couldn't get image stream tag %q: %v", istag, err)
			}
			// In case the image stream tag was not found, then it either doesn't exist or doesn't exist yet
			// (a build configuration output points to it so it's going to be populated at some point in the
			// future)
			invalidIsTags = append(invalidIsTags, istag)
			for _, bc := range bcList.Items {
				if bc.Spec.Output.To != nil && bc.Spec.Output.To.Kind == "ImageStreamTag" {
					parts := strings.Split(bc.Spec.Output.To.Name, ":")
					if len(parts) != 2 {
						return "", existingDeployments, latestDeploymentExists, fmt.Errorf("invalid image stream tag: %q", bc.Spec.Output.To.Name)
					}
					if parts[0] == name && parts[1] == tag {
						isTagBuilds[istag] = bc.Name
						break
					}
				}
			}
			// Try to see if the image stream exists, if not then the build will never be able to update the
			// tag in question
			if _, err := c.osClient.ImageStreams(config.Namespace).Get(name); err != nil {
				if !errors.IsNotFound(err) {
					return "", existingDeployments, latestDeploymentExists, fmt.Errorf("couldn't get image stream %q: %v", name, err)
				}
				isTagsMissingStreams[istag] = name
			}
		}
	}

	details := []string{}
	for _, isTagName := range invalidIsTags {
		if streamName, missingStream := isTagsMissingStreams[isTagName]; missingStream {
			details = append(details, fmt.Sprintf("The image trigger for image stream tag %q will have no effect because image stream %q does not exist.", isTagName, streamName))
			continue
		}
		if buildName, hasBuild := isTagBuilds[isTagName]; hasBuild {
			details = append(details, fmt.Sprintf("The image trigger for image stream tag %q will have no effect because image stream tag %q does not exist.\n\tIf image stream tag %q is expected, check build config %q which produces image stream tag %q.", isTagName, isTagName, isTagName, buildName, isTagName))
		} else {
			details = append(details, fmt.Sprintf("The image trigger for image stream tag %q will have no effect because image stream tag %q does not exist.", isTagName, isTagName))
		}
	}

	if len(details) > 1 {
		for i := range details {
			details[i] = fmt.Sprintf("\t* %s", details[i])
		}
		// Prepend multiple errors warning
		multipleErrWarning := fmt.Sprintf("Deployment config %q blocked by multiple errors:\n", config.Name)
		details = append([]string{multipleErrWarning}, details...)
	}

	return strings.Join(details, "\n"), existingDeployments, latestDeploymentExists, nil
}

// updateDetails updates a deployment configuration with the provided details
func (c *DeploymentConfigController) updateDetails(config *deployapi.DeploymentConfig, details string) (*deployapi.DeploymentConfig, error) {
	if config.Details == nil {
		config.Details = new(deployapi.DeploymentDetails)
	}
	if details != config.Details.Message {
		config.Details.Message = details
		return c.osClient.DeploymentConfigs(config.Namespace).Update(config)
	}
	return config, nil
}
