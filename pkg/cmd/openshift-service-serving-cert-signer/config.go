package openshift_service_serving_cert_signer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/origin/pkg/cmd/openshift-service-serving-cert-signer/apis/serviceservingcertsigner/v1alpha1"
)

var (
	configScheme = runtime.NewScheme()
	configCodecs = serializer.NewCodecFactory(configScheme)
)

func init() {
	v1alpha1.AddToScheme(configScheme)
}

func readConfig(filename string) (*v1alpha1.ServiceServingCertSignerConfig, error) {
	config := &v1alpha1.ServiceServingCertSignerConfig{}
	if err := readYAMLFileInto(filename, config); err != nil {
		return nil, err
	}
	return config, nil
}

func readAndResolveConfig(filename string) (*v1alpha1.ServiceServingCertSignerConfig, error) {
	config, err := readConfig(filename)
	if err != nil {
		return nil, err
	}

	if err := resolveConfigPaths(config, path.Dir(filename)); err != nil {
		return nil, err
	}

	return config, nil
}

func readYAMLInto(data []byte, obj runtime.Object) error {
	data, err := kyaml.ToJSON(data)
	if err != nil {
		return err
	}
	err = runtime.DecodeInto(configCodecs.LegacyCodec(v1alpha1.SchemeGroupVersion), data, obj)
	return captureSurroundingJSONForError("error reading config: ", data, err)
}

func readYAMLFileInto(filename string, obj runtime.Object) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = readYAMLInto(data, obj)
	if err != nil {
		return fmt.Errorf("could not load config file %q due to an error: %v", filename, err)
	}
	return nil
}

// TODO: we ultimately want a better decoder for JSON that allows us exact line numbers and better
// surrounding text description. This should be removed / replaced when that happens.
func captureSurroundingJSONForError(prefix string, data []byte, err error) error {
	if syntaxErr, ok := err.(*json.SyntaxError); err != nil && ok {
		offset := syntaxErr.Offset
		begin := offset - 20
		if begin < 0 {
			begin = 0
		}
		end := offset + 20
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		return fmt.Errorf("%s%v (found near '%s')", prefix, err, string(data[begin:end]))
	}
	if err != nil {
		return fmt.Errorf("%s%v", prefix, err)
	}
	return err
}

func resolveConfigPaths(config *v1alpha1.ServiceServingCertSignerConfig, base string) error {
	return resolvePaths(getFileReferences(config), base)
}

func getFileReferences(config *v1alpha1.ServiceServingCertSignerConfig) []*string {
	refs := []*string{}

	refs = append(refs, &config.ServingInfo.CertInfo.CertFile)
	refs = append(refs, &config.ServingInfo.CertInfo.KeyFile)
	refs = append(refs, &config.ServingInfo.ClientCA)
	for i := range config.ServingInfo.NamedCertificates {
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].CertFile)
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].KeyFile)
	}
	refs = append(refs, &config.Signer.CertFile)
	refs = append(refs, &config.Signer.KeyFile)

	return refs
}

// resolvePaths updates the given refs to be absolute paths, relative to the given base directory
func resolvePaths(refs []*string, base string) error {
	for _, ref := range refs {
		// Don't resolve empty paths
		if len(*ref) > 0 {
			// Don't resolve absolute paths
			if !filepath.IsAbs(*ref) {
				*ref = filepath.Join(base, *ref)
			}
		}
	}
	return nil
}

type ClientConnectionOverrides struct {
	// AcceptContentTypes defines the Accept header sent by clients when connecting to a server, overriding the
	// default value of 'application/json'. This field will control all connections to the server used by a particular
	// client.
	AcceptContentTypes string
	// ContentType is the content type used when sending data to the server from this client.
	ContentType string

	// QPS controls the number of queries per second allowed for this connection.
	QPS float32
	// Burst allows extra queries to accumulate when a client is exceeding its rate.
	Burst int32
}

var defaultConnectionOverrides = &ClientConnectionOverrides{
	AcceptContentTypes: "application/vnd.kubernetes.protobuf,application/json",
	ContentType:        "application/vnd.kubernetes.protobuf",
	QPS:                50,
	Burst:              100,
}

// getKubeConfigOrInClusterConfig loads in-cluster config if kubeConfigFile is empty or the file if not,
// then applies overrides.
func getKubeConfigOrInClusterConfig(kubeConfigFile string, overrides *ClientConnectionOverrides) (*rest.Config, error) {
	if len(kubeConfigFile) > 0 {
		return getClientConfig(kubeConfigFile, overrides)
	}

	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	applyClientConnectionOverrides(overrides, clientConfig)
	clientConfig.WrapTransport = defaultClientTransport

	return clientConfig, nil
}

func getClientConfig(kubeConfigFile string, overrides *ClientConnectionOverrides) (*rest.Config, error) {
	kubeConfigBytes, err := ioutil.ReadFile(kubeConfigFile)
	if err != nil {
		return nil, err
	}
	kubeConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfigBytes)
	if err != nil {
		return nil, err
	}
	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	applyClientConnectionOverrides(overrides, clientConfig)
	clientConfig.WrapTransport = defaultClientTransport

	return clientConfig, nil
}

// applyClientConnectionOverrides updates a kubeConfig with the overrides from the config.
func applyClientConnectionOverrides(overrides *ClientConnectionOverrides, kubeConfig *rest.Config) {
	if overrides == nil {
		return
	}
	kubeConfig.QPS = overrides.QPS
	kubeConfig.Burst = int(overrides.Burst)
	kubeConfig.ContentConfig.AcceptContentTypes = overrides.AcceptContentTypes
	kubeConfig.ContentConfig.ContentType = overrides.ContentType
}

// defaultClientTransport sets defaults for a client Transport that are suitable
// for use by infrastructure components.
func defaultClientTransport(rt http.RoundTripper) http.RoundTripper {
	transport, ok := rt.(*http.Transport)
	if !ok {
		return rt
	}

	// TODO: this should be configured by the caller, not in this method.
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport.Dial = dialer.Dial
	// Hold open more internal idle connections
	// TODO: this should be configured by the caller, not in this method.
	transport.MaxIdleConnsPerHost = 100
	return transport
}
