package builds

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"
	configv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"

	"github.com/openshift/origin/pkg/clusterversion"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][webhook]", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithPodSecurityLevel("build-webhooks", admissionapi.LevelBaseline)
	)

	g.It("TestWebhook [apigroup:build.openshift.io][apigroup:image.openshift.io]", g.Label("Size:L"), func() {
		TestWebhook(g.GinkgoT(), oc)
	})
	g.It("TestWebhookGitHubPushWithImage [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		TestWebhookGitHubPushWithImage(g.GinkgoT(), oc)
	})
	g.It("TestWebhookGitHubPushWithImageStream [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		TestWebhookGitHubPushWithImageStream(g.GinkgoT(), oc)
	})
	g.It("TestWebhookGitHubPing [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		TestWebhookGitHubPing(g.GinkgoT(), oc)
	})
})

func TestWebhook(t g.GinkgoTInterface, oc *exutil.CLI) {
	ctx := context.Background()
	clusterAdminBuildClient := oc.AdminBuildClient().BuildV1()
	clusterVersion, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "get cluster version")
	adminHTTPClient := clusterAdminBuildClient.RESTClient().(*rest.RESTClient).Client

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")
	if _, err := clusterAdminBuildClient.BuildConfigs(oc.Namespace()).Create(ctx, buildConfig, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Bug #1752581: reduce number of URLs per test case
	// OCP 4.2 tests on GCP had high flake levels because namespaces took too long to tear down
	tests := []struct {
		Name           string
		Payload        string
		HeaderFunc     func(*http.Header)
		URLs           []string
		client         *http.Client
		expectedStatus int
	}{
		{
			Name:       "generic",
			Payload:    "generic/testdata/push-generic.json",
			HeaderFunc: genericHeaderFunc,
			URLs: []string{
				"/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret200/generic",
			},
			client:         adminHTTPClient,
			expectedStatus: http.StatusOK,
		},
		{
			Name:       "github",
			Payload:    "github/testdata/pushevent.json",
			HeaderFunc: githubHeaderFunc,
			URLs: []string{
				"/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret100/github",
			},
			client:         adminHTTPClient,
			expectedStatus: http.StatusOK,
		},
		{
			Name:       "gitlab",
			Payload:    "gitlab/testdata/pushevent.json",
			HeaderFunc: gitlabHeaderFunc,
			URLs: []string{
				"/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret300/gitlab",
			},
			client:         adminHTTPClient,
			expectedStatus: http.StatusOK,
		},
		{
			Name:       "bitbucket",
			Payload:    "bitbucket/testdata/pushevent.json",
			HeaderFunc: bitbucketHeaderFunc,
			URLs: []string{
				"/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret400/bitbucket",
			},
			client:         adminHTTPClient,
			expectedStatus: http.StatusOK,
		},
		{
			// AUTH-509: Webhooks do not allow unauthenticated requests by default.
			// Test will verify that an unauthenticated request fails with 403 Forbidden.
			Name:       "unauthenticated forbidden",
			Payload:    "generic/testdata/push-generic.json",
			HeaderFunc: genericHeaderFunc,
			URLs: []string{
				"/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret200/generic",
			},
			// Need client to skip TLS verification - CI clusters have self-signed certificates.
			// Transport also needs to accept proxy information from *_PROXY environment variables.
			client: &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			},
			expectedStatus: expectedUnauthWebhookStatus(clusterVersion),
		},
	}

	for _, test := range tests {
		g.By(fmt.Sprintf("testing %s webhooks", test.Name))
		for _, s := range test.URLs {
			// trigger build event sending push notification
			clusterAdminClientConfig := oc.AdminConfig()

			g.By("executing the webhook to get the build object")
			body := postFile(test.client, test.HeaderFunc, test.Payload, clusterAdminClientConfig.Host+s, test.expectedStatus, t, oc)
			o.Expect(body).NotTo(o.BeEmpty())
			// If expected HTTP status is not 200 OK, continue as we will not receive a Build object in the response body.
			if test.expectedStatus != http.StatusOK {
				continue
			}

			g.By("Unmarshalling the build object")
			returnedBuild := &buildv1.Build{}
			err := json.Unmarshal(body, returnedBuild)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that the build exists")
			actual, err := clusterAdminBuildClient.Builds(oc.Namespace()).Get(context.Background(), returnedBuild.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking that we found the correct build")
			// There should only be one trigger on these builds.
			o.Expect(actual.Spec.TriggeredBy[0].Message).To(o.Equal(returnedBuild.Spec.TriggeredBy[0].Message))
		}
	}
}

// expectedUnauthWebhookStatus returns the exepcted HTTPS status code for unauthenticated webhook
// requests sent to a stock/unmodified OpenShift cluster.
//
// For clusters upgraded from or through 4.15, unauthenticated webhooks are allowed and should
// return code 200 (OK). Starting in OCP 4.16, unauthenticated webhooks should return code 403
// (Forbidden).
func expectedUnauthWebhookStatus(cv *configv1.ClusterVersion) int {
	if clusterversion.IsUpgradedFromMinorVersion("4.15", cv) {
		return http.StatusOK
	}
	return http.StatusForbidden
}

func TestWebhookGitHubPushWithImage(t g.GinkgoTInterface, oc *exutil.CLI) {
	const registryHostname = "registry:3000"

	clusterAdminClientConfig := oc.AdminConfig()
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	clusterAdminBuildClient := oc.AdminBuildClient().BuildV1()
	adminHTTPClient := clusterAdminBuildClient.RESTClient().(*rest.RESTClient).Client

	// create imagerepo
	imageStream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Spec: imagev1.ImageStreamSpec{
			DockerImageRepository: registryHostname + "/" + oc.Namespace() + "/imagestream",
			Tags: []imagev1.TagReference{
				{
					Name: "validtag",
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: registryHostname + "/" + oc.Namespace() + "/imagestream:success",
					},
				},
			},
		},
	}
	if _, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(context.Background(), imageStream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ism := &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Tag:        "validtag",
		Image: imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: registryHostname + "/" + oc.Namespace() + "/imagestream:success",
		},
	}
	if _, err := clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(context.Background(), ism, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")

	if _, err := clusterAdminBuildClient.BuildConfigs(oc.Namespace()).Create(context.Background(), buildConfig, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Bug #1752581: reduce number of URLs per test case
	// OCP 4.2 tests on GCP had high flake levels because namespaces took too long to tear down
	for _, s := range []string{
		"/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret100/github",
	} {

		// trigger build event sending push notification
		body := postFile(adminHTTPClient, githubHeaderFunc, "github/testdata/pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t, oc)
		if len(body) == 0 {
			t.Errorf("Webhook did not return Build in body")
		}
		returnedBuild := &buildv1.Build{}
		err := json.Unmarshal(body, returnedBuild)
		if err != nil {
			t.Errorf("Returned body is not a v1 Build")
		}
		if returnedBuild.Spec.Strategy.DockerStrategy == nil {
			t.Errorf("Webhook returned incomplete or wrong Build")
		}

		actual, err := clusterAdminBuildClient.Builds(oc.Namespace()).Get(context.Background(), returnedBuild.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Created build not found in cluster: %v", err)
		}

		// FIXME: I think the build creation is fast and in some situation we miss
		// the BuildPhaseNew here. Note that this is not a bug, in future we should
		// move this to use go routine to capture all events.
		if actual.Status.Phase != buildv1.BuildPhaseNew && actual.Status.Phase != buildv1.BuildPhasePending {
			t.Errorf("Expected %s or %s, got %s", buildv1.BuildPhaseNew, buildv1.BuildPhasePending, actual.Status.Phase)
		}

		if actual.Spec.Strategy.DockerStrategy.From.Name != "originalimage" {
			t.Errorf("Expected %s, got %s", "originalimage", actual.Spec.Strategy.DockerStrategy.From.Name)
		}
		if actual.Name != returnedBuild.Name {
			t.Errorf("Build returned in response body does not match created Build. Expected %s, got %s", actual.Name, returnedBuild.Name)
		}
	}
}

func TestWebhookGitHubPushWithImageStream(t g.GinkgoTInterface, oc *exutil.CLI) {
	const registryHostname = "registry:3000"

	clusterAdminClientConfig := oc.AdminConfig()
	clusterAdminImageClient := oc.AdminImageClient().ImageV1()
	clusterAdminBuildClient := oc.AdminBuildClient().BuildV1()
	adminHTTPClient := clusterAdminBuildClient.RESTClient().(*rest.RESTClient).Client

	// create imagerepo
	imageStream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Spec: imagev1.ImageStreamSpec{
			DockerImageRepository: registryHostname + "/" + oc.Namespace() + "/imagestream",
			Tags: []imagev1.TagReference{
				{
					Name: "validtag",
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: registryHostname + "/" + oc.Namespace() + "/imagestream:success",
					},
				},
			},
		},
	}
	if _, err := clusterAdminImageClient.ImageStreams(oc.Namespace()).Create(context.Background(), imageStream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ism := &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "image-stream"},
		Tag:        "validtag",
		Image: imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myimage",
			},
			DockerImageReference: registryHostname + "/" + oc.Namespace() + "/imagestream:success",
		},
	}
	if _, err := clusterAdminImageClient.ImageStreamMappings(oc.Namespace()).Create(context.Background(), ism, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// create buildconfig
	buildConfig := mockBuildConfigImageStreamParms("originalimage", "image-stream", "validtag")

	if _, err := clusterAdminBuildClient.BuildConfigs(oc.Namespace()).Create(context.Background(), buildConfig, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminBuildClient.Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	s := "/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github"

	// trigger build event sending push notification
	postFile(adminHTTPClient, githubHeaderFunc, "github/testdata/pushevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t, oc)

	var build *buildv1.Build

Loop:
	for {
		select {
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for build event")
		case event := <-watch.ResultChan():
			actual := event.Object.(*buildv1.Build)
			t.Logf("Saw build object %#v", actual)
			if actual.Status.Phase != buildv1.BuildPhasePending {
				continue
			}
			build = actual
			break Loop
		}
	}
	if build.Spec.Strategy.SourceStrategy.From.Name != registryHostname+"/"+oc.Namespace()+"/imagestream:success" {
		t.Errorf("Expected %s, got %s", registryHostname+"/"+oc.Namespace()+"/imagestream:success", build.Spec.Strategy.SourceStrategy.From.Name)
	}
}

func TestWebhookGitHubPing(t g.GinkgoTInterface, oc *exutil.CLI) {
	clusterAdminBuildClient := oc.AdminBuildClient().BuildV1()
	adminHTTPClient := clusterAdminBuildClient.RESTClient().(*rest.RESTClient).Client

	// create buildconfig
	buildConfig := mockBuildConfigImageParms("originalimage", "imagestream", "validtag")
	if _, err := clusterAdminBuildClient.BuildConfigs(oc.Namespace()).Create(context.Background(), buildConfig, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	watch, err := clusterAdminBuildClient.Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't subscribe to builds: %v", err)
	}
	defer watch.Stop()

	// Bug #1752581: reduce number of URLs per test case
	// OCP 4.2 tests on GCP had high flake levels because namespaces took too long to tear down
	for _, s := range []string{
		"/apis/build.openshift.io/v1/namespaces/" + oc.Namespace() + "/buildconfigs/pushbuild/webhooks/secret101/github",
	} {
		// trigger build event sending push notification
		clusterAdminClientConfig := oc.AdminConfig()

		postFile(adminHTTPClient, githubHeaderFuncPing, "github/testdata/pingevent.json", clusterAdminClientConfig.Host+s, http.StatusOK, t, oc)

		// TODO: improve negative testing
		timer := time.NewTimer(time.Second * 5)
		select {
		case <-timer.C:
			// nothing should happen
		case event := <-watch.ResultChan():
			build := event.Object.(*buildv1.Build)
			t.Fatalf("Unexpected build created: %#v", build)
		}
	}
}

func postFile(client *http.Client, headerFunc func(*http.Header), filename, url string, expStatusCode int, t g.GinkgoTInterface, oc *exutil.CLI) []byte {
	path := exutil.FixturePath("testdata", "builds", "webhook", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to open %s: %v", filename, err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Error creating POST request: %v", err)
	}
	headerFunc(&req.Header)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed posting webhook: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expStatusCode {
		t.Errorf("Wrong response code, expecting %d, got %d: %s!", expStatusCode, resp.StatusCode, string(body))
	}
	return body
}

func mockBuildConfigImageParms(imageName, imageStream, imageTag string) *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pushbuild",
		},
		Spec: buildv1.BuildConfigSpec{
			RunPolicy: buildv1.BuildRunPolicyParallel,
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: buildv1.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
				{
					Type: buildv1.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: "secret102",
					},
				},
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret401",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret400",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret402",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						URI: "http://my.docker/build",
					},
					ContextDir: "context",
				},
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						From: &corev1.ObjectReference{
							Kind: "DockerImage",
							Name: imageName,
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "namespace/builtimage",
					},
				},
			},
		},
	}
}

func mockBuildConfigImageStreamParms(imageName, imageStream, imageTag string) *buildv1.BuildConfig {
	return &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pushbuild",
		},
		Spec: buildv1.BuildConfigSpec{
			RunPolicy: buildv1.BuildRunPolicyParallel,
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: buildv1.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: "secret100",
					},
				},
				{
					Type: buildv1.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: "secret102",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildv1.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildv1.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret401",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret400",
					},
				},
				{
					Type: buildv1.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildv1.WebHookTrigger{
						Secret: "secret402",
					},
				},
			},
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Git: &buildv1.GitBuildSource{
						URI: "http://my.docker/build",
					},
					ContextDir: "context",
				},
				Strategy: buildv1.BuildStrategy{
					SourceStrategy: &buildv1.SourceBuildStrategy{
						From: corev1.ObjectReference{
							Kind: "ImageStreamTag",
							Name: imageStream + ":" + imageTag,
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
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
