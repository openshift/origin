package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildapiv1 "github.com/openshift/origin/pkg/build/api/v1"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestWebhookGeneric(t *testing.T) {
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

	kubeClient.Core().Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: testutil.Namespace()},
	})

	if err := testserver.WaitForServiceAccounts(kubeClient, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

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
		"/oapi/v1/namespaces/" + testutil.Namespace() + "/buildconfigs/pushbuild/webhooks/secret103/generic",
	} {
		// trigger build event sending push notification
		clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		body := postFile(osClient.RESTClient.Client, "", "../../generic/testdata/push-generic.json", clusterAdminClientConfig.Host+s, http.StatusOK, t)
		if len(body) == 0 {
			t.Fatalf("Webhook did not return expected Build object.")
		}

		returnedBuild := &buildapiv1.Build{}
		err = json.Unmarshal(body, returnedBuild)
		if err != nil {
			t.Fatalf("Unable to unmarshal returned body into a Build object.")
		}

		// TODO: improve negative testing
		timer := time.NewTimer(time.Minute * 1)
		select {
		case <-timer.C:
			t.Fatalf("Did not receive created build.")
		case event := <-watch.ResultChan():
			build := event.Object.(*buildapi.Build)
			if build.Name != returnedBuild.Name {
				t.Fatalf("Webhook returned incorrect build.")
			}
		}
	}
}
