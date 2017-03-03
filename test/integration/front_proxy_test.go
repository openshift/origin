package integration

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectapiv1 "github.com/openshift/origin/pkg/project/api/v1"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestFrontProxy(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}

	frontProxyCAPrefix := "frontproxycatest"
	proxyCertCommonName := "frontproxycerttest"
	proxyUserHeader := "X-Remote-User"
	proxyGroupHeader := "X-Remote-Group"

	certDir, err := ioutil.TempDir("", "frontproxycatestdir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(certDir)
	t.Logf("cert dir is %s\n", certDir)

	frontProxyClientCA, err := createCA(certDir, frontProxyCAPrefix)
	if err != nil {
		t.Fatal(err)
	}

	proxyCert, err := createCert(proxyCertCommonName, certDir, frontProxyCAPrefix)
	if err != nil {
		t.Fatal(err)
	}

	masterConfig.AuthConfig.RequestHeader = &api.RequestHeaderAuthenticationOptions{
		ClientCA:          frontProxyClientCA,
		ClientCommonNames: []string{proxyCertCommonName},

		// These don't get defaulted because we don't round trip config // TODO fix?
		UsernameHeaders: []string{proxyUserHeader},
		GroupHeaders:    []string{proxyGroupHeader},
	}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
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

	proxyHTTPHandler, err := newFrontProxyHandler(clusterAdminClientConfig.Host, masterConfig.ServingInfo.ClientCA, proxyUserHeader, proxyGroupHeader, proxyCert)
	if err != nil {
		t.Fatal(err)
	}
	proxyServer := httptest.NewServer(proxyHTTPHandler)
	defer proxyServer.Close()
	t.Logf("front proxy server is on %v\n", proxyServer.URL)

	w, err := clusterAdminClient.Projects().Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	listProjectsRoleName := "list-projects-role"
	if _, err := clusterAdminClient.ClusterRoles().Create(
		&authorizationapi.ClusterRole{
			ObjectMeta: kapi.ObjectMeta{Name: listProjectsRoleName},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("list").Groups(projectapi.LegacyGroupName).Resources("projects").RuleOrDie(),
			},
		},
	); err != nil {
		t.Fatal(err)
	}

	for _, username := range []string{"david", "jordan"} {
		projectName := username + "-project"
		if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, username); err != nil {
			t.Fatal(err)
		}
		waitForAdd(projectName, w, t)

		// make it so that the user can list projects without any groups
		if _, err := clusterAdminClient.ClusterRoleBindings().Create(
			&authorizationapi.ClusterRoleBinding{
				ObjectMeta: kapi.ObjectMeta{Name: username + "-clusterrolebinding"},
				Subjects: []kapi.ObjectReference{
					{Kind: authorizationapi.UserKind, Name: username},
				},
				RoleRef: kapi.ObjectReference{Name: listProjectsRoleName},
			},
		); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		name             string
		user             user.Info
		isUnauthorized   bool
		expectedProjects sets.String
	}{
		{
			name:           "empty user",
			isUnauthorized: true,
		},
		{
			name: "david can only see his project",
			user: &user.DefaultInfo{
				Name: "david",
			},
			expectedProjects: sets.NewString("david-project"),
		},
		{
			name: "david can see all projects when given cluster admin group",
			user: &user.DefaultInfo{
				Name:   "david",
				Groups: []string{bootstrappolicy.ClusterAdminGroup},
			},
			expectedProjects: sets.NewString(
				"david-project",
				"jordan-project",
				"default",
				"kube-system",
				"openshift",
				"openshift-infra",
			),
		},
	} {
		proxyHTTPHandler.setUser(test.user)

		response, err := http.Get(proxyServer.URL + "/oapi/v1/projects")
		if err != nil {
			t.Fatal(err)
		}
		data, err := ioutil.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		dataString := string(data)

		if test.isUnauthorized {
			if dataString != "Unauthorized\n" || response.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s does not have unauthorized error: %d %s", test.name, response.StatusCode, dataString)
			}
			continue
		}

		projectList := &projectapiv1.ProjectList{}
		if err := json.Unmarshal(data, projectList); err != nil {
			t.Errorf("%s failed to unmarshal project list: %v %s", test.name, err, dataString)
			continue
		}

		actualProjects := sets.NewString()
		for _, project := range projectList.Items {
			actualProjects.Insert(project.Name)
		}

		if !test.expectedProjects.Equal(actualProjects) {
			t.Errorf("%s failed to list correct projects expected %v got %v %s", test.name, test.expectedProjects.List(), actualProjects.List(), dataString)
		}
	}
}

type frontProxyHandler struct {
	proxier     *httputil.ReverseProxy
	lock        sync.Mutex
	user        user.Info
	userHeader  string
	groupHeader string
}

func (handler *frontProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Header.Del(handler.userHeader)
	r.Header.Del(handler.groupHeader)

	if handler.user != nil {
		handler.lock.Lock()
		defer handler.lock.Unlock()

		r.Header.Set(handler.userHeader, handler.user.GetName())
		for _, group := range handler.user.GetGroups() {
			r.Header.Add(handler.groupHeader, group)
		}
	}

	handler.proxier.ServeHTTP(w, r)
}

func (handler *frontProxyHandler) setUser(user user.Info) {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	handler.user = user
}

func newFrontProxyHandler(rawURL, clientCA, userHeader, groupHeader string, proxyCert *tls.Certificate) (*frontProxyHandler, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	rt, err := mutualAuthRoundTripper(clientCA, proxyCert)
	if err != nil {
		return nil, err
	}
	proxier := httputil.NewSingleHostReverseProxy(parsedURL)
	proxier.Transport = rt
	return &frontProxyHandler{
		proxier:     proxier,
		userHeader:  userHeader,
		groupHeader: groupHeader,
	}, nil
}

func mutualAuthRoundTripper(ca string, cert *tls.Certificate) (http.RoundTripper, error) {
	caBundleBytes, err := ioutil.ReadFile(ca)
	if err != nil {
		return nil, err
	}
	bundle := x509.NewCertPool()
	bundle.AppendCertsFromPEM(caBundleBytes)
	tlsConfig := &tls.Config{
		RootCAs:      bundle,
		ClientCAs:    bundle,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{*cert},
	}
	tlsConfig.BuildNameToCertificate()
	return &http.Transport{TLSClientConfig: tlsConfig}, nil
}

func createCert(commonName, certDir, caPrefix string) (*tls.Certificate, error) {
	signerCertOptions := &admin.SignerCertOptions{
		CertFile:   admin.DefaultCertFilename(certDir, caPrefix),
		KeyFile:    admin.DefaultKeyFilename(certDir, caPrefix),
		SerialFile: admin.DefaultSerialFilename(certDir, caPrefix),
	}
	clientCertOptions := &admin.CreateClientCertOptions{
		SignerCertOptions: signerCertOptions,
		CertFile:          admin.DefaultCertFilename(certDir, commonName),
		KeyFile:           admin.DefaultKeyFilename(certDir, commonName),
		ExpireDays:        crypto.DefaultCertificateLifetimeInDays,
		User:              commonName,
		Overwrite:         true,
	}
	if err := clientCertOptions.Validate(nil); err != nil {
		return nil, err
	}
	certConfig, err := clientCertOptions.CreateClientCert()
	if err != nil {
		return nil, err
	}
	certBytes, keyBytes, err := certConfig.GetPEMBytes()
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func createCA(certDir, caPrefix string) (string, error) {
	createSignerCertOptions := admin.CreateSignerCertOptions{
		CertFile:   admin.DefaultCertFilename(certDir, caPrefix),
		KeyFile:    admin.DefaultKeyFilename(certDir, caPrefix),
		SerialFile: admin.DefaultSerialFilename(certDir, caPrefix),
		ExpireDays: crypto.DefaultCACertificateLifetimeInDays,
		Name:       caPrefix,
		Overwrite:  true,
	}
	if err := createSignerCertOptions.Validate(nil); err != nil {
		return "", err
	}
	if _, err := createSignerCertOptions.CreateSignerCert(); err != nil {
		return "", err
	}
	return createSignerCertOptions.CertFile, nil
}
