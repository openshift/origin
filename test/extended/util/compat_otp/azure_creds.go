package compat_otp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"
)

const (
	AzureCredsLocationEnv = "AZURE_AUTH_LOCATION"
)

type AzureCredentials struct {
	AzureClientID       string `json:"azure_client_id,omitempty"`
	AzureClientSecret   string `json:"azure_client_secret,omitempty" sensitive:"true"`
	AzureSubscriptionID string `json:"azure_subscription_id,omitempty"`
	AzureTenantID       string `json:"azure_tenant_id,omitempty"`
	AzureResourceGroup  string `json:"azure_resourcegroup,omitempty"`
	AzureResourcePrefix string `json:"azure_resource_prefix,omitempty"`
	AzureRegion         string `json:"azure_region"`

	decoded bool
}

func NewEmptyAzureCredentials() *AzureCredentials {
	return &AzureCredentials{}
}

func (ac *AzureCredentials) String() string {
	return AzureCredentialsStructToString(*ac)
}

func (ac *AzureCredentials) GetFromClusterAndDecode(oc *exutil.CLI) error {
	if err := ac.getFromCluster(oc); err != nil {
		return fmt.Errorf("error getting credentials from the cluster: %v", err)
	}
	if err := ac.decodeLazy(); err != nil {
		return fmt.Errorf("error decoding the credentials: %v", err)
	}
	return nil
}

// SetSdkEnvVars sets some environment variables used by azure-sdk-for-go.
// See https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#environment-variables.
func (ac *AzureCredentials) SetSdkEnvVars() error {
	if err := ac.decodeLazy(); err != nil {
		return fmt.Errorf("error setting environment variables used by azure-sdk-for-go: %v", err)
	}
	return errors.Join(
		os.Setenv("AZURE_CLIENT_ID", ac.AzureClientID),
		os.Setenv("AZURE_CLIENT_SECRET", ac.AzureClientSecret),
		os.Setenv("AZURE_TENANT_ID", ac.AzureTenantID),
	)
}

func (ac *AzureCredentials) getFromCluster(oc *exutil.CLI) error {
	stdout, _, err := oc.AsAdmin().WithoutNamespace().Run("get").
		Args("secret/azure-credentials", "-n", "kube-system", "-o=jsonpath={.data}").Outputs()
	if err != nil {
		return fmt.Errorf("error getting in-cluster root credentials: %v", err)
	}

	if err = json.Unmarshal([]byte(stdout), ac); err != nil {
		return fmt.Errorf("error unmarshaling in-cluster root credentials: %v", err)
	}
	ac.decoded = false
	return nil
}

func (ac *AzureCredentials) decodeLazy() error {
	if ac.decoded {
		return nil
	}
	return ac.decode()
}

func (ac *AzureCredentials) decode() error {
	v := reflect.ValueOf(ac).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if !t.Field(i).IsExported() {
			continue
		}

		field := v.Field(i)
		for field.Kind() == reflect.Ptr {
			field = field.Elem()
		}
		if field.Kind() != reflect.String {
			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(field.String())
		if err != nil {
			return fmt.Errorf("error performing base64 decoding: %v", err)
		}
		field.SetString(string(decoded))
	}
	ac.decoded = true
	return nil
}

type AzureCredentialsFromFile struct {
	AzureClientID       string `json:"clientId,omitempty"`
	AzureClientSecret   string `json:"clientSecret,omitempty" sensitive:"true"`
	AzureSubscriptionID string `json:"subscriptionId,omitempty"`
	AzureTenantID       string `json:"tenantId,omitempty"`
}

func NewEmptyAzureCredentialsFromFile() *AzureCredentialsFromFile {
	return &AzureCredentialsFromFile{}
}

func (ac *AzureCredentialsFromFile) String() string {
	return AzureCredentialsStructToString(*ac)
}

func (ac *AzureCredentialsFromFile) LoadFromFile(filePath string) error {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading credentials file: %v", err)
	}
	fileData = bytes.ReplaceAll(fileData, []byte("azure_subscription_id"), []byte("subscriptionId"))
	fileData = bytes.ReplaceAll(fileData, []byte("azure_client_id"), []byte("clientId"))
	fileData = bytes.ReplaceAll(fileData, []byte("azure_client_secret"), []byte("clientSecret"))
	fileData = bytes.ReplaceAll(fileData, []byte("azure_tenant_id"), []byte("tenantId"))
	if err = json.Unmarshal(fileData, ac); err != nil {
		return fmt.Errorf("error unmarshaling credentials file: %v", err)
	}
	return nil
}

// SetSdkEnvVars sets some environment variables used by azure-sdk-for-go.
// See https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#environment-variables.
func (ac *AzureCredentialsFromFile) SetSdkEnvVars() error {
	return errors.Join(
		os.Setenv("AZURE_CLIENT_ID", ac.AzureClientID),
		os.Setenv("AZURE_CLIENT_SECRET", ac.AzureClientSecret),
		os.Setenv("AZURE_TENANT_ID", ac.AzureTenantID),
	)
}

func AzureCredentialsStructToString[T any](s T) string {
	var sb strings.Builder
	v := reflect.ValueOf(s)
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fieldType := t.Field(i)
		if !fieldType.IsExported() {
			continue
		}
		// Censorship
		if tag, ok := fieldType.Tag.Lookup("sensitive"); ok && tag == "true" {
			continue
		}

		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("%s: %v", fieldType.Name, v.Field(i)))
	}
	return sb.String()
}

func GetAzureCredsLocation() (string, error) {
	credsLocation := os.Getenv(AzureCredsLocationEnv)
	if len(credsLocation) == 0 {
		return "", fmt.Errorf("found empty azure credentials location. Please export %s=<path to azure.json>", AzureCredsLocationEnv)
	}
	return credsLocation, nil
}

func MustGetAzureCredsLocation() string {
	credsLocation, err := GetAzureCredsLocation()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get azure credentials location")
	return credsLocation
}
