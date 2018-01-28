package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	api "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"
)

const CreateDockerConfigSecretRecommendedName = "new-dockercfg"

var (
	createDockercfgLong = templates.LongDesc(`
    Create a new dockercfg secret

    Dockercfg secrets are used to authenticate against Docker registries.

    When using the Docker command line to push images, you can authenticate to a given registry by running
    'docker login DOCKER_REGISTRY_SERVER --username=DOCKER_USER --password=DOCKER_PASSWORD --email=DOCKER_EMAIL'.
    That produces a ~/.dockercfg file that is used by subsequent 'docker push' and 'docker pull' commands to
    authenticate to the registry.

    When creating applications, you may have a Docker registry that requires authentication.  In order for the
    nodes to pull images on your behalf, they have to have the credentials.  You can provide this information
    by creating a dockercfg secret and attaching it to your service account.`)

	createDockercfgExample = templates.Examples(`
    # Create a new .dockercfg secret:
    %[1]s SECRET --docker-server=DOCKER_REGISTRY_SERVER --docker-username=DOCKER_USER --docker-password=DOCKER_PASSWORD --docker-email=DOCKER_EMAIL

    # Create a new .dockercfg secret from an existing file:
    %[2]s SECRET path/to/.dockercfg

    # Create a new .docker/config.json secret from an existing file:
    %[2]s SECRET .dockerconfigjson=path/to/.docker/config.json

    # To add new secret to 'imagePullSecrets' for the node, or 'secrets' for builds, use:
    %[3]s SERVICE_ACCOUNT`)
)

type CreateDockerConfigOptions struct {
	SecretName       string
	RegistryLocation string
	Username         string
	Password         string
	EmailAddress     string

	SecretsInterface kcoreclient.SecretInterface

	Out io.Writer
}

// NewCmdCreateDockerConfigSecret creates a command object for making a dockercfg secret
func NewCmdCreateDockerConfigSecret(name, fullName string, f kcmdutil.Factory, out io.Writer, newSecretFullName, ocEditFullName string) *cobra.Command {
	o := &CreateDockerConfigOptions{Out: out}

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s SECRET --docker-server=DOCKER_REGISTRY_SERVER --docker-username=DOCKER_USER --docker-password=DOCKER_PASSWORD --docker-email=DOCKER_EMAIL", name),
		Short:      "Create a new dockercfg secret",
		Long:       createDockercfgLong,
		Example:    fmt.Sprintf(createDockercfgExample, fullName, newSecretFullName, ocEditFullName),
		Deprecated: "use oc create secret",
		Run: func(c *cobra.Command, args []string) {
			if err := o.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
			}

			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
			}

			if len(kcmdutil.GetFlagString(c, "output")) != 0 {
				secret, err := o.NewDockerSecret()
				kcmdutil.CheckErr(err)

				mapper, _ := f.Object()
				kcmdutil.CheckErr(f.PrintObject(c, false, mapper, secret, out))
				return
			}

			if err := o.CreateDockerSecret(); err != nil {
				kcmdutil.CheckErr(err)
			}

		},
	}

	cmd.Flags().StringVar(&o.Username, "docker-username", "", "Username for Docker registry authentication")
	cmd.Flags().StringVar(&o.Password, "docker-password", "", "Password for Docker registry authentication")
	cmd.Flags().StringVar(&o.EmailAddress, "docker-email", "", "Email for Docker registry")
	cmd.Flags().StringVar(&o.RegistryLocation, "docker-server", "https://index.docker.io/v1/", "Server location for Docker registry")
	kcmdutil.AddPrinterFlags(cmd)

	return cmd
}

func (o CreateDockerConfigOptions) CreateDockerSecret() error {
	secret, err := o.NewDockerSecret()
	if err != nil {
		return err
	}

	if _, err := o.SecretsInterface.Create(secret); err != nil {
		return err
	}

	fmt.Fprintf(o.GetOut(), "secret/%s\n", secret.Name)

	return nil
}

func (o CreateDockerConfigOptions) NewDockerSecret() (*api.Secret, error) {
	dockercfgAuth := credentialprovider.DockerConfigEntry{
		Username: o.Username,
		Password: o.Password,
		Email:    o.EmailAddress,
	}

	dockerCfg := credentialprovider.DockerConfigJson{
		Auths: map[string]credentialprovider.DockerConfigEntry{o.RegistryLocation: dockercfgAuth},
	}

	dockercfgContent, err := json.Marshal(dockerCfg)
	if err != nil {
		return nil, err
	}

	secret := &api.Secret{}
	secret.Name = o.SecretName
	secret.Type = api.SecretTypeDockerConfigJson
	secret.Data = map[string][]byte{}
	secret.Data[api.DockerConfigJsonKey] = dockercfgContent

	return secret, nil
}

func (o *CreateDockerConfigOptions) Complete(f kcmdutil.Factory, args []string) error {
	if len(args) != 1 {
		return errors.New("must have exactly one argument: secret name")
	}
	o.SecretName = args[0]

	client, err := f.ClientSet()
	if err != nil {
		return err
	}
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.SecretsInterface = client.Core().Secrets(namespace)

	return nil
}

func (o CreateDockerConfigOptions) Validate() error {
	if len(o.SecretName) == 0 {
		return errors.New("secret name must be present")
	}
	if len(o.RegistryLocation) == 0 {
		return errors.New("docker-server must be present")
	}
	if len(o.Username) == 0 {
		return errors.New("docker-username must be present")
	}
	if len(o.Password) == 0 {
		return errors.New("docker-password must be present")
	}
	if len(o.EmailAddress) == 0 {
		return errors.New("docker-email must be present")
	}
	if o.SecretsInterface == nil {
		return errors.New("secrets interface must be present")
	}

	if strings.Contains(o.Username, ":") {
		return fmt.Errorf("username '%v' is illegal because it contains a ':'", o.Username)
	}

	return nil
}

func (o CreateDockerConfigOptions) GetOut() io.Writer {
	if o.Out == nil {
		return ioutil.Discard
	}

	return o.Out
}
