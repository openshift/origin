package integration

import (
	"fmt"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
	watchapi "k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const maxUpdateRetries = 5

func TestTriggers_manual(t *testing.T) {
	const namespace = "test-triggers-manual"

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, namespace, "my-test-user")
	if err != nil {
		t.Fatal(err)
	}
	osClient, kubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "my-test-user")
	if err != nil {
		t.Fatal(err)
	}

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = namespace
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{
		{
			Type: deployapi.DeploymentTriggerManual,
		},
	}

	dc, err := osClient.DeploymentConfigs(namespace).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
	}

	rcWatch, err := kubeClient.ReplicationControllers(namespace).Watch(kapi.ListOptions{ResourceVersion: dc.ResourceVersion})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments: %v", err)
	}
	defer rcWatch.Stop()

	retryErr := kclient.RetryOnConflict(wait.Backoff{Steps: maxUpdateRetries}, func() error {
		config, err := osClient.DeploymentConfigs(namespace).Generate(config.Name)
		if err != nil {
			return err
		}
		if config.Status.LatestVersion != 1 {
			t.Fatalf("Generated deployment should have version 1: %#v", config)
		}
		t.Logf("config(1): %#v", config)
		updatedConfig, err := osClient.DeploymentConfigs(namespace).Update(config)
		if err != nil {
			return err
		}
		t.Logf("config(2): %#v", updatedConfig)
		return nil
	})
	if retryErr != nil {
		t.Fatal(err)
	}
	event := <-rcWatch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	deployment := event.Object.(*kapi.ReplicationController)

	if e, a := config.Name, deployutil.DeploymentConfigNameFor(deployment); e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}
	if e, a := int64(1), deployutil.DeploymentVersionFor(deployment); e != a {
		t.Fatalf("Deployment annotation version does not match: %#v", deployment)
	}
}

// TestTriggers_imageChange ensures that a deployment config with an ImageChange trigger
// will start a new deployment when an image change happens.
func TestTriggers_imageChange(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("error starting master: %v", err)
	}
	openshiftClusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client: %v", err)
	}
	openshiftClusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client config: %v", err)
	}
	openshiftProjectAdminClient, err := testserver.CreateNewProject(openshiftClusterAdminClient, *openshiftClusterAdminClientConfig, testutil.Namespace(), "bob")
	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}

	imageStream := &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: deploytest.ImageStreamName}}

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{deploytest.OkImageChangeTrigger()}

	configWatch, err := openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to deploymentconfigs %v", err)
	}
	defer configWatch.Stop()

	if imageStream, err = openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Couldn't create imagestream: %v", err)
	}

	imageWatch, err := openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to imagestreams: %v", err)
	}
	defer imageWatch.Stop()

	updatedImage := fmt.Sprintf("sha256:%s", deploytest.ImageID)
	updatedPullSpec := fmt.Sprintf("registry:8080/%s/%s@%s", testutil.Namespace(), deploytest.ImageStreamName, updatedImage)
	// Make a function which can create a new tag event for the image stream and
	// then wait for the stream status to be asynchronously updated.
	createTagEvent := func() {
		mapping := &imageapi.ImageStreamMapping{
			ObjectMeta: kapi.ObjectMeta{Name: imageStream.Name},
			Tag:        imageapi.DefaultImageTag,
			Image: imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: updatedImage,
				},
				DockerImageReference: updatedPullSpec,
			},
		}
		if err := openshiftProjectAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		t.Log("Waiting for image stream mapping to be reflected in the image stream status...")
	statusLoop:
		for {
			select {
			case event := <-imageWatch.ResultChan():
				stream := event.Object.(*imageapi.ImageStream)
				if _, ok := stream.Status.Tags[imageapi.DefaultImageTag]; ok {
					t.Logf("imagestream %q now has status with tags: %#v", stream.Name, stream.Status.Tags)
					break statusLoop
				}
				t.Logf("Still waiting for latest tag status on imagestream %q", stream.Name)
			}
		}
	}

	if config, err = openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Create(config); err != nil {
		t.Fatalf("Couldn't create deploymentconfig: %v", err)
	}

	createTagEvent()

	var newConfig *deployapi.DeploymentConfig
	t.Log("Waiting for a new deployment config in response to imagestream update")
waitForNewConfig:
	for {
		select {
		case event := <-configWatch.ResultChan():
			if event.Type == watchapi.Modified {
				newConfig = event.Object.(*deployapi.DeploymentConfig)
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
}

// TestTriggers_imageChange_nonAutomatic ensures that a deployment config with a non-automatic
// trigger will have its image updated without starting a new deployment.
func TestTriggers_imageChange_nonAutomatic(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("error starting master: %v", err)
	}
	openshiftClusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client: %v", err)
	}
	openshiftClusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client config: %v", err)
	}
	openshiftProjectAdminClient, err := testserver.CreateNewProject(openshiftClusterAdminClient, *openshiftClusterAdminClientConfig, testutil.Namespace(), "bob")
	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}

	imageStream := &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: deploytest.ImageStreamName}}

	if imageStream, err = openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Couldn't create imagestream: %v", err)
	}

	imageWatch, err := openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to imagestreams: %v", err)
	}
	defer imageWatch.Stop()

	image := fmt.Sprintf("sha256:%s", deploytest.ImageID)
	pullSpec := fmt.Sprintf("registry:5000/%s/%s@%s", testutil.Namespace(), deploytest.ImageStreamName, image)
	// Make a function which can create a new tag event for the image stream and
	// then wait for the stream status to be asynchronously updated.
	mapping := &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: imageStream.Name},
		Tag:        imageapi.DefaultImageTag,
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: image,
			},
			DockerImageReference: pullSpec,
		},
	}
	updated := ""

	createTagEvent := func(mapping *imageapi.ImageStreamMapping) {
		if err := openshiftProjectAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		t.Log("Waiting for image stream mapping to be reflected in the image stream status...")

		for {
			select {
			case event := <-imageWatch.ResultChan():
				stream := event.Object.(*imageapi.ImageStream)
				tagEventList, ok := stream.Status.Tags[imageapi.DefaultImageTag]
				if ok {
					if updated != tagEventList.Items[0].DockerImageReference {
						updated = tagEventList.Items[0].DockerImageReference
						return
					}
				}
				t.Logf("Still waiting for latest tag status update on imagestream %q", stream.Name)
			}
		}
	}

	configWatch, err := openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to deploymentconfigs: %v", err)
	}
	defer configWatch.Stop()

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{deploytest.OkImageChangeTrigger()}
	config.Spec.Triggers[0].ImageChangeParams.Automatic = false
	if config, err = openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Create(config); err != nil {
		t.Fatalf("Couldn't create deploymentconfig: %v", err)
	}

	createTagEvent(mapping)

	var newConfig *deployapi.DeploymentConfig
	t.Log("Waiting for the initial deploymentconfig update in response to the imagestream update")

	timeout := time.After(30 * time.Second)

	// This is the initial deployment with automatic=false in its ICT - it should be updated to pullSpec
out:
	for {
		select {
		case event := <-configWatch.ResultChan():
			if event.Type != watchapi.Modified {
				continue
			}

			newConfig = event.Object.(*deployapi.DeploymentConfig)

			if newConfig.Status.LatestVersion > 0 {
				t.Fatalf("unexpected latestVersion update - the config has no config change trigger")
			}

			if e, a := updated, newConfig.Spec.Template.Spec.Containers[0].Image; e == a {
				break out
			}
		case <-timeout:
			t.Fatalf("timed out waiting for the image update to happen")
		}
	}

	t.Log("Waiting for the second imagestream update - it shouldn't update the deploymentconfig")

	// Subsequent updates to the image shouldn't update the pod template image
	mapping.Image.Name = "sha256:thisupdatedimageshouldneverlandinthepodtemplate"
	mapping.Image.DockerImageReference = fmt.Sprintf("registry:8080/%s/%s@%s", testutil.Namespace(), deploytest.ImageStreamName, mapping.Image.Name)
	createTagEvent(mapping)

	for {
		select {
		case event := <-configWatch.ResultChan():
			if event.Type != watchapi.Modified {
				continue
			}

			newConfig = event.Object.(*deployapi.DeploymentConfig)

			if newConfig.Status.LatestVersion > 0 {
				t.Fatalf("unexpected latestVersion update - the config has no config change trigger")
			}

			if e, a := updated, newConfig.Spec.Template.Spec.Containers[0].Image; e == a {
				t.Fatalf("unexpected image update, expected initial image to be the same: %#v", newConfig)
			}
		case <-timeout:
			return
		}
	}
}

// TestTriggers_MultipleICTs ensures that a deployment config with more than one ImageChange trigger
// will start a new deployment iff all images are resolved.
func TestTriggers_MultipleICTs(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("error starting master: %v", err)
	}
	openshiftClusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client: %v", err)
	}
	openshiftClusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client config: %v", err)
	}
	openshiftProjectAdminClient, err := testserver.CreateNewProject(openshiftClusterAdminClient, *openshiftClusterAdminClientConfig, testutil.Namespace(), "bob")
	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}

	imageStream := &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: deploytest.ImageStreamName}}
	secondImageStream := &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "sample"}}

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()
	firstTrigger := deploytest.OkImageChangeTrigger()
	secondTrigger := deploytest.OkImageChangeTrigger()
	secondTrigger.ImageChangeParams.ContainerNames = []string{"container2"}
	secondTrigger.ImageChangeParams.From.Name = imageapi.JoinImageStreamTag("sample", imageapi.DefaultImageTag)
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{firstTrigger, secondTrigger}

	configWatch, err := openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to deploymentconfigs %v", err)
	}
	defer configWatch.Stop()

	if imageStream, err = openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Couldn't create imagestream %q: %v", imageStream.Name, err)
	}
	if secondImageStream, err = openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Create(secondImageStream); err != nil {
		t.Fatalf("Couldn't create imagestream %q: %v", secondImageStream.Name, err)
	}

	imageWatch, err := openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to imagestreams: %v", err)
	}
	defer imageWatch.Stop()

	updatedImage := fmt.Sprintf("sha256:%s", deploytest.ImageID)
	updatedPullSpec := fmt.Sprintf("registry:8080/%s/%s@%s", testutil.Namespace(), deploytest.ImageStreamName, updatedImage)

	// Make a function which can create a new tag event for the image stream and
	// then wait for the stream status to be asynchronously updated.
	createTagEvent := func(name, tag, image, pullSpec string) {
		mapping := &imageapi.ImageStreamMapping{
			ObjectMeta: kapi.ObjectMeta{Name: name},
			Tag:        tag,
			Image: imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: image,
				},
				DockerImageReference: pullSpec,
			},
		}
		if err := openshiftProjectAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		t.Log("Waiting for image stream mapping to be reflected in the image stream status...")
	statusLoop:
		for {
			select {
			case event := <-imageWatch.ResultChan():
				stream := event.Object.(*imageapi.ImageStream)
				if stream.Name != name {
					continue
				}
				if _, ok := stream.Status.Tags[tag]; ok {
					t.Logf("imagestream %q now has status with tags: %#v", stream.Name, stream.Status.Tags)
					break statusLoop
				}
				t.Logf("Still waiting for latest tag status on imagestream %q", stream.Name)
			}
		}
	}

	if config, err = openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Create(config); err != nil {
		t.Fatalf("Couldn't create deploymentconfig: %v", err)
	}

	timeout := time.After(30 * time.Second)

	t.Log("Should not trigger a new deployment in response to the first imagestream update")
	createTagEvent(imageStream.Name, imageapi.DefaultImageTag, updatedImage, updatedPullSpec)
out:
	for {
		select {
		case event := <-configWatch.ResultChan():
			if event.Type != watchapi.Modified {
				continue
			}

			newConfig := event.Object.(*deployapi.DeploymentConfig)
			if newConfig.Status.LatestVersion > 0 {
				t.Fatalf("unexpected latestVersion update: %#v", newConfig)
			}
			container := newConfig.Spec.Template.Spec.Containers[0]
			if e, a := updatedPullSpec, container.Image; e == a {
				break out
			}

		case <-timeout:
			t.Fatalf("timed out waiting for the first image update to happen")
		}
	}

	t.Log("Should trigger a new deployment in response to the second imagestream update")
	updatedImage = "sampleImage"
	updatedPullSpec = "samplePullSpec"
	createTagEvent(secondImageStream.Name, imageapi.DefaultImageTag, updatedImage, updatedPullSpec)
	for {
	inner:
		select {
		case event := <-configWatch.ResultChan():
			if event.Type != watchapi.Modified {
				continue
			}

			newConfig := event.Object.(*deployapi.DeploymentConfig)
			switch {
			case newConfig.Status.LatestVersion == 0:
				t.Logf("Wating for latestVersion to update to 1")
				break inner
			case newConfig.Status.LatestVersion > 1:
				t.Fatalf("unexpected latestVersion %d for %#v", newConfig.Status.LatestVersion, newConfig)
			}

			container := newConfig.Spec.Template.Spec.Containers[1]
			if e, a := updatedPullSpec, container.Image; e != a {
				t.Fatalf("unexpected image for pod template container %q; expected %q, got %q", container.Name, e, a)
			}

			return

		case <-timeout:
			t.Fatalf("timed out waiting for the second image update to happen")
		}
	}
}

// TestTriggers_configChange ensures that a change in the template of a deployment config with
// a config change trigger will start a new deployment.
func TestTriggers_configChange(t *testing.T) {
	const namespace = "test-triggers-configchange"

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, namespace, "my-test-user")
	if err != nil {
		t.Fatal(err)
	}
	osClient, kubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "my-test-user")
	if err != nil {
		t.Fatal(err)
	}

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = namespace
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{deploytest.OkConfigChangeTrigger()}

	rcWatch, err := kubeClient.ReplicationControllers(namespace).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer rcWatch.Stop()

	// submit the initial deployment config
	if _, err := osClient.DeploymentConfigs(namespace).Create(config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	// verify the initial deployment exists
	event := <-rcWatch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	deployment := event.Object.(*kapi.ReplicationController)

	if e, a := config.Name, deployutil.DeploymentConfigNameFor(deployment); e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}

	assertEnvVarEquals("ENV1", "VAL1", deployment, t)

	retryErr := kclient.RetryOnConflict(wait.Backoff{Steps: maxUpdateRetries}, func() error {
		// submit a new config with an updated environment variable
		config, err := osClient.DeploymentConfigs(namespace).Generate(config.Name)
		if err != nil {
			return err
		}

		config.Spec.Template.Spec.Containers[0].Env[0].Value = "UPDATED"

		// before we update the config, we need to update the state of the existing deployment
		// this is required to be done manually since the deployment and deployer pod controllers are not run in this test
		// get this live or conflicts will never end up resolved
		liveDeployment, err := kubeClient.ReplicationControllers(deployment.Namespace).Get(deployment.Name)
		if err != nil {
			return err
		}
		liveDeployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusComplete)
		// update the deployment
		if _, err := kubeClient.ReplicationControllers(namespace).Update(liveDeployment); err != nil {
			return err
		}

		event = <-rcWatch.ResultChan()
		if e, a := watchapi.Modified, event.Type; e != a {
			t.Fatalf("expected watch event type %s, got %s", e, a)
		}

		if _, err := osClient.DeploymentConfigs(namespace).Update(config); err != nil {
			return err
		}
		return nil
	})
	if retryErr != nil {
		t.Fatal(retryErr)
	}

	var newDeployment *kapi.ReplicationController
	for {
		event = <-rcWatch.ResultChan()
		if event.Type != watchapi.Added {
			// Discard modifications which could be applied to the original RC, etc.
			continue
		}
		newDeployment = event.Object.(*kapi.ReplicationController)
		break
	}

	assertEnvVarEquals("ENV1", "UPDATED", newDeployment, t)

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
}

func assertEnvVarEquals(name string, value string, deployment *kapi.ReplicationController, t *testing.T) {
	env := deployment.Spec.Template.Spec.Containers[0].Env

	for _, e := range env {
		if e.Name == name && e.Value == value {
			return
		}
	}

	t.Fatalf("Expected env var with name %s and value %s", name, value)
}

func makeStream(name, tag, dir, image string) *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: name},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				tag: {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: dir,
							Image:                image,
						},
					},
				},
			},
		},
	}
}
