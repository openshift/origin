package integration

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	oclient "github.com/openshift/origin/pkg/client"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestOAuthCertFallback(t *testing.T) {

	var (
		invalidToken = "invalid"
		noToken      = ""

		invalidCert = restclient.TLSClientConfig{
		// We have to generate this dynamically in order to have an invalid cert signed by a signer with the same name as the valid CA
		// CertData: ...,
		// KeyData:  ...,
		}
		noCert = restclient.TLSClientConfig{}

		tokenUser = "user"
		certUser  = "system:admin"

		unauthorizedError = "the server has asked for the client to provide credentials (get users ~)"
		anonymousError    = `User "system:anonymous" cannot get users at the cluster scope`
	)

	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	// Build master config
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Start server
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	adminConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validCert := adminConfig.TLSClientConfig

	validToken, err := tokencmd.RequestToken(adminConfig, nil, tokenUser, "pass")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(validToken) == 0 {
		t.Fatalf("Expected valid token, got none")
	}

	// make a client cert signed by a fake CA with the same name as the real CA.
	// this is needed to get the go client to actually send the cert to the server,
	// since the server advertises the signer name it requires
	fakecadir, err := ioutil.TempDir("", "fakeca")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer os.RemoveAll(fakecadir)
	cacerts, err := util.CertificatesFromFile(masterOptions.ServingInfo.ClientCA)
	if err != nil || len(cacerts) != 1 {
		t.Fatalf("Unexpected error or number of certs: %v, %d", err, len(cacerts))
	}
	fakeca, err := (&admin.CreateSignerCertOptions{
		CertFile:   path.Join(fakecadir, "fakeca.crt"),
		KeyFile:    path.Join(fakecadir, "fakeca.key"),
		SerialFile: path.Join(fakecadir, "fakeca.serial"),
		Name:       cacerts[0].Subject.CommonName,
		Output:     ioutil.Discard,
		Overwrite:  true,
	}).CreateSignerCert()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	clientCertConfig, err := fakeca.MakeClientCertificate(
		path.Join(fakecadir, "fakeclient.crt"),
		path.Join(fakecadir, "fakeclient.key"),
		&user.DefaultInfo{Name: "fakeuser"},
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	invalidCert.CertData, invalidCert.KeyData, err = clientCertConfig.GetPEMBytes()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for k, test := range map[string]struct {
		token         string
		cert          restclient.TLSClientConfig
		expectedUser  string
		errorExpected bool
		errorString   string
	}{
		"valid token, valid cert": {
			token:        validToken,
			cert:         validCert,
			expectedUser: tokenUser,
		},
		"valid token, invalid cert": {
			token:        validToken,
			cert:         invalidCert,
			expectedUser: tokenUser,
		},
		"valid token, no cert": {
			token:        validToken,
			cert:         noCert,
			expectedUser: tokenUser,
		},
		"invalid token, valid cert": {
			token:         invalidToken,
			cert:          validCert,
			errorExpected: true,
			errorString:   unauthorizedError,
		},
		"invalid token, invalid cert": {
			token:         invalidToken,
			cert:          invalidCert,
			errorExpected: true,
			errorString:   unauthorizedError,
		},
		"invalid token, no cert": {
			token:         invalidToken,
			cert:          noCert,
			errorExpected: true,
			errorString:   unauthorizedError,
		},
		"no token, valid cert": {
			token:        noToken,
			cert:         validCert,
			expectedUser: certUser,
		},
		"no token, invalid cert": {
			token:         noToken,
			cert:          invalidCert,
			errorExpected: true,
			errorString:   unauthorizedError,
		},
		"no token, no cert": {
			token:         noToken,
			cert:          noCert,
			errorExpected: true,
			errorString:   anonymousError,
		},
	} {
		config := *adminConfig
		config.BearerToken = test.token
		config.TLSClientConfig = test.cert
		config.CAData = adminConfig.CAData

		client := oclient.NewOrDie(&config)

		user, err := client.Users().Get("~")

		if user.Name != test.expectedUser {
			t.Errorf("%s: unexpected user %q", k, user.Name)
		}

		if test.errorExpected {
			if err == nil {
				t.Errorf("%s: expected error but got nil", k)
			} else if err.Error() != test.errorString {
				t.Errorf("%s: unexpected error string %q", k, err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("%s: unexpected error %q", k, err.Error())
			}
		}
	}

}
