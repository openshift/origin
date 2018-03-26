package controller

import (
	"context"
	"fmt"
	"time"

	kappsv1beta1 "k8s.io/api/apps/v1beta1"
	kappsv1beta2 "k8s.io/api/apps/v1beta2"
	kbatchv1 "k8s.io/api/batch/v1"
	kbatchv1beta1 "k8s.io/api/batch/v1beta1"
	kbatchv2alpha1 "k8s.io/api/batch/v2alpha1"
	kapiv1 "k8s.io/api/core/v1"
	kextensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"

	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imagecontroller "github.com/openshift/origin/pkg/image/controller"
	imagesignaturecontroller "github.com/openshift/origin/pkg/image/controller/signature"
	imagetriggercontroller "github.com/openshift/origin/pkg/image/controller/trigger"
	triggerannotations "github.com/openshift/origin/pkg/image/trigger/annotations"
	triggerbuildconfigs "github.com/openshift/origin/pkg/image/trigger/buildconfigs"
	triggerdeploymentconfigs "github.com/openshift/origin/pkg/image/trigger/deploymentconfigs"
)

func RunImageTriggerController(ctx ControllerContext) (bool, error) {
	informer := ctx.ImageInformers.Image().InternalVersion().ImageStreams()

	buildClient, err := ctx.ClientBuilder.OpenshiftInternalBuildClient(bootstrappolicy.InfraImageTriggerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	appsClient, err := ctx.ClientBuilder.OpenshiftInternalAppsClient(bootstrappolicy.InfraImageTriggerControllerServiceAccountName)
	if err != nil {
		return true, err
	}
	kclient := ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraImageTriggerControllerServiceAccountName)

	updater := podSpecUpdater{kclient}
	bcInstantiator := buildclient.NewClientBuildConfigInstantiatorClient(buildClient)
	broadcaster := imagetriggercontroller.NewTriggerEventBroadcaster(kv1core.New(kclient.CoreV1().RESTClient()))

	sources := []imagetriggercontroller.TriggerSource{
		{
			Resource:  schema.GroupResource{Group: "apps.openshift.io", Resource: "deploymentconfigs"},
			Informer:  ctx.AppInformers.Apps().InternalVersion().DeploymentConfigs().Informer(),
			Store:     ctx.AppInformers.Apps().InternalVersion().DeploymentConfigs().Informer().GetIndexer(),
			TriggerFn: triggerdeploymentconfigs.NewDeploymentConfigTriggerIndexer,
			Reactor:   &triggerdeploymentconfigs.DeploymentConfigReactor{Client: appsClient.Apps()},
		},
	}
	sources = append(sources, imagetriggercontroller.TriggerSource{
		Resource:  schema.GroupResource{Group: "build.openshift.io", Resource: "buildconfigs"},
		Informer:  ctx.BuildInformers.Build().InternalVersion().BuildConfigs().Informer(),
		Store:     ctx.BuildInformers.Build().InternalVersion().BuildConfigs().Informer().GetIndexer(),
		TriggerFn: triggerbuildconfigs.NewBuildConfigTriggerIndexer,
		Reactor:   triggerbuildconfigs.NewBuildConfigReactor(bcInstantiator, kclient.Core().RESTClient()),
	})
	sources = append(sources, imagetriggercontroller.TriggerSource{
		Resource:  schema.GroupResource{Group: "extensions", Resource: "deployments"},
		Informer:  ctx.ExternalKubeInformers.Extensions().V1beta1().Deployments().Informer(),
		Store:     ctx.ExternalKubeInformers.Extensions().V1beta1().Deployments().Informer().GetIndexer(),
		TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
		Reactor:   &triggerannotations.AnnotationReactor{Updater: updater},
	})
	sources = append(sources, imagetriggercontroller.TriggerSource{
		Resource:  schema.GroupResource{Group: "extensions", Resource: "daemonsets"},
		Informer:  ctx.ExternalKubeInformers.Extensions().V1beta1().DaemonSets().Informer(),
		Store:     ctx.ExternalKubeInformers.Extensions().V1beta1().DaemonSets().Informer().GetIndexer(),
		TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
		Reactor:   &triggerannotations.AnnotationReactor{Updater: updater},
	})
	sources = append(sources, imagetriggercontroller.TriggerSource{
		Resource:  schema.GroupResource{Group: "apps", Resource: "statefulsets"},
		Informer:  ctx.ExternalKubeInformers.Apps().V1beta1().StatefulSets().Informer(),
		Store:     ctx.ExternalKubeInformers.Apps().V1beta1().StatefulSets().Informer().GetIndexer(),
		TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
		Reactor:   &triggerannotations.AnnotationReactor{Updater: updater},
	})
	sources = append(sources, imagetriggercontroller.TriggerSource{
		Resource:  schema.GroupResource{Group: "batch", Resource: "cronjobs"},
		Informer:  ctx.ExternalKubeInformers.Batch().V2alpha1().CronJobs().Informer(),
		Store:     ctx.ExternalKubeInformers.Batch().V2alpha1().CronJobs().Informer().GetIndexer(),
		TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
		Reactor:   &triggerannotations.AnnotationReactor{Updater: updater},
	})

	go imagetriggercontroller.NewTriggerController(
		broadcaster,
		informer,
		sources...,
	).Run(5, ctx.Stop)

	return true, nil
}

type podSpecUpdater struct {
	kclient kclientsetexternal.Interface
}

func (u podSpecUpdater) Update(obj runtime.Object) error {
	switch t := obj.(type) {
	case *kextensionsv1beta1.DaemonSet:
		_, err := u.kclient.Extensions().DaemonSets(t.Namespace).Update(t)
		return err
	case *kextensionsv1beta1.Deployment:
		_, err := u.kclient.Extensions().Deployments(t.Namespace).Update(t)
		return err
	case *kappsv1beta1.Deployment:
		_, err := u.kclient.AppsV1beta1().Deployments(t.Namespace).Update(t)
		return err
	case *kappsv1beta2.Deployment:
		_, err := u.kclient.AppsV1beta2().Deployments(t.Namespace).Update(t)
		return err
	case *kappsv1beta1.StatefulSet:
		_, err := u.kclient.AppsV1beta1().StatefulSets(t.Namespace).Update(t)
		return err
	case *kappsv1beta2.StatefulSet:
		_, err := u.kclient.AppsV1beta2().StatefulSets(t.Namespace).Update(t)
		return err
	case *kbatchv1.Job:
		_, err := u.kclient.Batch().Jobs(t.Namespace).Update(t)
		return err
	case *kbatchv1beta1.CronJob:
		_, err := u.kclient.BatchV1beta1().CronJobs(t.Namespace).Update(t)
		return err
	case *kbatchv2alpha1.CronJob:
		_, err := u.kclient.BatchV2alpha1().CronJobs(t.Namespace).Update(t)
		return err
	case *kapiv1.Pod:
		_, err := u.kclient.Core().Pods(t.Namespace).Update(t)
		return err
	default:
		return fmt.Errorf("unrecognized object - no trigger update possible for %T", obj)
	}
}

type ImageSignatureImportControllerConfig struct {
	ResyncPeriod          time.Duration
	SignatureFetchTimeout time.Duration
	SignatureImportLimit  int
}

func (c *ImageSignatureImportControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	controller := imagesignaturecontroller.NewSignatureImportController(
		context.Background(),
		ctx.ClientBuilder.OpenshiftInternalImageClientOrDie(bootstrappolicy.InfraImageImportControllerServiceAccountName),
		ctx.ImageInformers.Image().InternalVersion().Images(),
		c.ResyncPeriod,
		c.SignatureFetchTimeout,
		c.SignatureImportLimit,
	)
	go controller.Run(5, ctx.Stop)
	return true, nil
}

type ImageImportControllerConfig struct {
	MaxScheduledImageImportsPerMinute          int
	ScheduledImageImportMinimumIntervalSeconds int
	DisableScheduledImport                     bool
	ResyncPeriod                               time.Duration
}

func (c *ImageImportControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	informer := ctx.ImageInformers.Image().InternalVersion().ImageStreams()
	controller := imagecontroller.NewImageStreamController(
		ctx.ClientBuilder.OpenshiftInternalImageClientOrDie(bootstrappolicy.InfraImageImportControllerServiceAccountName),
		informer,
	)
	go controller.Run(5, ctx.Stop)

	if c.DisableScheduledImport {
		return true, nil
	}

	scheduledController := imagecontroller.NewScheduledImageStreamController(
		ctx.ClientBuilder.OpenshiftInternalImageClientOrDie(bootstrappolicy.InfraImageImportControllerServiceAccountName),
		informer,
		imagecontroller.ScheduledImageStreamControllerOptions{
			Resync: time.Duration(c.ScheduledImageImportMinimumIntervalSeconds) * time.Second,

			Enabled:                  !c.DisableScheduledImport,
			DefaultBucketSize:        4,
			MaxImageImportsPerMinute: c.MaxScheduledImageImportsPerMinute,
		},
	)

	controller.SetNotifier(scheduledController)
	go scheduledController.Run(ctx.Stop)

	return true, nil
}
