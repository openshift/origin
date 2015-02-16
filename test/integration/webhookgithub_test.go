// +build integration,!no-etcd

package integration

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	requireEtcd()
}

func TestWebhookGithubPush(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	// create buildconfig
	buildConfig := mockBuildConfigParms("image", "repo", "tag")
	if _, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile("push", "pushevent.json", openshift.server.URL+openshift.whPrefix+"pushbuild/secret101/github?namespace="+testNamespace, http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}
}

func TestWebhookGithubPushWithImageTag(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	// create imagerepo
	imageRepo := &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: "imageRepo"},
		Tags:       map[string]string{"validTag": "success"},
	}
	if _, err := openshift.Client.ImageRepositories(testNamespace).Create(imageRepo); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageRepo", "validTag")

	if _, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile("push", "pushevent.json", openshift.server.URL+openshift.whPrefix+"pushbuild/secret101/github?namespace="+testNamespace, http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}

	if actual.Parameters.Strategy.DockerStrategy.BaseImage != "registry:3000/integration-test/imageRepo:success" {
		t.Errorf("Expected %s, got %s", "registry:3000/integration-test/imageRepo:success", actual.Parameters.Strategy.DockerStrategy.BaseImage)
	}
}

func TestWebhookGithubPushWithImageTagUnmatched(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	// create imagerepo
	imageRepo := &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: "imageRepo"},
		Tags:       map[string]string{"validTag": "success"},
	}
	if _, err := openshift.Client.ImageRepositories(testNamespace).Create(imageRepo); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageRepo", "invalidTag")
	if _, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile("push", "pushevent.json", openshift.server.URL+openshift.whPrefix+"pushbuild/secret101/github?namespace="+testNamespace, http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}

	if actual.Parameters.Strategy.DockerStrategy.BaseImage != "originalImage" {
		t.Errorf("Expected %s, got %s", "originalImage", actual.Parameters.Strategy.DockerStrategy.BaseImage)
	}
}

func TestWebhookGithubPushWithNamespaceUnmatched(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	// create imagerepo
	imageRepo := &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Namespace: "unmatched", Name: "imageRepo"},
		Tags:       map[string]string{"validTag": "success"},
	}
	if _, err := openshift.Client.ImageRepositories("unmatched").Create(imageRepo); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageRepo", "validTag")

	if _, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile("push", "pushevent.json", openshift.server.URL+openshift.whPrefix+"pushbuild/secret101/github?namespace="+testNamespace, http.StatusOK, t)

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if actual.Status != buildapi.BuildStatusNew {
		t.Errorf("Expected %s, got %s", buildapi.BuildStatusNew, actual.Status)
	}

	if actual.Parameters.Strategy.DockerStrategy.BaseImage != "originalImage" {
		t.Errorf("Expected %s, got %s", "originalImage", actual.Parameters.Strategy.DockerStrategy.BaseImage)
	}
}

func TestWebhookGithubPing(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	// create buildconfig
	buildConfig := mockBuildConfigParms("originalImage", "imageRepo", "validTag")
	if _, err := openshift.Client.BuildConfigs(testNamespace).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// trigger build event sending push notification
	postFile("ping", "pingevent.json", openshift.server.URL+openshift.whPrefix+"pushbuild/secret101/github?namespace="+testNamespace, http.StatusOK, t)

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

func postFile(event, filename, url string, expStatusCode int, t *testing.T) {
	client := &http.Client{}
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

func mockBuildConfigParms(imageName, imageRepo, imageTag string) *buildapi.BuildConfig {
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
						Name: imageRepo,
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
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
					BaseImage:  imageName,
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}
}
