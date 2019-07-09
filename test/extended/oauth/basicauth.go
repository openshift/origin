package oauth

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1 "github.com/openshift/api/config/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	utiloauth "github.com/openshift/origin/test/extended/util/oauthserver"
	"github.com/openshift/origin/test/extended/util/oauthserver/tokencmd"
)

const (
	configNamespace        = "openshift-config"
	managedConfigNamespace = "openshift-config-managed"
	testRouteName          = "nginx-route"

	correctLogin    = "franta"
	correctPassword = "natrabanta"
	incorrectLogin  = "pepa"

	caField = "ca"
)

type BasicAuthTestCase struct {
	TestName        string
	RemoteStatus    int32
	ResponseConfig  string
	RemoteBody      []byte
	Login           string
	Password        string
	ExpectErrStatus int
}

var failTestcases = []BasicAuthTestCase{
	{
		TestName:        "login",
		RemoteStatus:    401,
		RemoteBody:      []byte(`{"error":"bad-user"}`),
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 401,
	},
	{
		TestName:        "status-301",
		RemoteStatus:    301,
		ResponseConfig:  "return 301 http://www.example.com;",
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 500,
	},
	{
		TestName:        "status-302",
		RemoteStatus:    302,
		ResponseConfig:  "return 302 http://www.example.com;",
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 500,
	},
	{
		TestName:        "status-303",
		RemoteStatus:    303,
		ResponseConfig:  "return 303 http://www.example.com;",
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 500,
	},
	{
		TestName:        "status-304",
		RemoteStatus:    304,
		ResponseConfig:  "return 304 http://www.example.com;",
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 500,
	},
	{
		TestName:        "status-305",
		RemoteStatus:    305,
		ResponseConfig:  "return 305 http://www.example.com;",
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 500,
	},
	{
		TestName:        "status-404",
		RemoteStatus:    404,
		ResponseConfig:  "return 404;",
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 500,
	},
	{
		TestName:        "status-500",
		RemoteStatus:    500,
		ResponseConfig:  "return 500;",
		Login:           "pepa",
		Password:        "rakos",
		ExpectErrStatus: 500,
	},
}

var _ = g.Describe("[Suite:openshift/oauth/basicauthidp] BasicAuth Identity Provider:", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("cluster-basic-auth", exutil.KubeConfigPath())

		identitiesBackup = sets.NewString()
		usersBackup      = sets.NewString()

		privilegedSCCClusterRoleName string
	)

	g.BeforeEach(func() {
		privilegedSCCClusterRoleName = fmt.Sprintf("scc-privileged-user-%s", oc.Namespace())
		_, err := oc.AdminKubeClient().RbacV1().ClusterRoles().Create(
			&rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: privilegedSCCClusterRoleName,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"security.openshift.io"},
						Resources:     []string{"securitycontextconstraints"},
						ResourceNames: []string{"privileged"},
						Verbs:         []string{"use"},
					},
				},
			})
		o.Expect(err).NotTo(o.HaveOccurred())

		// allow the service acccount running deployments to use the privileged scc
		err = oc.AsAdmin().Run("adm").Args("policy", "add-role-to-user", privilegedSCCClusterRoleName, "-z", "default", "-n", oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		usersClient := oc.AdminUserClient().UserV1()
		users, err := usersClient.Users().List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		identities, err := usersClient.Identities().List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, id := range identities.Items {
			identitiesBackup.Insert(id.Name)
		}

		for _, user := range users.Items {
			usersBackup.Insert(user.Name)
		}
	})

	g.AfterEach(func() {
		usersClient := oc.AdminUserClient().UserV1()
		users, err := usersClient.Users().List(metav1.ListOptions{})

		oc.AdminKubeClient().RbacV1().ClusterRoles().Delete(privilegedSCCClusterRoleName, &metav1.DeleteOptions{})

		identities, err := usersClient.Identities().List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, id := range identities.Items {
			if !identitiesBackup.Has(id.Name) {
				oc.Run("delete").Args("identity", id.Name)
			}
		}

		for _, user := range users.Items {
			if !usersBackup.Has(user.Name) {
				oc.Run("delete").Args("user", user.Name)
			}
		}
	})

	g.It("run login tests", func() {
		nginxBasicAuthConfig := `
events { }

http {
	server {
		listen 8080;

		auth_basic "basic auth testing";
		auth_basic_user_file /etc/secret/htpasswd;

		types { } default_type "application/json";

		root /var/www;
		index index.json;

		error_page 401 /var/www/401.json;
	}
}`
		errs := runWithBasicAuthIDP(oc, nginxBasicAuthConfig, func(tokenOpts *tokencmd.RequestTokenOptions) []error {
			return testLoginWorks(oc, tokenOpts)
		})
		o.Expect(errs).To(o.HaveLen(0))
		cleanAllOfOAuthServer(oc)
	})

	g.It("run negative login tests", func() {
		nginxConfigFmt := `
		server {
			listen 8080;
			%s
		}`

		for _, tc := range failTestcases {
			config := fmt.Sprintf(nginxConfigFmt, tc.ResponseConfig)
			errs := runWithBasicAuthIDP(oc, config, func(tokenOpts *tokencmd.RequestTokenOptions) []error {
				return runFailCase(oc, tokenOpts, tc)
			})
			o.Expect(errs).To(o.HaveLen(1), fmt.Sprintf("test case: %s", tc.TestName))
			cleanAllOfOAuthServer(oc)
		}
	})
})

func testLoginWorks(oc *exutil.CLI, tokenOpts *tokencmd.RequestTokenOptions) []error {
	errs := []error{}

	origUser := oc.Username()
	defer oc.ChangeUser(origUser)

	// Test that login works
	e2e.Logf("trying to login with wrong credentials")
	_, err := utiloauth.RequestTokenForUser(tokenOpts, incorrectLogin, correctPassword)
	o.Expect(err).To(o.HaveOccurred(), "expected error while using wrong credentials")
	o.Expect(err.Error()).To(o.ContainSubstring("challenger chose not to retry the request"))

	token, err := utiloauth.RequestTokenForUser(tokenOpts, correctLogin, correctPassword)
	o.Expect(err).NotTo(o.HaveOccurred(), "expected to retrieve a token with correct credentials")

	// Check that the logged user is who we think it is
	user, err := utiloauth.GetUserForToken(oc.AdminConfig(), token, correctLogin)
	o.Expect(err).NotTo(o.HaveOccurred())
	if user.Name != correctLogin {
		errs = append(errs, fmt.Errorf("expected user to be '%s', got '%s'", correctLogin, user.Name))
	}

	return errs
}

func runFailCase(oc *exutil.CLI, tokenOpts *tokencmd.RequestTokenOptions, testCase BasicAuthTestCase) []error {
	errs := []error{}

	origUser := oc.Username()
	defer oc.ChangeUser(origUser)

	e2e.Logf("running test '%s'", testCase.TestName)

	_, err := utiloauth.RequestTokenForUser(tokenOpts, testCase.Login, testCase.Password)
	if err == nil {
		errs = append(errs, fmt.Errorf("%s: Expected error", testCase.TestName))
	} else if !strings.Contains(err.Error(), strconv.Itoa(testCase.ExpectErrStatus)) {
		errs = append(errs, fmt.Errorf("%s: Expected error status '%d' to appear in error message, got '%v'", testCase.TestName, testCase.ExpectErrStatus, err))
	}

	return errs
}

func runNginxDeployment(oc *exutil.CLI, config string) string {
	// The config for the actual proper IdP to check correct and incorrect credentials against

	// push the config into a config map
	testNamespace := oc.Namespace()
	_, err := oc.AdminKubeClient().CoreV1().ConfigMaps(testNamespace).Create(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "config",
			},
			Data: map[string]string{
				"nginx.conf": config,
			},
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	// we have config, create the depoyment that takes it
	err = oc.Run("create").Args("-f", exutil.FixturePath("testdata", "oauth_idp", "basic-auth-server.yaml")).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// TODO: wait for the route to have successfully admitted the host for the server
	route, err := oc.AdminRouteClient().RouteV1().Routes(testNamespace).Get(testRouteName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	time.Sleep(10 * time.Second)

	return route.Spec.Host
}

func runWithBasicAuthIDP(oc *exutil.CLI, nginxConfig string, testFunc func(*tokencmd.RequestTokenOptions) []error) []error {
	defer func() {
		oc.AsAdmin().Run("delete").Args("-f", exutil.FixturePath("testdata", "oauth_idp", "basic-auth-server.yaml")).Execute()
	}()

	nginxHostname := runNginxDeployment(oc, nginxConfig)
	defer oc.AdminKubeClient().CoreV1().ConfigMaps(oc.Namespace()).Delete("config", &metav1.DeleteOptions{})

	// grab the router CA
	routerCACM, err := oc.AdminKubeClient().CoreV1().ConfigMaps(managedConfigNamespace).Get("router-ca", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	routerCASecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: routerCACM.Name,
		},
		Data: map[string][]byte{"ca.crt": []byte(routerCACM.Data["ca-bundle.crt"])},
	}

	basicAuthProvider, err := utiloauth.GetRawExtensionForOsinProvider(
		&osinv1.BasicAuthPasswordIdentityProvider{
			RemoteConnectionInfo: configv1.RemoteConnectionInfo{
				URL: fmt.Sprintf("https://%s", nginxHostname),
				CA:  utiloauth.GetPathFromConfigMapSecretName(routerCASecret.Name, "ca.crt"),
			},
		})
	o.Expect(err).NotTo(o.HaveOccurred())

	basicAuthConfig := osinv1.IdentityProvider{
		Name:            "basic-auth-idp",
		UseAsChallenger: true,
		UseAsLogin:      true,
		MappingMethod:   string(configv1.MappingMethodClaim),
		Provider:        *basicAuthProvider,
	}
	tokenOpts, oauthCleanup, err := utiloauth.DeployOAuthServer(
		oc, []osinv1.IdentityProvider{basicAuthConfig}, nil, []corev1.Secret{routerCASecret},
	)
	defer oauthCleanup()
	defer oc.AdminKubeClient().CoreV1().Secrets(oc.Namespace()).Delete(routerCACM.Name, &metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return testFunc(tokenOpts)
}

func cleanAllOfOAuthServer(oc *exutil.CLI) {
	oc.AsAdmin().Run("delete").Args("all", "--selector", "app=test-oauth-server").Execute()
	oc.AsAdmin().Run("delete").Args("cm", "--selector", "app=test-oauth-server").Execute()
	oc.AsAdmin().Run("delete").Args("secret", "--selector", "app=test-oauth-server").Execute()
	oc.AsAdmin().Run("delete").Args("sa", "--selector", "app=test-oauth-server").Execute()
}
