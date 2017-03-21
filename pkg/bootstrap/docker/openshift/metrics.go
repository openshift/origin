package openshift

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	infraNamespace         = "openshift-infra"
	svcMetrics             = "hawkular-metrics"
	metricsDeployerSA      = "metrics-deployer"
	metricsDeployerSecret  = "metrics-deployer"
	metricsDeployerJobName = "metrics-deployer-pod"
)

// InstallMetrics checks whether metrics is installed and installs it if not already installed
func (h *Helper) InstallMetrics(f *clientcmd.Factory, hostName, imagePrefix, imageVersion string) error {
	osClient, kubeClient, err := f.Clients()
	if err != nil {
		return errors.NewError("cannot obtain API clients").WithCause(err).WithDetails(h.OriginLog())
	}

	_, err = kubeClient.Core().Services(infraNamespace).Get(svcMetrics)
	if err == nil {
		// If there's no error, the metrics service already exists
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving metrics service").WithCause(err).WithDetails(h.OriginLog())
	}

	// Create metrics deployer service account
	routerSA := &kapi.ServiceAccount{}
	routerSA.Name = metricsDeployerSA
	_, err = kubeClient.Core().ServiceAccounts(infraNamespace).Create(routerSA)
	if err != nil {
		return errors.NewError("cannot create metrics deployer service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add edit role to deployer service account
	if err = AddRoleToServiceAccount(osClient, "edit", metricsDeployerSA, infraNamespace); err != nil {
		return errors.NewError("cannot add edit role to metrics deployer service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add view role to the hawkular service account
	if err = AddRoleToServiceAccount(osClient, "view", "hawkular", infraNamespace); err != nil {
		return errors.NewError("cannot add view role to the hawkular service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add cluster reader role to heapster service account
	if err = AddClusterRole(osClient, "cluster-reader", "system:serviceaccount:openshift-infra:heapster"); err != nil {
		return errors.NewError("cannot add cluster reader role to heapster service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Create metrics deployer secret
	deployerSecret := &kapi.Secret{}
	deployerSecret.Name = metricsDeployerSecret
	deployerSecret.Data = map[string][]byte{"nothing": []byte("/dev/null")}
	if _, err = kubeClient.Core().Secrets(infraNamespace).Create(deployerSecret); err != nil {
		return errors.NewError("cannot create metrics deployer secret").WithCause(err).WithDetails(h.OriginLog())
	}

	// Create the job client
	jobClient := kubeClient.Batch().Jobs(infraNamespace)

	// Submit the job
	jobClient.Create(metricsDeployerJob(hostName, imagePrefix, imageVersion))

	return nil
}

// Returns a job to create the metrics deployer pod
func metricsDeployerJob(hostName, imagePrefix, imageVersion string) *kbatch.Job {
	env := []kapi.EnvVar{
		{
			Name: "PROJECT",
			ValueFrom: &kapi.EnvVarSource{
				FieldRef: &kapi.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &kapi.EnvVarSource{
				FieldRef: &kapi.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "IMAGE_PREFIX",
			Value: fmt.Sprintf("%s-", imagePrefix),
		},
		{
			Name:  "IMAGE_VERSION",
			Value: imageVersion,
		},
		{
			Name:  "MASTER_URL",
			Value: "https://kubernetes.default.svc:443",
		},
		{
			Name:  "HAWKULAR_METRICS_HOSTNAME",
			Value: hostName,
		},
		{
			Name:  "MODE",
			Value: "deploy",
		},
		{
			Name:  "REDEPLOY",
			Value: "false",
		},
		{
			Name:  "IGNORE_PREFLIGHT",
			Value: "false",
		},
		{
			Name:  "USE_PERSISTENT_STORAGE",
			Value: "true",
		},
		{
			Name:  "CASSANDRA_NODES",
			Value: "1",
		},
		{
			Name:  "CASSANDRA_PV_SIZE",
			Value: "10Gi",
		},
		{
			Name:  "METRIC_DURATION",
			Value: "7",
		},
		{
			Name:  "HEAPSTER_NODE_ID",
			Value: "nodename",
		},
		{
			Name:  "METRIC_RESOLUTION",
			Value: "10s",
		},
	}
	podSpec := kapi.PodSpec{
		DNSPolicy:          kapi.DNSClusterFirst,
		RestartPolicy:      kapi.RestartPolicyNever,
		ServiceAccountName: metricsDeployerSA,
		Volumes: []kapi.Volume{
			{
				Name: "empty",
				VolumeSource: kapi.VolumeSource{
					EmptyDir: &kapi.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "secret",
				VolumeSource: kapi.VolumeSource{
					Secret: &kapi.SecretVolumeSource{
						SecretName: metricsDeployerSecret,
					},
				},
			},
		},
		Containers: []kapi.Container{
			{
				Image: fmt.Sprintf("%s-metrics-deployer:%s", imagePrefix, imageVersion),
				Name:  "deployer",
				VolumeMounts: []kapi.VolumeMount{
					{
						Name:      "secret",
						MountPath: "/secret",
						ReadOnly:  true,
					},
					{
						Name:      "empty",
						MountPath: "/etc/deploy",
					},
				},
				Env: env,
			},
		},
	}

	completions := int32(1)

	deadline := int64(60 * 5)

	meta := kapi.ObjectMeta{
		Name: metricsDeployerJobName,
	}

	job := &kbatch.Job{
		ObjectMeta: meta,
		Spec: kbatch.JobSpec{
			Completions:           &completions,
			ActiveDeadlineSeconds: &deadline,
			Template: kapi.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}
	return job
}

func MetricsHost(routingSuffix, serverIP string) string {
	if len(routingSuffix) > 0 {
		return fmt.Sprintf("metrics-openshift-infra.%s", routingSuffix)
	}
	return fmt.Sprintf("metrics-openshift-infra.%s.nip.io", serverIP)
}
