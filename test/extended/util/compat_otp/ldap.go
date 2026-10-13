package compat_otp

import exutil "github.com/openshift/origin/test/extended/util"

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	app "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	testdata "github.com/openshift/origin/test/extended/util/compat_otp/testdata"
)

const (
	// This image is used for both client and server pods. Temporary repo location.
	OpenLDAPTestImage = "docker.io/mrogers950/origin-openldap-test:fedora29"
	caCertFilename    = "ca.crt"
	caKeyFilename     = "ca.key"
	caName            = "ldap CA"
	saName            = "ldap"
	// These names are in sync with those in ldapserver-deployment.yaml
	configMountName = "ldap-config"
	certMountName   = "ldap-cert"
	// Used for telling the ldap client where to mount.
	configMountPath = "/etc/openldap"
	certMountPath   = "/usr/local/etc/ldapcert"
	// Confirms slapd operation
	ldapSearchCommandFormat    = "ldapsearch -x -H ldap://%s -Z -b dc=example,dc=com cn -LLL"
	expectedLDAPClientResponse = "cn: Manager"
)

// CreateLDAPTestServer deploys an LDAP server on the service network and then confirms StartTLS connectivity with an
// ldapsearch against it. It returns the ldapserver host and the ldap CA, or an error.
func CreateLDAPTestServer(oc *exutil.CLI) (string, []byte, error) {
	deploy, ldapService, ldif, scripts := ReadLDAPServerTestData()
	certDir, err := ioutil.TempDir("", "testca")
	if err != nil {
		return "", nil, err
	}
	defer os.RemoveAll(certDir)

	if _, err := oc.AdminKubeClient().CoreV1().ConfigMaps(oc.Namespace()).Create(context.Background(), ldif, metav1.CreateOptions{}); err != nil {
		return "", nil, err
	}
	if _, err := oc.AdminKubeClient().CoreV1().ConfigMaps(oc.Namespace()).Create(context.Background(), scripts, metav1.CreateOptions{}); err != nil {
		return "", nil, err
	}
	if _, err := oc.AdminKubeClient().CoreV1().Services(oc.Namespace()).Create(context.Background(), ldapService, metav1.CreateOptions{}); err != nil {
		return "", nil, err
	}

	// Create SA.
	if _, err := oc.AdminKubeClient().CoreV1().ServiceAccounts(oc.Namespace()).Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: v1.ObjectMeta{
			Name: saName,
		},
	}, metav1.CreateOptions{}); err != nil {
		return "", nil, err
	}

	// Create CA.
	ca, err := crypto.MakeSelfSignedCA(path.Join(certDir, caCertFilename), path.Join(certDir, caKeyFilename),
		path.Join(certDir, "serial"), caName, 100)
	if err != nil {
		return "", nil, err
	}

	// Ensure that the server cert is valid for localhost and the service network hostname.
	serviceHost := ldapService.Name + "." + oc.Namespace() + ".svc"
	serverCertConfig, err := ca.MakeServerCert(sets.New("localhost", "127.0.0.1", serviceHost), 100)
	if err != nil {
		return "", nil, err
	}

	caPEM, _, err := ca.Config.GetPEMBytes()
	if err != nil {
		return "", nil, err
	}

	serverCertPEM, serverCertKeyPEM, err := serverCertConfig.GetPEMBytes()
	if err != nil {
		return "", nil, err
	}

	_, err = oc.AdminKubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name: certMountName,
		},
		Data: map[string][]byte{
			corev1.TLSCertKey:       []byte(serverCertPEM),
			corev1.TLSPrivateKeyKey: serverCertKeyPEM,
			caCertFilename:          caPEM,
		},
		Type: corev1.SecretTypeTLS,
	}, metav1.CreateOptions{})
	if err != nil {
		return "", nil, err
	}

	// Allow the openldap container to run as root and privileged. This lets us use the existing openldap server
	// container startup scripts to poplate its database using ldapi:///.
	// TODO: Turn these, (and other resources in this function) into yamls.
	err = oc.AsAdmin().Run("create").Args("role", "scc-anyuid", "--verb=use", "--resource=scc",
		"--resource-name=anyuid").Execute()
	if err != nil {
		return "", nil, err
	}
	err = oc.AsAdmin().Run("adm").Args("policy", "add-role-to-user", "scc-anyuid", "-z", "ldap",
		"--role-namespace", oc.Namespace()).Execute()
	if err != nil {
		return "", nil, err
	}

	err = oc.AsAdmin().Run("create").Args("role", "scc-priv", "--verb=use", "--resource=scc",
		"--resource-name=privileged").Execute()
	if err != nil {
		return "", nil, err
	}
	err = oc.AsAdmin().Run("adm").Args("policy", "add-role-to-user", "scc-priv", "-z", "ldap",
		"--role-namespace", oc.Namespace()).Execute()
	if err != nil {
		return "", nil, err
	}

	serverDeployment, err := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Create(context.Background(), deploy, metav1.CreateOptions{})
	if err != nil {
		return "", nil, err
	}

	// Wait for an available replica.
	err = wait.PollImmediate(1*time.Second, 5*time.Minute, func() (done bool, err error) {
		dep, getErr := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Get(context.Background(), serverDeployment.Name,
			v1.GetOptions{})
		if getErr != nil {
			return false, getErr
		}
		if dep.Status.AvailableReplicas == 0 {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("replica for %s not avaiable: %v", serverDeployment.Name, err)
	}

	// Confirm ldap server availability. Since the ldap client does not support SNI, a TLS passthrough route will not
	// work, so we need to talk to the server over the service network.
	if err := checkLDAPConn(oc, serviceHost); err != nil {
		return "", nil, err
	}

	return serviceHost, caPEM, nil
}

// Confirm that the ldapserver host is responding to ldapsearch.
func checkLDAPConn(oc *exutil.CLI, host string) error {
	compareString := expectedLDAPClientResponse
	output, err := runLDAPSearchInPod(oc, host)
	if err != nil {
		return err
	}
	if !strings.Contains(output, compareString) {
		return fmt.Errorf("ldapsearch output does not contain %s\n Output: \n%s", compareString, output)
	}
	return nil
}

// Run an ldapsearch in a pod against host.
func runLDAPSearchInPod(oc *exutil.CLI, host string) (string, error) {
	mounts, volumes := LDAPClientMounts()
	output, errs := RunOneShotCommandPod(oc, "runonce-ldapsearch-pod", OpenLDAPTestImage, fmt.Sprintf(ldapSearchCommandFormat, host), mounts, volumes, nil, 8*time.Minute)
	if len(errs) != 0 {
		return output, fmt.Errorf("errours encountered trying to run ldapsearch pod: %v", errs)
	}
	return output, nil
}

func ReadLDAPServerTestData() (*app.Deployment, *corev1.Service, *corev1.ConfigMap, *corev1.ConfigMap) {
	return resourceread.ReadDeploymentV1OrDie(testdata.MustAsset(
			"test/extended/testdata/ldap/ldapserver-deployment.yaml")),
		resourceread.ReadServiceV1OrDie(testdata.MustAsset(
			"test/extended/testdata/ldap/ldapserver-service.yaml")),
		resourceread.ReadConfigMapV1OrDie(testdata.MustAsset(
			"test/extended/testdata/ldap/ldapserver-config-cm.yaml")),
		resourceread.ReadConfigMapV1OrDie(testdata.MustAsset(
			"test/extended/testdata/ldap/ldapserver-scripts-cm.yaml"))
}

func LDAPClientMounts() ([]corev1.VolumeMount, []corev1.Volume) {
	return []corev1.VolumeMount{
			{
				Name:      configMountName,
				MountPath: configMountPath,
			},
			{
				Name:      certMountName,
				MountPath: certMountPath,
			},
		}, []corev1.Volume{
			{
				Name: certMountName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: certMountName,
					},
				},
			},
			{
				Name: configMountName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMountName,
						},
					},
				},
			},
		}
}
