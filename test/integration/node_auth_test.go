package integration

import (
	"net"
	"net/http"
	"strconv"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

type testRequest struct {
	Method string
	Path   string
	Result int
}

func TestNodeAuth(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	// Server config
	masterConfig, nodeConfig, adminKubeConfigFile, err := testserver.StartTestAllInOne()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cluster admin clients and client configs
	adminClient, err := testutil.GetClusterAdminKubeClient(adminKubeConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	originAdminClient, err := testutil.GetClusterAdminClient(adminKubeConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	adminConfig, err := testutil.GetClusterAdminClientConfig(adminKubeConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Client configs for lesser users
	masterKubeletClientConfig := configapi.GetKubeletClientConfig(*masterConfig)

	anonymousConfig := clientcmd.AnonymousClientConfig(adminConfig)

	badTokenConfig := clientcmd.AnonymousClientConfig(adminConfig)
	badTokenConfig.BearerToken = "bad-token"

	bobClient, _, bobConfig, err := testutil.GetClientForUser(*adminConfig, "bob")
	_, _, aliceConfig, err := testutil.GetClientForUser(*adminConfig, "alice")
	sa1Client, _, sa1Config, err := testutil.GetClientForServiceAccount(adminClient, *adminConfig, "default", "sa1")
	_, _, sa2Config, err := testutil.GetClientForServiceAccount(adminClient, *adminConfig, "default", "sa2")

	// Grant Bob system:node-reader, which should let them read metrics and stats
	addBob := &policy.RoleModificationOptions{
		RoleName:            bootstrappolicy.NodeReaderRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(originAdminClient),
		Subjects:            []kapi.ObjectReference{{Kind: "User", Name: "bob"}},
	}
	if err := addBob.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// create a scoped token for bob that is only good for getting user info
	bobUser, err := bobClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	whoamiOnlyBobToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: kapi.ObjectMeta{Name: "whoami-token-plus-some-padding-here-to-make-the-limit"},
		ClientName: origin.OpenShiftCLIClientID,
		ExpiresIn:  200,
		Scopes:     []string{scope.UserInfo},
		UserName:   bobUser.Name,
		UserUID:    string(bobUser.UID),
	}
	if _, err := originAdminClient.OAuthAccessTokens().Create(whoamiOnlyBobToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _, bobWhoamiOnlyConfig, err := testutil.GetClientForUser(*adminConfig, "bob")
	bobWhoamiOnlyConfig.BearerToken = whoamiOnlyBobToken.Name

	// Grant sa1 system:cluster-reader, which should let them read metrics and stats
	addSA1 := &policy.RoleModificationOptions{
		RoleName:            bootstrappolicy.ClusterReaderRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(originAdminClient),
		Subjects:            []kapi.ObjectReference{{Kind: "ServiceAccount", Namespace: "default", Name: "sa1"}},
	}
	if err := addSA1.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for policy cache
	if err := testutil.WaitForClusterPolicyUpdate(bobClient, "get", kapi.Resource("nodes/metrics"), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForClusterPolicyUpdate(sa1Client, "get", kapi.Resource("nodes/metrics"), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, nodePort, err := net.SplitHostPort(nodeConfig.ServingInfo.BindAddress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodePortInt, err := strconv.ParseInt(nodePort, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodeTLS := configapi.UseTLS(nodeConfig.ServingInfo)

	kubeletClientConfig := func(config *restclient.Config) *kubeletclient.KubeletClientConfig {
		return &kubeletclient.KubeletClientConfig{
			Port:            uint(nodePortInt),
			EnableHttps:     nodeTLS,
			TLSClientConfig: config.TLSClientConfig,
			BearerToken:     config.BearerToken,
		}
	}

	testCases := map[string]struct {
		KubeletClientConfig *kubeletclient.KubeletClientConfig
		Forbidden           bool
		NodeViewer          bool
		NodeAdmin           bool
	}{
		"bad token": {
			KubeletClientConfig: kubeletClientConfig(&badTokenConfig),
		},
		"anonymous": {
			KubeletClientConfig: kubeletClientConfig(&anonymousConfig),
			Forbidden:           true,
		},
		"cluster admin": {
			KubeletClientConfig: kubeletClientConfig(adminConfig),
			NodeAdmin:           true,
		},
		"master kubelet client": {
			KubeletClientConfig: masterKubeletClientConfig,
			NodeAdmin:           true,
		},
		"bob": {
			KubeletClientConfig: kubeletClientConfig(bobConfig),
			NodeViewer:          true,
		},
		// bob is normally a viewer, but when using a scoped token, he should end up denied
		"bob-scoped": {
			KubeletClientConfig: kubeletClientConfig(bobWhoamiOnlyConfig),
			Forbidden:           true,
		},
		"alice": {
			KubeletClientConfig: kubeletClientConfig(aliceConfig),
			Forbidden:           true,
		},
		"sa1": {
			KubeletClientConfig: kubeletClientConfig(sa1Config),
			NodeViewer:          true,
		},
		"sa2": {
			KubeletClientConfig: kubeletClientConfig(sa2Config),
			Forbidden:           true,
		},
	}

	for k, tc := range testCases {

		var (
			// expected result for requests a viewer should be able to make
			viewResult int
			// expected result for requests an admin should be able to make (that can actually complete with a 200 in our tests)
			adminResultOK int
			// expected result for requests an admin should be able to make (that return a 404 in this test if the authn/authz layer is completed)
			adminResultMissing int
		)
		switch {
		case tc.NodeAdmin:
			viewResult = http.StatusOK
			adminResultOK = http.StatusOK
			adminResultMissing = http.StatusNotFound
		case tc.NodeViewer:
			viewResult = http.StatusOK
			adminResultOK = http.StatusForbidden
			adminResultMissing = http.StatusForbidden
		case tc.Forbidden:
			viewResult = http.StatusForbidden
			adminResultOK = http.StatusForbidden
			adminResultMissing = http.StatusForbidden
		default:
			viewResult = http.StatusUnauthorized
			adminResultOK = http.StatusUnauthorized
			adminResultMissing = http.StatusUnauthorized
		}

		requests := []testRequest{
			// Responses to invalid paths are the same for all users
			{"GET", "/", http.StatusNotFound},
			{"GET", "/stats", http.StatusMovedPermanently}, // ServeMux redirects to the directory
			{"GET", "/logs", http.StatusMovedPermanently},  // ServeMux redirects to the directory
			{"GET", "/invalid", http.StatusNotFound},

			// viewer requests
			{"GET", "/metrics", viewResult},
			{"GET", "/stats/", viewResult},
			{"POST", "/stats/", viewResult}, // stats requests can be POSTs which contain query options

			// successful admin requests
			{"GET", "/healthz", adminResultOK},
			{"GET", "/pods", adminResultOK},
			{"GET", "/logs/", adminResultOK},

			// not found admin requests
			{"GET", "/containerLogs/mynamespace/mypod/mycontainer", adminResultMissing},
			{"POST", "/exec/mynamespace/mypod/mycontainer", adminResultMissing},
			{"POST", "/run/mynamespace/mypod/mycontainer", adminResultMissing},
			{"POST", "/attach/mynamespace/mypod/mycontainer", adminResultMissing},
			{"POST", "/portForward/mynamespace/mypod/mycontainer", adminResultMissing},

			// GET is supported in origin on /exec and /attach for backwards compatibility
			// make sure node admin permissions are required
			{"GET", "/exec/mynamespace/mypod/mycontainer", adminResultMissing},
			{"GET", "/attach/mynamespace/mypod/mycontainer", adminResultMissing},
		}

		rt, err := kubeletclient.MakeTransport(tc.KubeletClientConfig)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		for _, r := range requests {
			req, err := http.NewRequest(r.Method, "https://"+nodeConfig.NodeName+":10250"+r.Path, nil)
			if err != nil {
				t.Errorf("%s: %s: unexpected error: %v", k, r.Path, err)
				continue
			}
			resp, err := rt.RoundTrip(req)
			if err != nil {
				t.Errorf("%s: %s: unexpected error: %v", k, r.Path, err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != r.Result {
				t.Errorf("%s: token=%s %s: expected %d, got %d", k, tc.KubeletClientConfig.BearerToken, r.Path, r.Result, resp.StatusCode)
				continue
			}
		}
	}
}
