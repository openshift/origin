package integration

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestWebhookGitHubPushWithImage(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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
	if err != nil {
		t.Fatal(err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "registry:3000/integration/imagestream",
			Tags: map[string]imageapi.TagReference{
				"validtag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:3000/integration/imagestream:success",
					},
				},
			},
		},
	}
	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ism := &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
		Tag:        "validtag",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: "registry:3000/integration/imagestream:success",
		},
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(ism); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")

	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	for _, s := range []string{
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret100/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret102/github",
	} {

		// trigger build event sending push notification
		postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)

		event := <-watch.ResultChan()
		actual := event.Object.(*buildapi.Build)

		// FIXME: I think the build creation is fast and in some situation we miss
		// the BuildPhaseNew here. Note that this is not a bug, in future we should
		// move this to use go routine to capture all events.
		if actual.Status.Phase != buildapi.BuildPhaseNew && actual.Status.Phase != buildapi.BuildPhasePending {
			t.Errorf("Expected %s or %s, got %s", buildapi.BuildPhaseNew, buildapi.BuildPhasePending, actual.Status.Phase)
		}

		if actual.Spec.Strategy.DockerStrategy.From.Name != "originalimage" {
			t.Errorf("Expected %s, got %s", "originalimage", actual.Spec.Strategy.DockerStrategy.From.Name)
		}
	}
}

func TestWebhookGitHubPushWithImageStream(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
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
	if err != nil {
		t.Fatal(err)
	}

	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create imagerepo
	imageStream := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "registry:3000/integration/imagestream",
			Tags: map[string]imageapi.TagReference{
				"validtag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:3000/integration/imagestream:success",
					},
				},
			},
		},
	}
	if _, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ism := &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: "image-stream"},
		Tag:        "validtag",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: "registry:3000/integration/imagestream:success",
		},
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(ism); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageStreamParms("originalimage", "image-stream", "validtag")

	if _, err := clusterAdminClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	for _, s := range []string{
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
	} {

		// trigger build event sending push notification
		postFile(clusterAdminClient.RESTClient.Client, "push", "pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)

		event := <-watch.ResultChan()
		actual := event.Object.(*buildapi.Build)

		// FIXME: I think the build creation is fast and in some situlation we miss
		// the BuildPhaseNew here. Note that this is not a bug, in future we should
		// move this to use go routine to capture all events.
		if actual.Status.Phase != buildapi.BuildPhaseNew && actual.Status.Phase != buildapi.BuildPhasePending {
			t.Errorf("Expected %s or %s, got %s", buildapi.BuildPhaseNew, buildapi.BuildPhasePending, actual.Status.Phase)
		}

		if actual.Spec.Strategy.SourceStrategy.From.Name != "registry:3000/integration/imagestream:success" {
			t.Errorf("Expected %s, got %s", "registry:3000/integration/imagestream:success", actual.Spec.Strategy.SourceStrategy.From.Name)
		}
	}
}

func TestWebhookGitHubPing(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unable to start master: %v", err)
	}

	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unable to get kubeClient: %v", err)
	}
	osClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unable to get osClient: %v", err)
	}

	kubeClient.Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: testutil.Namespace()},
	})

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")
	if _, err := osClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := osClient.Builds(testutil.Namespace()).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	for _, s := range []string{
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret100/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret102/github",
	} {
		// trigger build event sending push notification
		clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		postFile(kubeClient.RESTClient.Client, "ping", "pingevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)

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

func postFile(client restclient.HTTPClient, event, filename, url string, expStatusCode int, t *testing.T) {
	data, err := ioutil.ReadFile("../../pkg/build/webhook/github/testdata/" + filename)
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
		t.Errorf("Wrong response code, expecting %d, got %d: %s!", expStatusCode, resp.StatusCode, string(body))
	}
}

func mockBuildConfigImageParms(imageName, imageStream, imageTag string) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "pushbuild",
		},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicyParallel,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret100",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret102",
					},
				},
			},
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://my.docker/build",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: imageName,
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "namespace/builtimage",
					},
				},
			},
		},
	}
}

func mockBuildConfigImageStreamParms(imageName, imageStream, imageTag string) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "pushbuild",
		},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicyParallel,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret100",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret102",
					},
				},
			},
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://my.docker/build",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{
						From: kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: imageStream + ":" + imageTag,
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "namespace/builtimage",
					},
				},
			},
		},
	}
}
