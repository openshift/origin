package controller

import (
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/cert"
	kapi "k8s.io/kubernetes/pkg/api"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/serviceaccount"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func getOpenShiftClientEnvVars(options configapi.MasterConfig) ([]kapi.EnvVar, error) {
	_, kclientConfig, err := configapi.GetInternalKubeClient(
		options.MasterClients.OpenShiftLoopbackKubeConfig,
		options.MasterClients.OpenShiftLoopbackClientConnectionOverrides,
	)
	if err != nil {
		return nil, err
	}
	return clientcmd.EnvVars(
		kclientConfig.Host,
		kclientConfig.CAData,
		kclientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	), nil
}

// OpenshiftControllerConfig is the runtime (non-serializable) config object used to
// launch the set of openshift (not kube) controllers.
type OpenshiftControllerConfig struct {
	ServiceAccountTokenControllerOptions ServiceAccountTokenControllerOptions

	ServiceAccountControllerOptions ServiceAccountControllerOptions

	BuildControllerConfig BuildControllerConfig

	DeployerControllerConfig          DeployerControllerConfig
	DeploymentConfigControllerConfig  DeploymentConfigControllerConfig
	DeploymentTriggerControllerConfig DeploymentTriggerControllerConfig

	ImageTriggerControllerConfig ImageTriggerControllerConfig
	ImageImportControllerConfig  ImageImportControllerConfig

	ServiceServingCertsControllerOptions ServiceServingCertsControllerOptions

	SDNControllerConfig       SDNControllerConfig
	UnidlingControllerConfig  UnidlingControllerConfig
	IngressIPControllerConfig IngressIPControllerConfig

	ClusterQuotaReconciliationControllerConfig ClusterQuotaReconciliationControllerConfig
}

func (c *OpenshiftControllerConfig) GetControllerInitializers() (map[string]InitFunc, error) {
	ret := map[string]InitFunc{}

	ret["openshift.io/serviceaccount"] = c.ServiceAccountControllerOptions.RunController

	ret["openshift.io/serviceaccount-pull-secrets"] = RunServiceAccountPullSecretsController
	ret["openshift.io/origin-namespace"] = RunOriginNamespaceController
	ret["openshift.io/service-serving-cert"] = c.ServiceServingCertsControllerOptions.RunController

	ret["openshift.io/build"] = c.BuildControllerConfig.RunController
	ret["openshift.io/build-config-change"] = RunBuildConfigChangeController

	ret["openshift.io/deployer"] = c.DeployerControllerConfig.RunController
	ret["openshift.io/deploymentconfig"] = c.DeploymentConfigControllerConfig.RunController
	ret["openshift.io/deploymenttrigger"] = c.DeploymentTriggerControllerConfig.RunController

	ret["openshift.io/image-trigger"] = c.ImageTriggerControllerConfig.RunController
	ret["openshift.io/image-import"] = c.ImageImportControllerConfig.RunController

	ret["openshift.io/templateinstance"] = RunTemplateInstanceController

	ret["openshift.io/sdn"] = c.SDNControllerConfig.RunController
	ret["openshift.io/unidling"] = c.UnidlingControllerConfig.RunController
	ret["openshift.io/ingress-ip"] = c.IngressIPControllerConfig.RunController

	ret["openshift.io/resourcequota"] = RunResourceQuotaManager
	ret["openshift.io/cluster-quota-reconciliation"] = c.ClusterQuotaReconciliationControllerConfig.RunController

	return ret, nil
}

// NewOpenShiftControllerPreStartInitializers returns list of initializers for controllers
// that needed to be run before any other controller is started.
// Typically this has to done for the serviceaccount-token controller as it provides
// tokens to other controllers.
func (c *OpenshiftControllerConfig) ServiceAccountContentControllerInit() InitFunc {
	return c.ServiceAccountTokenControllerOptions.RunController
}

func BuildOpenshiftControllerConfig(options configapi.MasterConfig) (*OpenshiftControllerConfig, error) {
	var err error
	ret := &OpenshiftControllerConfig{}

	_, loopbackClientConfig, err := configapi.GetInternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}

	ret.ServiceAccountTokenControllerOptions = ServiceAccountTokenControllerOptions{
		RootClientBuilder: kcontroller.SimpleControllerClientBuilder{
			ClientConfig: loopbackClientConfig,
		},
	}
	if len(options.ServiceAccountConfig.PrivateKeyFile) > 0 {
		ret.ServiceAccountTokenControllerOptions.PrivateKey, err = serviceaccount.ReadPrivateKey(options.ServiceAccountConfig.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("error reading signing key for Service Account Token Manager: %v", err)
		}
	}
	if len(options.ServiceAccountConfig.MasterCA) > 0 {
		ret.ServiceAccountTokenControllerOptions.RootCA, err = ioutil.ReadFile(options.ServiceAccountConfig.MasterCA)
		if err != nil {
			return nil, fmt.Errorf("error reading master ca file for Service Account Token Manager: %s: %v", options.ServiceAccountConfig.MasterCA, err)
		}
		if _, err := cert.ParseCertsPEM(ret.ServiceAccountTokenControllerOptions.RootCA); err != nil {
			return nil, fmt.Errorf("error parsing master ca file for Service Account Token Manager: %s: %v", options.ServiceAccountConfig.MasterCA, err)
		}
	}
	if options.ControllerConfig.ServiceServingCert.Signer != nil && len(options.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
		certFile := options.ControllerConfig.ServiceServingCert.Signer.CertFile
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
		if len(ret.ServiceAccountTokenControllerOptions.RootCA) > 0 {
			ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, ret.ServiceAccountTokenControllerOptions.RootCA...)
			ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, []byte("\n")...)
		}
		ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, serviceServingCA...)
	}

	ret.ServiceAccountControllerOptions = ServiceAccountControllerOptions{
		ManagedNames: options.ServiceAccountConfig.ManagedNames,
	}

	storageVersion := options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := schema.GroupVersion{Group: "", Version: storageVersion}
	annotationCodec := kapi.Codecs.LegacyCodec(groupVersion)

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	ret.BuildControllerConfig = BuildControllerConfig{
		DockerImage:           imageTemplate.ExpandOrDie("docker-builder"),
		S2IImage:              imageTemplate.ExpandOrDie("sti-builder"),
		AdmissionPluginConfig: options.AdmissionConfig.PluginConfig,
		Codec: annotationCodec,
	}

	vars, err := getOpenShiftClientEnvVars(options)
	if err != nil {
		return nil, err
	}
	ret.DeployerControllerConfig = DeployerControllerConfig{
		ImageName:     imageTemplate.ExpandOrDie("deployer"),
		Codec:         annotationCodec,
		ClientEnvVars: vars,
	}
	ret.DeploymentConfigControllerConfig = DeploymentConfigControllerConfig{
		Codec: annotationCodec,
	}
	ret.DeploymentTriggerControllerConfig = DeploymentTriggerControllerConfig{
		Codec: annotationCodec,
	}

	ret.ImageTriggerControllerConfig = ImageTriggerControllerConfig{
		HasBuilderEnabled: options.DisabledFeatures.Has(configapi.FeatureBuilder),
		// TODO: make these consts in configapi
		HasDeploymentsEnabled:  options.DisabledFeatures.Has("triggers.image.openshift.io/deployments"),
		HasDaemonSetsEnabled:   options.DisabledFeatures.Has("triggers.image.openshift.io/daemonsets"),
		HasStatefulSetsEnabled: options.DisabledFeatures.Has("triggers.image.openshift.io/statefulsets"),
		HasCronJobsEnabled:     options.DisabledFeatures.Has("triggers.image.openshift.io/cronjobs"),
	}
	ret.ImageImportControllerConfig = ImageImportControllerConfig{
		MaxScheduledImageImportsPerMinute:          options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
		ResyncPeriod:                               10 * time.Minute,
		DisableScheduledImport:                     options.ImagePolicyConfig.DisableScheduledImport,
		ScheduledImageImportMinimumIntervalSeconds: options.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds,
	}

	ret.ServiceServingCertsControllerOptions = ServiceServingCertsControllerOptions{
		Signer: options.ControllerConfig.ServiceServingCert.Signer,
	}

	ret.SDNControllerConfig = SDNControllerConfig{
		NetworkConfig: options.NetworkConfig,
	}
	ret.UnidlingControllerConfig = UnidlingControllerConfig{
		ResyncPeriod: 2 * time.Hour,
	}
	ret.IngressIPControllerConfig = IngressIPControllerConfig{
		IngressIPSyncPeriod:  10 * time.Minute,
		IngressIPNetworkCIDR: options.NetworkConfig.IngressIPNetworkCIDR,
	}

	ret.ClusterQuotaReconciliationControllerConfig = ClusterQuotaReconciliationControllerConfig{
		DefaultResyncPeriod:            5 * time.Minute,
		DefaultReplenishmentSyncPeriod: 12 * time.Hour,
	}

	return ret, nil
}
