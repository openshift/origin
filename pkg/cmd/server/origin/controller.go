package origin

import (
	"fmt"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/cert"
	kapi "k8s.io/kubernetes/pkg/api"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/origin/controller"
)

// NewOpenShiftControllerPreStartInitializers returns list of initializers for controllers
// that needed to be run before any other controller is started.
// Typically this has to done for the serviceaccount-token controller as it provides
// tokens to other controllers.
func (c *MasterConfig) NewOpenShiftControllerPreStartInitializers() (map[string]controller.InitFunc, error) {
	ret := map[string]controller.InitFunc{}

	saToken := controller.ServiceAccountTokenControllerOptions{
		RootClientBuilder: kubecontroller.SimpleControllerClientBuilder{
			ClientConfig: &c.PrivilegedLoopbackClientConfig,
		},
	}

	if len(c.Options.ServiceAccountConfig.PrivateKeyFile) == 0 {
		glog.Infof("Skipped starting Service Account Token Manager, no private key specified")
		return nil, nil
	}

	var err error

	saToken.PrivateKey, err = serviceaccount.ReadPrivateKey(c.Options.ServiceAccountConfig.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("error reading signing key for Service Account Token Manager: %v", err)
	}

	if len(c.Options.ServiceAccountConfig.MasterCA) > 0 {
		saToken.RootCA, err = ioutil.ReadFile(c.Options.ServiceAccountConfig.MasterCA)
		if err != nil {
			return nil, fmt.Errorf("error reading master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
		if _, err := cert.ParseCertsPEM(saToken.RootCA); err != nil {
			return nil, fmt.Errorf("error parsing master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
	}

	if c.Options.ControllerConfig.ServiceServingCert.Signer != nil && len(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
		certFile := c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile
		serviceServingCA, err := ioutil.ReadFile(certFile)
		if err != nil {
			return nil, fmt.Errorf("error reading ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}
		if _, err := crypto.CertsFromPEM(serviceServingCA); err != nil {
			return nil, fmt.Errorf("error parsing ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}

		// if we have a rootCA bundle add that too.  The rootCA will be used when hitting the default master service, since those are signed
		// using a different CA by default.  The rootCA's key is more closely guarded than ours and if it is compromised, that power could
		// be used to change the trusted signers for every pod anyway, so we're already effectively trusting it.
		if len(saToken.RootCA) > 0 {
			saToken.ServiceServingCA = append(saToken.ServiceServingCA, saToken.RootCA...)
			saToken.ServiceServingCA = append(saToken.ServiceServingCA, []byte("\n")...)
		}
		saToken.ServiceServingCA = append(saToken.ServiceServingCA, serviceServingCA...)
	}
	// this matches the upstream name
	ret["serviceaccount-token"] = saToken.RunController

	return ret, nil
}

func (c *MasterConfig) NewOpenshiftControllerInitializers() (map[string]controller.InitFunc, error) {
	ret := map[string]controller.InitFunc{}

	// TODO this overrides an upstream controller, so move this to where we initialize upstream controllers
	serviceAccount := controller.ServiceAccountControllerOptions{
		ManagedNames: c.Options.ServiceAccountConfig.ManagedNames,
	}
	ret["serviceaccount"] = serviceAccount.RunController

	ret["openshift.io/serviceaccount-pull-secrets"] = controller.RunServiceAccountPullSecretsController
	ret["openshift.io/origin-namespace"] = controller.RunOriginNamespaceController

	// initialize build controller
	storageVersion := c.Options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := schema.GroupVersion{Group: "", Version: storageVersion}
	// TODO: add codec to the controller context
	codec := kapi.Codecs.LegacyCodec(groupVersion)

	buildControllerConfig := controller.BuildControllerConfig{
		DockerImage:           c.ImageFor("docker-builder"),
		STIImage:              c.ImageFor("sti-builder"),
		AdmissionPluginConfig: c.Options.AdmissionConfig.PluginConfig,
		Codec: codec,
	}

	ret["openshift.io/build"] = buildControllerConfig.RunController
	ret["openshift.io/build-config-change"] = controller.RunBuildConfigChangeController

	// initialize apps.openshift.io controllers
	vars, err := c.GetOpenShiftClientEnvVars()
	if err != nil {
		return nil, err
	}
	deployer := controller.DeployerControllerConfig{ImageName: c.ImageFor("deployer"), Codec: codec, ClientEnvVars: vars}
	ret["openshift.io/deployer"] = deployer.RunController

	deploymentConfig := controller.DeploymentConfigControllerConfig{Codec: codec}
	ret["openshift.io/deploymentconfig"] = deploymentConfig.RunController

	deploymentTrigger := controller.DeploymentTriggerControllerConfig{Codec: codec}
	ret["openshift.io/deploymenttrigger"] = deploymentTrigger.RunController

	templateInstance := controller.TemplateInstanceControllerConfig{}
	ret["openshift.io/templateinstance"] = templateInstance.RunController

	return ret, nil
}
