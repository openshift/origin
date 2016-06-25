// +build integration

package integration

import (
	"testing"

	oclient "github.com/openshift/origin/pkg/client"
	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestOAuthCertFallback(t *testing.T) {

	var (
		invalidKeyData = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAuY3YLpZsU1HNFOb6n0EoeHyRRfgbu6tXdj0EjQWR8naSm7n0
HxunYDBKf2XODaLAsWcFfvQ4m7XuECtznr3gAATv1AV00POgngQi+wGTI4qmenAh
a8gBdoWnkQYlpdTtnsHHzOU80aEmszoMFtnBC86DLgyT43e5KNUxTtSP0AYuop7x
NRDXcWd4vjByJ8S8CLmkRvQY91EJThMZDa68BgabATJKOyixKFeFW9VpJViK9rsE
R2tsLB1TuTxrH5b6c/wK6DH9hN7HVVwbC1M7UcOGQthB6+3yvqSWPbUwgosCCdaD
z1OnyaMjC5Ait1xjCZxeWDll6MPCWzU2bvRsYwIDAQABAoIBAA1a7T1lLETO9XDU
syM1QGFzrc0Yb36RdYkYGTTBOuD1sdWti6mVhvWAZExJGoyWs0HRhW6+yzhB3vGg
/wBk8DNwJ4beIatMbboR2Cay1VFQkGztlyo3ygsq0YW5qIoIClZL4kKYGUmJTMzH
l8kpQSDFa2GsHBTaMCSFO7hNylARknfm70oAgrV8j3iIAKeBB/a17wCIP78jrX4v
MroarxWr32z9dWFUANyBVNSTo+Tuq4EXslWUFBCRN30zBZ0b7YT4sM86Yt/GYo9s
It79Y0BnUI/ykgrNKGmm0gA79Su1Obh2WQWMW9KpT66KrWuzqtIr5eOgRzZkQsZ3
klGJ7kkCgYEA2TERKA84ZXDn2zgbabejGSuONoX6TjJvT594GVHzrwm8pj8FKmXS
LbbRcUhjcY8e3a4O7NznZ5hfblqjn2NhqvdvRElI9DXwQmf6dXm8lzTacSLSy8j4
W1Nvcf2RD+7J9AymOjF2vvBEnksNgaMGjr6jW3J+vEF4hiTl2plOXa0CgYEA2rWW
uu6bkWzrPRPsQ8iqbhJOqqz9d6hrmHmnKSBUCFpmAmNep4Y+PmHUKEUh8elWzBGh
NsUuc110aK9Fccy7U+NUSnOJWYNmAez2tyVbQS/SErkPgJdAvwwqQHgLFgMnD/Wy
Bm8PXLfgUWs61GZjscjmA02YpJ/HWICkvWjEFE8CgYAXjzgCNWxzrISqBfMLS604
fL4Hcg8NznC+nVjEvlwFn7PEANAJolPjO5KKjESlO9YoS8o4rVm4phGsAc7/6iLd
DcwXBzAPtY4jVe4YMiVf7Y7IePOOwXUXSvyqy8uhg9CKVZjudREhcySuWwvTBSEf
+NP1hnzy5NMzEeuRA9I5XQKBgGtWW5d6q1cAAaOEN5w8y4gh7AHPzMYBHm1Cp0uD
1joTQ6VAZ6AIPlwXXyw0Yah8QGD+9gQPWfC8mPkXrBlhxT4yf5fahDouRs4DIkJY
TyT69zrBIF6X3OrmaYYiZC51daJbjvehYgS7KZhL7B958Mu8MUbFunhxAkDpQfDD
jhf5AoGAKuT+Wc2aeHguZaMmfnAoL3pOu9FH2+QPkHEGQp6vA55QKcAlWLkYd4Me
Pxwj3BzCwiKE44kqBn36rhow9a7N177ATDJta1iUlli2J7PtTRBmM60TOpfU+3i5
UeBTTl1GjZXtgxd486Aw6BsAD/rg2C+8ZcyUba+MYzuNzY4qw6o=
-----END RSA PRIVATE KEY-----`)
		invalidCertData = []byte(`-----BEGIN CERTIFICATE-----
MIIDDTCCAfWgAwIBAgIBBzANBgkqhkiG9w0BAQsFADAmMSQwIgYDVQQDDBtvcGVu
c2hpZnQtc2lnbmVyQDE0NjY3OTI3MTAwHhcNMTYwNjI0MTgyNTExWhcNMTgwNjI0
MTgyNTEyWjA3MR4wHAYDVQQKExVzeXN0ZW06Y2x1c3Rlci1hZG1pbnMxFTATBgNV
BAMTDHN5c3RlbTphZG1pbjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
ALmN2C6WbFNRzRTm+p9BKHh8kUX4G7urV3Y9BI0FkfJ2kpu59B8bp2AwSn9lzg2i
wLFnBX70OJu17hArc5694AAE79QFdNDzoJ4EIvsBkyOKpnpwIWvIAXaFp5EGJaXU
7Z7Bx8zlPNGhJrM6DBbZwQvOgy4Mk+N3uSjVMU7Uj9AGLqKe8TUQ13FneL4wcifE
vAi5pEb0GPdRCU4TGQ2uvAYGmwEySjsosShXhVvVaSVYiva7BEdrbCwdU7k8ax+W
+nP8Cugx/YTex1VcGwtTO1HDhkLYQevt8r6klj21MIKLAgnWg89Tp8mjIwuQIrdc
YwmcXlg5ZejDwls1Nm70bGMCAwEAAaM1MDMwDgYDVR0PAQH/BAQDAgWgMBMGA1Ud
JQQMMAoGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwDQYJKoZIhvcNAQELBQADggEB
AIKO3Qcl7lsmPImzLX+KZ1h54b0qp9LRbvNe1e9+SMkyMzNhyJExs3qqg7/Z9a8n
LRqjyPPRLOFeoherM+14mnxg9BXxuhoKKZCln3hLiDgzEPZVb9vsDxMKjLy+gRiH
oIyEuzexr/dldk3shmPLDAlpB0Mz+8eBWcXn8cRwXkUyEstY64nUSuMgnL72tU9y
3Yt5D2gTLJ2MUpMxWv+cFz/UJZ0TBKtoKLjtLGRCGwFcUOz6rJzM7QGChfQxMzD7
gydO+4blIu/i4strKbkR5jcRA1WgwS/1yW25F/hE0QglsWyXJJELf4ZCNpr+9zQb
eUArEOSZyp5WmmRcn0v/YZc=
-----END CERTIFICATE-----`)

		invalidToken = "invalid"
		noToken      = ""

		invalidCert = restclient.TLSClientConfig{
			CertData: invalidCertData,
			KeyData:  invalidKeyData,
		}
		noCert = restclient.TLSClientConfig{}

		tokenUser = "user"
		certUser  = "system:admin"

		unauthorizedError = "the server has asked for the client to provide credentials (get users ~)"
		anonymousError    = `User "system:anonymous" cannot get users at the cluster scope`
	)

	// TODO see comment below
	_ = invalidCert

	testutil.RequireEtcd(t)
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

	for _, test := range []struct {
		token         string
		cert          restclient.TLSClientConfig
		expectedUser  string
		errorExpected bool
		errorString   string
		insecure      bool
	}{
		{
			token:        validToken,
			cert:         validCert,
			expectedUser: tokenUser,
		},
		// TODO find a way to test invalidCert
		// Tests with invalidCert don't work because go does not send the client cert
		// because it knows the server does not want certs that are not signed it's CA
		// {
		// 	token:        validToken,
		// 	cert:         invalidCert,
		// 	expectedUser: tokenUser,
		// 	insecure:     true,
		// },
		{
			token:        validToken,
			cert:         noCert,
			expectedUser: tokenUser,
			insecure:     true,
		},
		{
			token:         invalidToken,
			cert:          validCert,
			errorExpected: true,
			errorString:   unauthorizedError,
		},
		// {
		// 	token:         invalidToken,
		// 	cert:          invalidCert,
		// 	errorExpected: true,
		// 	errorString:   unauthorizedError,
		// 	insecure:      true,
		// },
		{
			token:         invalidToken,
			cert:          noCert,
			errorExpected: true,
			errorString:   unauthorizedError,
			insecure:      true,
		},
		{
			token:        noToken,
			cert:         validCert,
			expectedUser: certUser,
		},
		// {
		// 	token:         noToken,
		// 	cert:          invalidCert,
		// 	errorExpected: true,
		// 	errorString:   unauthorizedError,
		// 	insecure:      true,
		// },
		{
			token:         noToken,
			cert:          noCert,
			errorExpected: true,
			errorString:   anonymousError,
			insecure:      true,
		},
	} {
		config := *adminConfig
		config.BearerToken = test.token
		config.TLSClientConfig = test.cert
		config.Insecure = test.insecure

		client := oclient.NewOrDie(&config)

		user, err := client.Users().Get("~")

		test.cert = noCert // Just to make errors more readible

		if user.Name != test.expectedUser {
			t.Errorf("unexpected user %q for test %+v", user.Name, test)
		}

		if test.errorExpected {
			if err == nil {
				t.Errorf("expected error but got nil for test %+v", test)
			} else if err.Error() != test.errorString {
				t.Errorf("unexpected error string %q for test %+v", err.Error(), test)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error %q for test %+v", err.Error(), test)
			}
		}
	}

}
