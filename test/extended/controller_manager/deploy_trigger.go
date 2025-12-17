package controller_manager

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"

	appsv1 "github.com/openshift/api/apps/v1"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/apps/appsutil"
	"github.com/openshift/library-go/pkg/image/imageutil"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-apps][Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("deployment-trigger")

	g.It("TestTriggers_manual [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		const maxUpdateRetries = 10
		namespace := oc.Namespace()

		kc := oc.KubeClient()
		adminAppsClient := oc.AdminAppsClient().AppsV1()

		config := OkDeploymentConfig(0)
		config.Namespace = namespace
		config.Spec.Triggers = []appsv1.DeploymentTriggerPolicy{{Type: "Manual"}}

		dc, err := adminAppsClient.DeploymentConfigs(namespace).Create(context.Background(), config, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
		}

		rcWatch, err := kc.CoreV1().ReplicationControllers(namespace).Watch(context.Background(), metav1.ListOptions{ResourceVersion: dc.ResourceVersion})
		if err != nil {
			t.Fatalf("Couldn't subscribe to Deployments: %v", err)
		}
		defer rcWatch.Stop()

		request := &appsv1.DeploymentRequest{
			Name:   config.Name,
			Latest: false,
			Force:  true,
		}

		retryErr := retry.RetryOnConflict(wait.Backoff{Steps: maxUpdateRetries}, func() error {
			var err error
			config, err = adminAppsClient.DeploymentConfigs(namespace).Instantiate(context.Background(), config.Name, request, metav1.CreateOptions{})
			return err
		})
		if retryErr != nil {
			t.Fatalf("Couldn't instantiate deployment config %q: %v", request.Name, err)
		}
		if config.Status.LatestVersion != 1 {
			t.Fatal("Instantiated deployment config should have version 1")
		}
		if config.Status.Details == nil || len(config.Status.Details.Causes) == 0 {
			t.Fatal("Instantiated deployment config should have a cause of deployment")
		}
		gotType := config.Status.Details.Causes[0].Type
		if gotType != "Manual" {
			t.Fatalf("Instantiated deployment config should have a %q cause of deployment instead of %q",
				"Manual", gotType)
		}

		event := <-rcWatch.ResultChan()
		if e, a := watchapi.Added, event.Type; e != a {
			t.Fatalf("expected watch event type %s, got %s", e, a)
		}
		deployment := event.Object.(*corev1.ReplicationController)

		if e, a := config.Name, appsutil.DeploymentConfigNameFor(deployment); e != a {
			t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
		}
		if e, a := int64(1), appsutil.DeploymentVersionFor(deployment); e != a {
			t.Fatalf("Deployment annotation version does not match: %#v", deployment)
		}
	})
})

var _ = g.Describe("[sig-apps][Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("deployment-trigger")

	// TestTriggers_imageChange ensures that a deployment config with an ImageChange trigger
	// will start a new deployment when an image change happens.
	g.It("TestTriggers_imageChange [apigroup:apps.openshift.io][apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		const registryHostname = "registry:8080"

		projectAdminAppsClient := oc.AppsClient().AppsV1()
		projectAdminImageClient := oc.ImageClient().ImageV1()

		imageStream := &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: ImageStreamName}}

		config := OkDeploymentConfig(0)
		config.Namespace = oc.Namespace()
		config.Spec.Triggers = []appsv1.DeploymentTriggerPolicy{OkImageChangeTrigger()}

		configWatch, err := projectAdminAppsClient.DeploymentConfigs(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Couldn't subscribe to deploymentconfigs %v", err)
		}
		defer configWatch.Stop()

		if imageStream, err = projectAdminImageClient.ImageStreams(oc.Namespace()).Create(context.Background(), imageStream, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Couldn't create imagestream: %v", err)
		}

		imageWatch, err := projectAdminImageClient.ImageStreams(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Couldn't subscribe to imagestreams: %v", err)
		}
		defer imageWatch.Stop()

		updatedImage := fmt.Sprintf("sha256:%s", ImageID)
		updatedPullSpec := fmt.Sprintf("%s/%s/%s@%s", registryHostname, oc.Namespace(), ImageStreamName, updatedImage)
		// Make a function which can create a new tag event for the image stream and
		// then wait for the stream status to be asynchronously updated.
		createTagEvent := func() {
			mapping := &imagev1.ImageStreamMapping{
				ObjectMeta: metav1.ObjectMeta{Name: imageStream.Name},
				Tag:        imagev1.DefaultImageTag,
				Image: imagev1.Image{
					ObjectMeta: metav1.ObjectMeta{
						Name: updatedImage,
					},
					DockerImageReference: updatedPullSpec,
				},
			}
			if _, err := projectAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(context.Background(), mapping, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Log("Waiting for image stream mapping to be reflected in the image stream status...")
		statusLoop:
			for {
				select {
				case event := <-imageWatch.ResultChan():
					stream := event.Object.(*imagev1.ImageStream)
					if _, ok := imageutil.StatusHasTag(stream, imagev1.DefaultImageTag); ok {
						t.Logf("imagestream %q now has status with tags: %#v", stream.Name, stream.Status.Tags)
						break statusLoop
					}
					t.Logf("Still waiting for latest tag status on imagestream %q", stream.Name)
				}
			}
		}

		if config, err = projectAdminAppsClient.DeploymentConfigs(oc.Namespace()).Create(context.Background(), config, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Couldn't create deploymentconfig: %v", err)
		}

		createTagEvent()

		var newConfig *appsv1.DeploymentConfig
		t.Log("Waiting for a new deployment config in response to imagestream update")
	waitForNewConfig:
		for {
			select {
			case event := <-configWatch.ResultChan():
				if event.Type == watchapi.Modified {
					newConfig = event.Object.(*appsv1.DeploymentConfig)
					// Multiple updates to the config can be expected (e.g. status
					// updates), so wait for a significant update (e.g. version).
					if newConfig.Status.LatestVersion > 0 {
						if e, a := updatedPullSpec, newConfig.Spec.Template.Spec.Containers[0].Image; e != a {
							t.Fatalf("unexpected image for pod template container 0; expected %q, got %q", e, a)
						}
						break waitForNewConfig
					}
					t.Log("Still waiting for a new deployment config in response to imagestream update")
				}
			}
		}

	})
})

var _ = g.Describe("[sig-apps][Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("deployment-trigger")

	// TestTriggers_imageChange_nonAutomatic ensures that a deployment config with a non-automatic
	// trigger will have its image updated when a deployment is started manually.
	g.It("TestTriggers_imageChange_nonAutomatic [apigroup:image.openshift.io][apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		const maxUpdateRetries = 10
		const registryHostname = "registry:8080"

		adminAppsClient := oc.AppsClient().AppsV1()
		adminImageClient := oc.ImageClient().ImageV1()

		imageStream := &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: ImageStreamName}}
		var err error

		if imageStream, err = adminImageClient.ImageStreams(oc.Namespace()).Create(context.Background(), imageStream, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Couldn't create imagestream: %v", err)
		}

		imageWatch, err := adminImageClient.ImageStreams(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Couldn't subscribe to imagestreams: %v", err)
		}
		defer imageWatch.Stop()

		image := fmt.Sprintf("sha256:%s", ImageID)
		pullSpec := fmt.Sprintf("registry:5000/%s/%s@%s", oc.Namespace(), ImageStreamName, image)
		// Make a function which can create a new tag event for the image stream and
		// then wait for the stream status to be asynchronously updated.
		mapping := &imagev1.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{Name: imageStream.Name},
			Tag:        imagev1.DefaultImageTag,
			Image: imagev1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: image,
				},
				DockerImageReference: pullSpec,
			},
		}

		createTagEvent := func(mapping *imagev1.ImageStreamMapping) {
			if _, err := adminImageClient.ImageStreamMappings(oc.Namespace()).Create(context.Background(), mapping, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Log("Waiting for image stream mapping to be reflected in the image stream status...")

			timeout := time.After(time.Minute)

			for {
				select {
				case event := <-imageWatch.ResultChan():
					switch event.Type {
					case watchapi.Error:
						if status, ok := event.Object.(*metav1.Status); ok {
							t.Fatalf("unexpected error from watcher: %v", errors.FromObject(status))
						}
						t.Fatalf("unexpected object from watcher: %#v", event.Object)
					default:
						stream := event.Object.(*imagev1.ImageStream)
						tagEventList, ok := imageutil.StatusHasTag(stream, imagev1.DefaultImageTag)
						if ok && len(tagEventList.Items) > 0 && tagEventList.Items[0].DockerImageReference == mapping.Image.DockerImageReference {
							t.Logf("imagestream %q now has status with tags: %#v", stream.Name, stream.Status.Tags)
							return
						}
						if len(tagEventList.Items) > 0 {
							t.Logf("want: %s, got: %s", mapping.Image.DockerImageReference, tagEventList.Items[0].DockerImageReference)
						}
						t.Logf("Still waiting for latest tag status update on imagestream %q with tags: %#v", stream.Name, tagEventList)
					}
				case <-timeout:
					t.Fatalf("timed out waiting for image stream %q to be updated", imageStream.Name)
				}
			}
		}

		configWatch, err := adminAppsClient.DeploymentConfigs(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Couldn't subscribe to deploymentconfigs: %v", err)
		}
		defer configWatch.Stop()

		config := OkDeploymentConfig(0)
		config.Namespace = oc.Namespace()
		config.Spec.Triggers = []appsv1.DeploymentTriggerPolicy{OkImageChangeTrigger()}
		config.Spec.Triggers[0].ImageChangeParams.Automatic = false
		if config, err = adminAppsClient.DeploymentConfigs(oc.Namespace()).Create(context.Background(), config, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Couldn't create deploymentconfig: %v", err)
		}

		createTagEvent(mapping)

		var newConfig *appsv1.DeploymentConfig
		t.Log("Waiting for the first imagestream update - no deployment should run")

		timeout := time.After(20 * time.Second)

		// Deployment config with automatic=false in its ICT - no deployment should trigger.
		// We don't really care about the initial update since it's not going to be deployed
		// anyway.
	out:
		for {
			select {
			case event := <-configWatch.ResultChan():
				if event.Type != watchapi.Modified {
					continue
				}

				newConfig = event.Object.(*appsv1.DeploymentConfig)

				if newConfig.Status.LatestVersion > 0 {
					t.Fatalf("unexpected latestVersion update - the config has no config change trigger")
				}

			case <-timeout:
				break out
			}
		}

		t.Log("Waiting for the second imagestream update - no deployment should run")

		// Subsequent updates to the image shouldn't update the pod template image
		mapping.Image.Name = "sha256:0000000000000000000000000000000000000000000000000000000000000321"
		mapping.Image.DockerImageReference = fmt.Sprintf("%s/%s/%s@%s", registryHostname, oc.Namespace(), ImageStreamName, mapping.Image.Name)
		createTagEvent(mapping)

		timeout = time.After(20 * time.Second)

	loop:
		for {
			select {
			case event := <-configWatch.ResultChan():
				if event.Type != watchapi.Modified {
					continue
				}

				newConfig = event.Object.(*appsv1.DeploymentConfig)

				if newConfig.Status.LatestVersion > 0 {
					t.Fatalf("unexpected latestVersion update - the config has no config change trigger")
				}

			case <-timeout:
				break loop
			}
		}

		t.Log("Instantiate the deployment config - the latest image should be picked up and a new deployment should run")
		request := &appsv1.DeploymentRequest{
			Name:   config.Name,
			Latest: true,
			Force:  true,
		}
		retryErr := retry.RetryOnConflict(wait.Backoff{Steps: maxUpdateRetries}, func() error {
			var err error
			config, err = adminAppsClient.DeploymentConfigs(oc.Namespace()).Instantiate(context.Background(), config.Name, request, metav1.CreateOptions{})
			return err
		})
		if retryErr != nil {
			t.Fatalf("Couldn't instantiate deployment config %q: %v", request.Name, err)
		}
		config, err = adminAppsClient.DeploymentConfigs(config.Namespace).Get(context.Background(), config.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if exp, got := mapping.Image.DockerImageReference, config.Spec.Template.Spec.Containers[0].Image; exp != got {
			t.Fatalf("Expected image %q instead of %q to be updated in deployment config %q", exp, got, config.Name)
		}
		if exp, got := int64(1), config.Status.LatestVersion; exp != got {
			t.Fatalf("Expected latestVersion for deployment config %q to be %d, got %d", config.Name, exp, got)
		}
		if config.Status.Details == nil || len(config.Status.Details.Causes) == 0 {
			t.Fatalf("Expected a cause of deployment for deployment config %q", config.Name)
		}
		if gotType, expectedType := config.Status.Details.Causes[0].Type, "Manual"; string(gotType) != expectedType {
			t.Fatalf("Instantiated deployment config should have a %q cause of deployment instead of %q", expectedType, gotType)
		}

	})
})

var _ = g.Describe("[sig-apps][Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("deployment-trigger")

	// TestTriggers_MultipleICTs ensures that a deployment config with more than one ImageChange trigger
	// will start a new deployment iff all images are resolved.
	g.It("TestTriggers_MultipleICTs [apigroup:apps.openshift.io][apigroup:images.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		const registryHostname = "registry:8080"
		adminAppsClient := oc.AppsClient().AppsV1()
		adminImageClient := oc.ImageClient().ImageV1()

		imageStream := &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: ImageStreamName}}
		secondImageStream := &imagev1.ImageStream{ObjectMeta: metav1.ObjectMeta{Name: "sample"}}

		config := OkDeploymentConfig(0)
		config.Namespace = oc.Namespace()
		firstTrigger := OkImageChangeTrigger()
		secondTrigger := OkImageChangeTrigger()
		secondTrigger.ImageChangeParams.ContainerNames = []string{"container2"}
		secondTrigger.ImageChangeParams.From.Name = imageutil.JoinImageStreamTag("sample", imagev1.DefaultImageTag)
		config.Spec.Triggers = []appsv1.DeploymentTriggerPolicy{firstTrigger, secondTrigger}

		configWatch, err := adminAppsClient.DeploymentConfigs(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Couldn't subscribe to deploymentconfigs %v", err)
		}
		defer configWatch.Stop()

		if imageStream, err = adminImageClient.ImageStreams(oc.Namespace()).Create(context.Background(), imageStream, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Couldn't create imagestream %q: %v", imageStream.Name, err)
		}
		if secondImageStream, err = adminImageClient.ImageStreams(oc.Namespace()).Create(context.Background(), secondImageStream, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Couldn't create imagestream %q: %v", secondImageStream.Name, err)
		}

		imageWatch, err := adminImageClient.ImageStreams(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Couldn't subscribe to imagestreams: %v", err)
		}
		defer imageWatch.Stop()

		updatedImage := fmt.Sprintf("sha256:%s", ImageID)
		updatedPullSpec := fmt.Sprintf("%s/%s/%s@%s", registryHostname, oc.Namespace(), ImageStreamName, updatedImage)

		// Make a function which can create a new tag event for the image stream and
		// then wait for the stream status to be asynchronously updated.
		createTagEvent := func(name, tag, image, pullSpec string) {
			mapping := &imagev1.ImageStreamMapping{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Tag:        tag,
				Image: imagev1.Image{
					ObjectMeta: metav1.ObjectMeta{
						Name: image,
					},
					DockerImageReference: pullSpec,
				},
			}
			if _, err := adminImageClient.ImageStreamMappings(oc.Namespace()).Create(context.Background(), mapping, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Log("Waiting for image stream mapping to be reflected in the image stream status...")
		statusLoop:
			for {
				select {
				case event := <-imageWatch.ResultChan():
					stream := event.Object.(*imagev1.ImageStream)
					if stream.Name != name {
						continue
					}
					if _, ok := imageutil.StatusHasTag(stream, tag); ok {
						t.Logf("imagestream %q now has status with tags: %#v", stream.Name, stream.Status.Tags)
						break statusLoop
					}
					t.Logf("Still waiting for latest tag status on imagestream %q", stream.Name)
				}
			}
		}

		if config, err = adminAppsClient.DeploymentConfigs(oc.Namespace()).Create(context.Background(), config, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Couldn't create deploymentconfig: %v", err)
		}

		timeout := time.After(30 * time.Second)

		t.Log("Should not trigger a new deployment in response to the first imagestream update")
		createTagEvent(imageStream.Name, imagev1.DefaultImageTag, updatedImage, updatedPullSpec)
	out:
		for {
			select {
			case event := <-configWatch.ResultChan():
				if event.Type != watchapi.Modified {
					continue
				}

				newConfig := event.Object.(*appsv1.DeploymentConfig)
				if newConfig.Status.LatestVersion > 0 {
					t.Fatalf("unexpected latestVersion update: %#v", newConfig)
				}
				container := newConfig.Spec.Template.Spec.Containers[0]
				if e, a := updatedPullSpec, container.Image; e == a {
					t.Fatalf("unexpected image update: %#v", newConfig)
				}

			case <-timeout:
				break out
			}
		}

		timeout = time.After(30 * time.Second)

		t.Log("Should trigger a new deployment in response to the second imagestream update")
		secondImage := "sampleimage"
		secondPullSpec := "samplepullspec"
		createTagEvent(secondImageStream.Name, imagev1.DefaultImageTag, secondImage, secondPullSpec)
		for {
		inner:
			select {
			case event := <-configWatch.ResultChan():
				if event.Type != watchapi.Modified {
					continue
				}

				newConfig := event.Object.(*appsv1.DeploymentConfig)
				switch {
				case newConfig.Status.LatestVersion == 0:
					t.Logf("Wating for latestVersion to update to 1")
					break inner
				case newConfig.Status.LatestVersion > 1:
					t.Fatalf("unexpected latestVersion %d for %#v", newConfig.Status.LatestVersion, newConfig)
				default:
					// Keep on
				}

				container := newConfig.Spec.Template.Spec.Containers[0]
				if e, a := updatedPullSpec, container.Image; e != a {
					t.Fatalf("unexpected image for pod template container %q; expected %q, got %q", container.Name, e, a)
				}

				container = newConfig.Spec.Template.Spec.Containers[1]
				if e, a := secondPullSpec, container.Image; e != a {
					t.Fatalf("unexpected image for pod template container %q; expected %q, got %q", container.Name, e, a)
				}

				return

			case <-timeout:
				t.Fatalf("timed out waiting for the second image update to happen")
			}
		}

	})
})

var _ = g.Describe("[sig-apps][Feature:OpenShiftControllerManager]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("deployment-trigger")

	// TestTriggers_configChange ensures that a change in the template of a deployment config with
	// a config change trigger will start a new deployment.
	g.It("TestTriggers_configChange [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
		t := g.GinkgoT()

		const maxUpdateRetries = 10
		namespace := oc.Namespace()

		kc := oc.KubeClient()
		adminAppsClient := oc.AdminAppsClient().AppsV1()

		config := OkDeploymentConfig(0)
		config.Namespace = namespace
		config.Spec.Triggers = []appsv1.DeploymentTriggerPolicy{{Type: appsv1.DeploymentTriggerOnConfigChange}}

		rcWatch, err := kc.CoreV1().ReplicationControllers(namespace).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Couldn't subscribe to Deployments %v", err)
		}
		defer rcWatch.Stop()

		// submit the initial deployment config
		config, err = adminAppsClient.DeploymentConfigs(namespace).Create(context.Background(), config, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Couldn't create DeploymentConfig: %v", err)
		}

		// verify the initial deployment exists
		event := <-rcWatch.ResultChan()
		if e, a := watchapi.Added, event.Type; e != a {
			t.Fatalf("expected watch event type %s, got %s", e, a)
		}

		deployment := event.Object.(*corev1.ReplicationController)

		if e, a := config.Name, appsutil.DeploymentConfigNameFor(deployment); e != a {
			t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
		}

		// before we update the config, we need to update the state of the existing deployment
		// this is required to be done manually since the deployment and deployer pod controllers are not run in this test
		// get this live or conflicts will never end up resolved
		retryErr := retry.RetryOnConflict(wait.Backoff{Steps: maxUpdateRetries}, func() error {
			liveDeployment, err := kc.CoreV1().ReplicationControllers(deployment.Namespace).Get(context.Background(), deployment.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			liveDeployment.Annotations[appsv1.DeploymentStatusAnnotation] = string(appsv1.DeploymentStatusComplete)

			// update the deployment
			_, err = kc.CoreV1().ReplicationControllers(namespace).Update(context.Background(), liveDeployment, metav1.UpdateOptions{})
			return err
		})
		if retryErr != nil {
			t.Fatal(retryErr)
		}

		event = <-rcWatch.ResultChan()
		if e, a := watchapi.Modified, event.Type; e != a {
			t.Fatalf("expected watch event type %s, got %s", e, a)
		}

		assertEnvVarEquals("ENV1", "VAL1", deployment, t)

		// Update the config with a new environment variable and observe a new deployment
		// coming up.
		retryErr = retry.RetryOnConflict(wait.Backoff{Steps: maxUpdateRetries}, func() error {
			latest, err := adminAppsClient.DeploymentConfigs(namespace).Get(context.Background(), config.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			for i, e := range latest.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "ENV1" {
					latest.Spec.Template.Spec.Containers[0].Env[i].Value = "UPDATED"
					break
				}
			}

			// update the config
			_, err = adminAppsClient.DeploymentConfigs(namespace).Update(context.Background(), latest, metav1.UpdateOptions{})
			return err
		})
		if retryErr != nil {
			t.Fatal(retryErr)
		}

		if retryErr := retry.RetryOnConflict(wait.Backoff{Steps: maxUpdateRetries}, func() error {
			// submit a new config with an updated environment variable
			newConfig, err := adminAppsClient.DeploymentConfigs(namespace).Get(context.Background(), config.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			newConfig.Spec.Template.Spec.Containers[0].Env[0].Value = "UPDATED"
			_, err = adminAppsClient.DeploymentConfigs(namespace).Update(context.Background(), newConfig, metav1.UpdateOptions{})
			return err
		}); retryErr != nil {
			t.Fatal(retryErr)
		}

		var newDeployment *corev1.ReplicationController
		for {
			event = <-rcWatch.ResultChan()
			if event.Type != watchapi.Added {
				// Discard modifications which could be applied to the original RC, etc.
				continue
			}
			newDeployment = event.Object.(*corev1.ReplicationController)
			break
		}

		assertEnvVarEquals("ENV1", "UPDATED", newDeployment, t)

		if newDeployment.Name == deployment.Name {
			t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
		}

	})
})

func assertEnvVarEquals(name string, value string, deployment *corev1.ReplicationController, t g.GinkgoTInterface) {
	env := deployment.Spec.Template.Spec.Containers[0].Env

	for _, e := range env {
		if e.Name == name && e.Value == value {
			return
		}
	}

	t.Fatalf("Expected env var with name %s and value %s", name, value)
}
