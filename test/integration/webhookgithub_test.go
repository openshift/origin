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
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestWebhookGithubPushWithImage(t *testing.T) {
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

	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	checkErr(t, err)

	if err := testutil.WaitForServiceAccounts(clusterAdminKubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
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

	ism := &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
		Tag:        "validTag",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: "registry:3000/integration/imageStream:success",
		},
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(ism); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalImage", "imageStream", "validTag")

	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	for _, s := range []string{
		"/osapi/v1beta3/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
	} {

		// trigger build event sending push notification
		postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)

		event := <-watch.ResultChan()
		actual := event.Object.(*buildapi.Build)

		// FIXME: I think the build creation is fast and in some situlation we miss
		// the BuildStatusNew here. Note that this is not a bug, in future we should
		// move this to use go routine to capture all events.
		if actual.Status != buildapi.BuildStatusNew && actual.Status != buildapi.BuildStatusPending {
			t.Errorf("Expected %s or %s, got %s", buildapi.BuildStatusNew, buildapi.BuildStatusPending, actual.Status)
		}

		if actual.Parameters.Strategy.DockerStrategy.From.Name != "originalImage" {
			t.Errorf("Expected %s, got %s", "originalImage", actual.Parameters.Strategy.DockerStrategy.From.Name)
		}
	}
}

func TestWebhookGithubPushWithImageStream(t *testing.T) {
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

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	checkErr(t, err)

	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := testutil.WaitForServiceAccounts(clusterAdminKubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
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

	ism := &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
		Tag:        "validTag",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: "registry:3000/integration/imageStream:success",
		},
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(ism); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageStreamParms("originalImage", "image-stream", "validTag")

	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	for _, s := range []string{
		"/osapi/v1beta3/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
	} {

		// trigger build event sending push notification
		postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)

		event := <-watch.ResultChan()
		actual := event.Object.(*buildapi.Build)

		// FIXME: I think the build creation is fast and in some situlation we miss
		// the BuildStatusNew here. Note that this is not a bug, in future we should
		// move this to use go routine to capture all events.
		if actual.Status != buildapi.BuildStatusNew && actual.Status != buildapi.BuildStatusPending {
			t.Errorf("Expected %s or %s, got %s", buildapi.BuildStatusNew, buildapi.BuildStatusPending, actual.Status)
		}

		if actual.Parameters.Strategy.SourceStrategy.From.Name != "registry:3000/integration/imageStream:success" {
			t.Errorf("Expected %s, got %s", "registry:3000/integration-test/imageStream:success", actual.Parameters.Strategy.SourceStrategy.From.Name)
		}
	}
}

func TestWebhookGithubPing(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	openshift.KubeClient.Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: testutil.Namespace()},
	})

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalImage", "imageStream", "validTag")
	if _, err := openshift.Client.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := openshift.Client.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	for _, s := range []string{
		openshift.whPrefix + "pushbuild/secret101/github?namespace=" + testutil.Namespace(),
		"/osapi/v1beta3/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
	} {
		// trigger build event sending push notification
		postFile(&http.Client{}, "ping", "pingevent.json", openshift.server.URL+s, http.StatusOK, t)

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

func mockBuildConfigImageParms(imageName, imageStream, imageTag string) *buildapi.BuildConfig {
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
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
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
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: imageName,
					},
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}
}

func mockBuildConfigImageStreamParms(imageName, imageStream, imageTag string) *buildapi.BuildConfig {
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
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
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
				Type: buildapi.SourceBuildStrategyType,
				SourceStrategy: &buildapi.SourceBuildStrategy{
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: imageStream + ":" + imageTag,
					},
				},
			},
			Output: buildapi.BuildOutput{
				DockerImageReference: "namespace/builtimage",
			},
		},
	}
}
