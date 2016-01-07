// +build integration,etcd

package integration

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
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

	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	checkErr(t, err)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	checkErr(t, err)
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	checkErr(t, err)
	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, namespace, "my-test-user")
	checkErr(t, err)
	osClient, kubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "my-test-user")
	checkErr(t, err)

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

	rcWatch, err := kubeClient.ReplicationControllers(namespace).Watch(labels.Everything(), fields.Everything(), dc.ResourceVersion)
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
	if e, a := 1, deployutil.DeploymentVersionFor(deployment); e != a {
		t.Fatalf("Deployment annotation version does not match: %#v", deployment)
	}
}

func TestTriggers_imageChange(t *testing.T) {
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

	imageStream := &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "test-image-stream"}}

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()

	configWatch, err := openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer configWatch.Stop()

	if imageStream, err = openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Couldn't create ImageStream: %v", err)
	}

	imageWatch, err := openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to ImageStreams: %s", err)
	}
	defer imageWatch.Stop()

	updatedImage := "sha256:00000000000000000000000000000001"
	updatedPullSpec := fmt.Sprintf("registry:8080/openshift/test-image@%s", updatedImage)
	// Make a function which can create a new tag event for the image stream and
	// then wait for the stream status to be asynchronously updated.
	createTagEvent := func() {
		mapping := &imageapi.ImageStreamMapping{
			ObjectMeta: kapi.ObjectMeta{Name: imageStream.Name},
			Tag:        "latest",
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

		t.Log("Waiting for image stream mapping to be reflected in the IS status...")
	statusLoop:
		for {
			select {
			case event := <-imageWatch.ResultChan():
				stream := event.Object.(*imageapi.ImageStream)
				if _, ok := stream.Status.Tags["latest"]; ok {
					t.Logf("ImageStream %s now has Status with tags: %#v", stream.Name, stream.Status.Tags)
					break statusLoop
				} else {
					t.Logf("Still waiting for latest tag status on ImageStream %s", stream.Name)
				}
			}
		}
	}

	if config, err = openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Create(config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	createTagEvent()

	var newConfig *deployapi.DeploymentConfig
	t.Log("Waiting for a new deployment config in response to ImageStream update")
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
				t.Log("Still waiting for a new deployment config in response to ImageStream update")
			}
		}
	}
}

func TestTriggers_configChange(t *testing.T) {
	const namespace = "test-triggers-configchange"

	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	checkErr(t, err)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	checkErr(t, err)
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	checkErr(t, err)
	_, err = testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, namespace, "my-test-user")
	checkErr(t, err)
	osClient, kubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "my-test-user")
	checkErr(t, err)

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = namespace
	config.Spec.Triggers[0] = deploytest.OkConfigChangeTrigger()

	rcWatch, err := kubeClient.ReplicationControllers(namespace).Watch(labels.Everything(), fields.Everything(), "0")
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
