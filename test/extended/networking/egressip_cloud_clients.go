package networking

import (
	"context"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	userAgent       = "e2e-cloud-network-config-controller"
	applicationPort = "ApplicationPort"
	sshPort         = "SSH"
	tcp             = "tcp"
	udp             = "udp"
	secretNamespace = "openshift-cloud-network-config-controller"
	secretName      = "cloud-credentials"
)

type cloudClient interface {
	initCloudSecret() error
	createVM(vm *vm, requestPublicIP bool) error
	deleteVM(*vm) error
	io.Closer
}

func newCloudClient(oc *exutil.CLI, platform configv1.PlatformType) (cloudClient, error) {
	switch platform {
	case configv1.GCPPlatformType:
		return newGCPCloudClient(oc)
	case configv1.AzurePlatformType:
		return newAzureCloudClient(oc)
	case configv1.AWSPlatformType:
		return newAWSCloudClient(oc)
	}
	return nil, fmt.Errorf("Invalid platform. Cannot create CloudClient for %q", platform)
}

type vm struct {
	name                    string
	id                      string
	privateIP               net.IP
	publicIP                net.IP
	ports                   map[string]protocolPort
	sshPrivateKey           string
	sshPublicKey            string
	startupScript           string
	startupScriptParameters map[string]string
	securityGroupID         string // Specifically for AWS.
	keyPairID               string // Specifially for AWS.
	publicIPID              string // Specifically for AWS.
}

type protocolPort struct {
	protocol string
	port     int
}

// readCloudSecret reads the cloud credentials from the provided secret.
func readCloudSecret(oc *exutil.CLI, secretNamespace, secretName string) (map[string][]byte, error) {
	client := oc.AsAdmin().KubeFramework().ClientSet
	secret, err := client.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error reading secret %s/%s, err: %q", secretNamespace, secretName, err)
	}
	return secret.Data, nil

}

// getWorkerProviderID returns the providerID of one of the worker nodes. This ID can then be used to extract further
// details about that node, such as the network that it resides on.
func getWorkerProviderID(oc *exutil.CLI) (string, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker=",
		Limit:         1,
	}
	nodes, err := oc.KubeClient().CoreV1().Nodes().List(context.TODO(), listOptions)
	if err != nil {
		return "", err
	}
	if len(nodes.Items) != 1 {
		return "", fmt.Errorf("expected to retrieve a single worker node, retrieved %d instead", len(nodes.Items))
	}
	node := nodes.Items[0]
	return node.Spec.ProviderID, nil
}

// generateSSHKeyPair generates an SSH keypair. It returns the private/public keys as strings or error.
// Directly taken from https://stackoverflow.com/questions/21151714/go-generate-an-ssh-public-key
func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(crand.Reader, 1024)
	if err != nil {
		return "", "", err
	}

	// generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pub))

	return pubKeyBuf.String(), privKeyBuf.String(), nil
}

// printp takes a text and a map of key/value pairs. {{key}} will be replaced with value.
func printp(text string, parameters map[string]string) string {
	for key, value := range parameters {
		text = strings.Replace(text, fmt.Sprintf("{{%s}}", key), value, -1)
	}
	return text
}
