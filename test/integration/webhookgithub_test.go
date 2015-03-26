// +build integration,!no-etcd

package integration

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
)

const whPrefix = "/osapi/v1beta1/buildConfigHooks/"

func init() {
	testutil.RequireEtcd()
}

func TestWebhookGithubPush(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	// create buildconfig
	buildConfig := mockBuildConfigParms("image", "repo", "tag")
	if _, err := openshift.Client.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile(&http.Client{}, "push", "pushevent.json", openshift.server.URL+openshift.whPrefix+"pushbuild/secret101/github?namespace="+testutil.Namespace(), http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}
}

func TestWebhookGithubPushWithImageTag(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "imageStream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "registry:3000/integration/imageStream",
			Tags: map[string]imageapi.TagReference{
				"validTag": {
					DockerImageReference: "registry:3000/integration/imageStream:success",
				},
			},
		},
	}
	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageStream", "validTag")

	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+whPrefix+"pushbuild/secret101/github?namespace="+testutil.Namespace(), http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}

	if actual.Parameters.Strategy.DockerStrategy.Image != "registry:3000/integration/imageStream:success" {
		t.Errorf("Expected %s, got %s", "registry:3000/integration-test/imageStream:success", actual.Parameters.Strategy.DockerStrategy.Image)
	}
}

func TestWebhookGithubPushWithImageTagRef(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "imageStream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "registry:3000/integration/imageStream",
			Tags: map[string]imageapi.TagReference{
				"validTag": {
					DockerImageReference: "registry:3000/integration/imageStream:success",
				},
			},
		},
	}
	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigRefParms("originalImage", "imageStream", "validTag")

	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+whPrefix+"pushbuild/secret101/github?namespace="+testutil.Namespace(), http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}

	if actual.Parameters.Strategy.STIStrategy.Image != "registry:3000/integration/imageStream:success" {
		t.Errorf("Expected %s, got %s", "registry:3000/integration-test/imageStream:success", actual.Parameters.Strategy.STIStrategy.Image)
	}
}

func TestWebhookGithubPushWithImageTagUnmatched(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "imageStream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "registry:3000/integration/imageStream",
			Tags: map[string]imageapi.TagReference{
				"validTag": {
					DockerImageReference: "registry:3000/integration/imageStream:success",
				},
			},
		},
	}
	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageStream", "invalidTag")
	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+whPrefix+"pushbuild/secret101/github?namespace="+testutil.Namespace(), http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}

	if actual.Parameters.Strategy.DockerStrategy.Image != "originalImage" {
		t.Errorf("Expected %s, got %s", "originalImage", actual.Parameters.Strategy.DockerStrategy.Image)
	}
}

func TestWebhookGithubPushWithNamespaceUnmatched(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Namespace: "unmatched", Name: "imageStream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "registry:3000/integration/imageStream",
			Tags: map[string]imageapi.TagReference{
				"validTag": {
					DockerImageReference: "registry:3000/integration/imageStream:success",
				},
			},
		},
	}
	if _, err := clusterAdminClient.ImageStreams("unmatched").Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageStream", "validTag")

	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+whPrefix+"pushbuild/secret101/github?namespace="+testutil.Namespace(), http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}

	if actual.Parameters.Strategy.DockerStrategy.Image != "originalImage" {
		t.Errorf("Expected %s, got %s", "originalImage", actual.Parameters.Strategy.DockerStrategy.Image)
	}
}

func TestWebhookGithubPing(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageStream", "validTag")
	if _, err := openshift.Client.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile(&http.Client{}, "ping", "pingevent.json", openshift.server.URL+openshift.whPrefix+"pushbuild/secret101/github?namespace="+testutil.Namespace(), http.StatusOK, t)

	// TODO: improve negative testing
	timer := time.NewTimer(time.Second / 2)
	select {
	case <-timer.C:
		// nothing should happen
	case event := <-watch.ResultChan():
		build := event.Object.(*buildapi.Build)
		t.Fatalf("Unexpected build created: %#v", build)
	}
}

func postFile(client kclient.HTTPClient, event, filename, url string, expStatusCode int, t *testing.T) {
	data, err := ioutil.ReadFile("../../pkg/build/webhook/github/fixtures/" + filename)
	if err != nil {
		t.Fatalf("Failed to open %s: %v", filename, err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Error creating POST request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "GitHub-Hookshot/github")
	req.Header.Add("X-Github-Event", event)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed posting webhook: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != expStatusCode {
		t.Errorf("Wrong response code, expecting %d, got %s: %s!", expStatusCode, resp.Status, string(body))
	}
}

func mockBuildConfigParms(imageName, imageStream, imageTag string) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "pushbuild",
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.GithubWebHookBuildTriggerType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: imageName,
					From: kapi.ObjectReference{
						Name: imageStream,
					},
					Tag: imageTag,
				},
			},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.docker/build",
				},
				ContextDir: "context",
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					Image: imageName,
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}
}

func mockBuildConfigRefParms(imageName, imageStream, imageTag string) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "pushbuild",
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.GithubWebHookBuildTriggerType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					Image: imageName,
					From: kapi.ObjectReference{
						Name: imageStream,
					},
					Tag: imageTag,
				},
			},
		},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.docker/build",
				},
				ContextDir: "context",
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.STIBuildStrategyType,
				STIStrategy: &buildapi.STIBuildStrategy{
					From: &kapi.ObjectReference{
						Name: imageStream,
					},
					Tag: imageTag,
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}
}
