package integration

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestWebhook(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unable to start master: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unable to get kubeClient: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unable to get osClient: %v", err)
	}
	clusterAdminBuildClient := buildclient.NewForConfigOrDie(clusterAdminClientConfig).Build()

	kubeClient.Core().Namespaces().Create(&kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: testutil.Namespace()},
	})

	if err := testserver.WaitForServiceAccounts(kubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")
	if _, err := clusterAdminBuildClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	tests := []struct {
		Name       string
		Payload    string
		HeaderFunc func(*http.Header)
		URLs       []string
	}{
		{
			Name:       "generic",
			Payload:    "generic/testdata/push-generic.json",
			HeaderFunc: genericHeaderFunc,
			URLs: []string{
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret200/generic",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret201/generic",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret202/generic",
			},
		},
		{
			Name:       "github",
			Payload:    "github/testdata/pushevent.json",
			HeaderFunc: githubHeaderFunc,
			URLs: []string{
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret100/github",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret102/github",
			},
		},
		{
			Name:       "gitlab",
			Payload:    "gitlab/testdata/pushevent.json",
			HeaderFunc: gitlabHeaderFunc,
			URLs: []string{
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret300/gitlab",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret301/gitlab",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret302/gitlab",
			},
		},
		{
			Name:       "bitbucket",
			Payload:    "bitbucket/testdata/pushevent.json",
			HeaderFunc: bitbucketHeaderFunc,
			URLs: []string{
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret400/bitbucket",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret401/bitbucket",
				"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret402/bitbucket",
			},
		},
	}

	for _, test := range tests {
		for _, s := range test.URLs {
			// trigger build event sending push notification
			clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			body := postFile(clusterAdminBuildClient.RESTClient(), test.HeaderFunc, test.Payload, clusterAdminClientConfig.Host+s, http.StatusOK, t)
			if len(body) == 0 {
				t.Fatalf("%s: Webhook did not return expected Build object.", test.Name)
			}

			returnedBuild := &buildapiv1.Build{}
			err = json.Unmarshal(body, returnedBuild)
			if err != nil {
				t.Fatalf("%s: Unable to unmarshal returned body into a Build object: %v", test.Name, err)
			}

			actual, err := clusterAdminBuildClient.Builds(testutil.Namespace()).Get(returnedBuild.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Created build not found in cluster: %v", err)
			}

			// There should only be one trigger on these builds.
			if actual.Spec.TriggeredBy[0].Message != returnedBuild.Spec.TriggeredBy[0].Message {
				t.Fatalf("%s: Webhook returned incorrect build.", test.Name)
			}
		}
	}
}

func TestWebhookGitHubPushWithImage(t *testing.T) {
	const registryHostname = "registry:3000"
	testutil.SetAdditionalAllowedRegistries(registryHostname)

	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminImageClient := imageclient.NewForConfigOrDie(clusterAdminClientConfig).Image()
	clusterAdminBuildClient := buildclient.NewForConfigOrDie(clusterAdminClientConfig).Build()

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
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: registryHostname + "/integration/imagestream",
			Tags: map[string]imageapi.TagReference{
				"validtag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: registryHostname + "/integration/imagestream:success",
					},
				},
			},
		},
	}
	if _, err := clusterAdminImageClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ism := &imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Tag:        "validtag",
		Image: imageapi.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: registryHostname + "/integration/imagestream:success",
		},
	}
	if _, err := clusterAdminImageClient.ImageStreamMappings(testutil.Namespace()).Create(ism); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")

	if _, err := clusterAdminBuildClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for _, s := range []string{
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret100/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret102/github",
	} {

		// trigger build event sending push notification
		body := postFile(clusterAdminBuildClient.RESTClient(), githubHeaderFunc, "github/testdata/pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)
		if len(body) == 0 {
			t.Errorf("Webhook did not return Build in body")
		}
		returnedBuild := &buildapiv1.Build{}
		err := json.Unmarshal(body, returnedBuild)
		if err != nil {
			t.Errorf("Returned body is not a v1 Build")
		}
		if returnedBuild.Spec.Strategy.DockerStrategy == nil {
			t.Errorf("Webhook returned incomplete or wrong Build")
		}

		actual, err := clusterAdminBuildClient.Builds(testutil.Namespace()).Get(returnedBuild.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Created build not found in cluster: %v", err)
		}

		// FIXME: I think the build creation is fast and in some situation we miss
		// the BuildPhaseNew here. Note that this is not a bug, in future we should
		// move this to use go routine to capture all events.
		if actual.Status.Phase != buildapi.BuildPhaseNew && actual.Status.Phase != buildapi.BuildPhasePending {
			t.Errorf("Expected %s or %s, got %s", buildapi.BuildPhaseNew, buildapi.BuildPhasePending, actual.Status.Phase)
		}

		if actual.Spec.Strategy.DockerStrategy.From.Name != "originalimage" {
			t.Errorf("Expected %s, got %s", "originalimage", actual.Spec.Strategy.DockerStrategy.From.Name)
		}

		if actual.Name != returnedBuild.Name {
			t.Errorf("Build returned in response body does not match created Build. Expected %s, got %s", actual.Name, returnedBuild.Name)
		}
	}
}

func TestWebhookGitHubPushWithImageStream(t *testing.T) {
	const registryHostname = "registry:3000"
	testutil.SetAdditionalAllowedRegistries(registryHostname)
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminImageClient := imageclient.NewForConfigOrDie(clusterAdminClientConfig).Image()
	clusterAdminBuildClient := buildclient.NewForConfigOrDie(clusterAdminClientConfig).Build()

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
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: registryHostname + "/integration/imagestream",
			Tags: map[string]imageapi.TagReference{
				"validtag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: registryHostname + "/integration/imagestream:success",
					},
				},
			},
		},
	}
	if _, err := clusterAdminImageClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ism := &imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Tag:        "validtag",
		Image: imageapi.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: registryHostname + "/integration/imagestream:success",
		},
	}
	if _, err := clusterAdminImageClient.ImageStreamMappings(testutil.Namespace()).Create(ism); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageStreamParms("originalimage", "image-stream", "validtag")

	if _, err := clusterAdminBuildClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminBuildClient.Builds(testutil.Namespace()).Watch(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	s := "/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github"

	// trigger build event sending push notification
	postFile(clusterAdminBuildClient.RESTClient(), githubHeaderFunc, "github/testdata/pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)

	var build *buildapi.Build

Loop:
	for {
		select {
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for build event")
		case event := <-watch.ResultChan():
			actual := event.Object.(*buildapi.Build)
			t.Logf("Saw build object %#v", actual)
			if actual.Status.Phase != buildapi.BuildPhasePending {
				continue
			}
			build = actual
			break Loop
		}
	}
	if build.Spec.Strategy.SourceStrategy.From.Name != registryHostname+"/integration/imagestream:success" {
		t.Errorf("Expected %s, got %s", registryHostname+"/integration/imagestream:success", build.Spec.Strategy.SourceStrategy.From.Name)
	}
}

func TestWebhookGitHubPing(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unable to start master: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unable to get kubeClient: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unable to get osClient: %v", err)
	}
	clusterAdminBuildClient := buildclient.NewForConfigOrDie(clusterAdminClientConfig).Build()

	kubeClient.Core().Namespaces().Create(&kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: testutil.Namespace()},
	})

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")
	if _, err := clusterAdminBuildClient.BuildConfigs(testutil.Namespace()).Create(buildConfig); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminBuildClient.Builds(testutil.Namespace()).Watch(metav1.ListOptions{})
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

		postFile(clusterAdminBuildClient.RESTClient(), githubHeaderFuncPing, "github/testdata/pingevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)

		// TODO: improve negative testing
		timer := time.NewTimer(time.Second * 5)
		select {
		case <-timer.C:
			// nothing should happen
		case event := <-watch.ResultChan():
			build := event.Object.(*buildapi.Build)
			t.Fatalf("Unexpected build created: %#v", build)
		}
	}
}

func postFile(client restclient.Interface, headerFunc func(*http.Header), filename, url string, expStatusCode int, t *testing.T) []byte {
	data, err := ioutil.ReadFile("../../pkg/build/webhook/" + filename)
	if err != nil {
		t.Fatalf("Failed to open %s: %v", filename, err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Error creating POST request: %v", err)
	}
	headerFunc(&req.Header)
	resp, err := client.(*restclient.RESTClient).Client.Do(req)
	if err != nil {
		t.Fatalf("Failed posting webhook: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != expStatusCode {
		t.Errorf("Wrong response code, expecting %d, got %d: %s!", expStatusCode, resp.StatusCode, string(body))
	}
	return body
}

func mockBuildConfigImageParms(imageName, imageStream, imageTag string) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
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
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret401",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret400",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret402",
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
		ObjectMeta: metav1.ObjectMeta{
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
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret401",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret400",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret402",
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

func genericHeaderFunc(header *http.Header) {
	header.Add("Content-Type", "application/json")
}

func githubHeaderFunc(header *http.Header) {
	header.Add("Content-Type", "application/json")
	header.Add("User-Agent", "GitHub-Hookshot/github")
	header.Add("X-Github-Event", "push")
}

func githubHeaderFuncPing(header *http.Header) {
	header.Add("Content-Type", "application/json")
	header.Add("User-Agent", "GitHub-Hookshot/github")
	header.Add("X-Github-Event", "ping")
}

func gitlabHeaderFunc(header *http.Header) {
	header.Add("Content-Type", "application/json")
	header.Add("X-Gitlab-Event", "Push Hook")
}

func bitbucketHeaderFunc(header *http.Header) {
	header.Add("Content-Type", "application/json")
	header.Add("X-Event-Key", "repo:push")
}
