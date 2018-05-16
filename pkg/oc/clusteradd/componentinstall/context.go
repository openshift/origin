package componentinstall

import (
	"io/ioutil"
	"path"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubeapiserver"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
)

const adminKubeConfigFileName = "admin.kubeconfig"

type Context interface {
	// ClusterAdminClientConfig is the cluster admin client configuration components can use to make their client.
	ClusterAdminClientConfig() *restclient.Config

	// BaseDir is the base directory that component should use to store files/logs/etc.
	BaseDir() string
	ClientImage() string
	// ImageFormat provides information about the image pull spec format. This is handy when trying to use different registries or image names.
	ImageFormat() string

	// ComponentLogLevel provides information about verbosity the component should log the messages.
	ComponentLogLevel() int

	// ImagePullPolicy provides information about what pull policy for images should be used. This is usually based on the presence of the `--tag`
	// flag which in that case the pull policy will be IfNotExists instead of Always. That allows local development without pulling the images.
	ImagePullPolicy() string
}

type installContext struct {
	restConfig  *restclient.Config
	clientImage string
	imageFormat string
	baseDir     string
	pullPolicy  string
	logLevel    int
}

// ImageFormat returns the format of the images to use when running commands like 'oc adm'
func (c *installContext) ImageFormat() string {
	return c.imageFormat
}

// ComponentLogLevel tells what log level user desire for the component
func (c *installContext) ComponentLogLevel() int {
	return c.logLevel
}

// ClusterAdminClientConfig provides a cluster admin REST client config
func (c *installContext) ClusterAdminClientConfig() *restclient.Config {
	return c.restConfig
}

// BaseDir provides the base directory with all configuration files
func (c *installContext) BaseDir() string {
	return c.baseDir
}

// ClientImage returns the name of the Docker image that provide the 'oc' binary
func (c *installContext) ClientImage() string {
	return c.clientImage
}

func (c *installContext) ImagePullPolicy() string {
	return c.pullPolicy
}

func NewComponentInstallContext(clientImageName, imageFormat, pullPolicy, baseDir string, logLevel int) (Context, error) {
	clusterAdminConfigBytes, err := ioutil.ReadFile(path.Join(baseDir, kubeapiserver.KubeAPIServerDirName, adminKubeConfigFileName))
	if err != nil {
		return nil, err
	}
	restConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminConfigBytes)
	if err != nil {
		return nil, err
	}
	return &installContext{
		restConfig:  restConfig,
		clientImage: clientImageName,
		baseDir:     baseDir,
		logLevel:    logLevel,
		imageFormat: imageFormat,
		pullPolicy:  pullPolicy,
	}, nil
}
