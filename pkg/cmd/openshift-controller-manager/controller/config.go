package controller

import (
	"path"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func envVars(host string, caData []byte, insecure bool, bearerTokenFile string) []kapi.EnvVar {
	envvars := []kapi.EnvVar{
		{Name: "KUBERNETES_MASTER", Value: host},
		{Name: "OPENSHIFT_MASTER", Value: host},
	}

	if len(bearerTokenFile) > 0 {
		envvars = append(envvars, kapi.EnvVar{Name: "BEARER_TOKEN_FILE", Value: bearerTokenFile})
	}

	if len(caData) > 0 {
		envvars = append(envvars, kapi.EnvVar{Name: "OPENSHIFT_CA_DATA", Value: string(caData)})
	} else if insecure {
		envvars = append(envvars, kapi.EnvVar{Name: "OPENSHIFT_INSECURE", Value: "true"})
	}

	return envvars
}

func getOpenShiftClientEnvVars(options configapi.MasterConfig) ([]kapi.EnvVar, error) {
	_, kclientConfig, err := configapi.GetInternalKubeClient(
		options.MasterClients.OpenShiftLoopbackKubeConfig,
		options.MasterClients.OpenShiftLoopbackClientConnectionOverrides,
	)
	if err != nil {
		return nil, err
	}
	return envVars(
		kclientConfig.Host,
		kclientConfig.CAData,
		kclientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	), nil
}

// OpenshiftControllerConfig is the runtime (non-serializable) config object used to
// launch the set of openshift (not kube) controllers.
type OpenshiftControllerConfig struct {
	ServiceAccountControllerOptions ServiceAccountControllerOptions

	BuildControllerConfig BuildControllerConfig

	DeployerControllerConfig         DeployerControllerConfig
	DeploymentConfigControllerConfig DeploymentConfigControllerConfig

	ImageSignatureImportControllerConfig ImageSignatureImportControllerConfig
	ImageImportControllerConfig          ImageImportControllerConfig

	ServiceServingCertsControllerOptions ServiceServingCertsControllerOptions

	SDNControllerConfig       SDNControllerConfig
	UnidlingControllerConfig  UnidlingControllerConfig
	IngressIPControllerConfig IngressIPControllerConfig

	ClusterQuotaReconciliationControllerConfig ClusterQuotaReconciliationControllerConfig

	HorizontalPodAutoscalerControllerConfig HorizontalPodAutoscalerControllerConfig
}

func (c *OpenshiftControllerConfig) GetControllerInitializers() (map[string]InitFunc, error) {
	ret := map[string]InitFunc{}

	ret["openshift.io/serviceaccount"] = c.ServiceAccountControllerOptions.RunController

	ret["openshift.io/default-rolebindings"] = RunDefaultRoleBindingController

	ret["openshift.io/serviceaccount-pull-secrets"] = RunServiceAccountPullSecretsController
	ret["openshift.io/origin-namespace"] = RunOriginNamespaceController
	ret["openshift.io/service-serving-cert"] = c.ServiceServingCertsControllerOptions.RunController

	ret["openshift.io/build"] = c.BuildControllerConfig.RunController
	ret["openshift.io/build-config-change"] = RunBuildConfigChangeController

	ret["openshift.io/deployer"] = c.DeployerControllerConfig.RunController
	ret["openshift.io/deploymentconfig"] = c.DeploymentConfigControllerConfig.RunController

	ret["openshift.io/image-trigger"] = RunImageTriggerController
	ret["openshift.io/image-import"] = c.ImageImportControllerConfig.RunController
	ret["openshift.io/image-signature-import"] = c.ImageSignatureImportControllerConfig.RunController

	ret["openshift.io/templateinstance"] = RunTemplateInstanceController

	ret["openshift.io/sdn"] = c.SDNControllerConfig.RunController
	ret["openshift.io/unidling"] = c.UnidlingControllerConfig.RunController
	ret["openshift.io/ingress-ip"] = c.IngressIPControllerConfig.RunController

	ret["openshift.io/resourcequota"] = RunResourceQuotaManager
	ret["openshift.io/cluster-quota-reconciliation"] = c.ClusterQuotaReconciliationControllerConfig.RunController

	// overrides the Kube HPA controller config, so that we can point it at an HTTPS Heapster
	// in openshift-infra, and pass it a scale client that knows how to scale DCs
	ret["openshift.io/horizontalpodautoscaling"] = c.HorizontalPodAutoscalerControllerConfig.RunController

	return ret, nil
}

func BuildOpenshiftControllerConfig(options configapi.MasterConfig) (*OpenshiftControllerConfig, error) {
	var err error
	ret := &OpenshiftControllerConfig{}

	ret.ServiceAccountControllerOptions = ServiceAccountControllerOptions{
		ManagedNames: options.ServiceAccountConfig.ManagedNames,
	}

	storageVersion := options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := schema.GroupVersion{Group: "", Version: storageVersion}
	annotationCodec := legacyscheme.Codecs.LegacyCodec(groupVersion)

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

	ret.ImageImportControllerConfig = ImageImportControllerConfig{
		MaxScheduledImageImportsPerMinute:          options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
		ResyncPeriod:                               10 * time.Minute,
		DisableScheduledImport:                     options.ImagePolicyConfig.DisableScheduledImport,
		ScheduledImageImportMinimumIntervalSeconds: options.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds,
	}
	ret.ImageSignatureImportControllerConfig = ImageSignatureImportControllerConfig{
		ResyncPeriod:          1 * time.Hour,
		SignatureFetchTimeout: 1 * time.Minute,
		SignatureImportLimit:  3,
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

	// TODO this goes away with a truly generic autoscaler
	ret.HorizontalPodAutoscalerControllerConfig = HorizontalPodAutoscalerControllerConfig{
		HeapsterNamespace: options.PolicyConfig.OpenShiftInfrastructureNamespace,
	}

	return ret, nil
}
