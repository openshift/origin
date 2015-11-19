package admin

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/util/sets"
)

func TestNodeConfigNonTLS(t *testing.T) {
	signerCert, signerKey, signerSerial := makeSignerCert(t)
	defer os.Remove(signerCert)
	defer os.Remove(signerKey)
	defer os.Remove(signerSerial)

	configDirName := executeNodeConfig([]string{"--node=my-node", "--hostnames=example.org", "--listen=http://0.0.0.0", "--certificate-authority=" + signerCert, "--signer-cert=" + signerCert, "--signer-key=" + signerKey, "--signer-serial=" + signerSerial})
	defer os.Remove(configDirName)

	configDir, err := os.Open(configDirName)
	if err != nil {
		t.Fatalf("unable to read %v", configDirName)
	}

	fileNameSlice, err := configDir.Readdirnames(0)
	if err != nil {
		t.Fatalf("unable to read %v", configDirName)
	}
	filenames := sets.NewString(fileNameSlice...)

	expectedNames := sets.NewString("master-client.crt", "master-client.key", "node.kubeconfig", "node-config.yaml", "node-registration.json", "ca.crt")
	if !filenames.HasAll(expectedNames.List()...) || !expectedNames.HasAll(filenames.List()...) {
		t.Errorf("expected %v, got %v", expectedNames.List(), filenames.List())
	}
}

func TestNodeConfigTLS(t *testing.T) {
	signerCert, signerKey, signerSerial := makeSignerCert(t)
	defer os.Remove(signerCert)
	defer os.Remove(signerKey)
	defer os.Remove(signerSerial)

	configDirName := executeNodeConfig([]string{"--node=my-node", "--hostnames=example.org", "--listen=https://0.0.0.0", "--certificate-authority=" + signerCert, "--node-client-certificate-authority=" + signerCert, "--signer-cert=" + signerCert, "--signer-key=" + signerKey, "--signer-serial=" + signerSerial})
	defer os.Remove(configDirName)

	configDir, err := os.Open(configDirName)
	if err != nil {
		t.Fatalf("unable to read %v", configDirName)
	}

	fileNameSlice, err := configDir.Readdirnames(0)
	if err != nil {
		t.Fatalf("unable to read %v", configDirName)
	}
	filenames := sets.NewString(fileNameSlice...)

	expectedNames := sets.NewString("master-client.crt", "master-client.key", "server.crt", "server.key", "node-client-ca.crt", "node.kubeconfig", "node-config.yaml", "node-registration.json", "ca.crt")
	if !filenames.HasAll(expectedNames.List()...) || !expectedNames.HasAll(filenames.List()...) {
		t.Errorf("expected %v, got %v", expectedNames.List(), filenames.List())
	}
}

func makeSignerCert(t *testing.T) (string, string, string) {
	certFile, _ := ioutil.TempFile("", "signer-cert.crt-")
	keyFile, _ := ioutil.TempFile("", "signer-key.key-")
	serialFile, _ := ioutil.TempFile("", "serial.txt-")

	options := CreateSignerCertOptions{
		CertFile:   certFile.Name(),
		KeyFile:    keyFile.Name(),
		SerialFile: serialFile.Name(),
		Name:       "unit-test-signer",
		Overwrite:  true,
	}

	if err := options.Validate(nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := options.CreateSignerCert(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	return certFile.Name(), keyFile.Name(), serialFile.Name()
}

func executeNodeConfig(args []string) string {
	configDir, _ := ioutil.TempDir("", "nodeconfig-test-")

	argsToUse := make([]string, 0, 4+len(args))
	argsToUse = append(argsToUse, "create-node-config")
	argsToUse = append(argsToUse, "--node-dir="+configDir)
	argsToUse = append(argsToUse, args...)

	root := &cobra.Command{
		Use:   "openshift",
		Short: "test",
		Long:  "",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	root.AddCommand(NewCommandNodeConfig("create-node-config", "openshift admin", ioutil.Discard))
	root.SetArgs(argsToUse)
	root.Execute()

	return configDir
}
