package openshift

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"text/template"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
)

const (
	defaultOpenshiftAnsibleImage   = "ansible"
	deploymentTypeOrigin           = "origin"
	deploymentTypeOCP              = "openshift-enterprise"
	imageStreamCentos              = "centos7"
	openshiftAnsibleServiceAccount = "openshift-ansible"
)

const defaultMetricsInventory = `
[OSEv3:children]
masters
nodes

[OSEv3:vars]
#openshift_release={{.OSERelease}}

openshift_deployment_type={{.OSEDeploymentType}} 

openshift_metrics_install_metrics=True
openshift_metrics_image_prefix={{.MetricsImagePrefix}}
openshift_metrics_image_version={{.MetricsImageVersion}}
openshift_metrics_resolution={{.MetricsResolution}}

openshift_metrics_hawkular_hostname={{.HawkularHostName}}

[masters]
{{.MasterIP}} ansible_connection=local

[nodes]
{{.MasterIP}}

[etcd]
{{.MasterIP}}
`

const defaultLoggingInventory = `
[OSEv3:children]
masters
nodes

[OSEv3:vars]
#openshift_release={{.OSERelease}}

openshift_deployment_type={{.OSEDeploymentType}} 

openshift_logging_image_prefix={{.LoggingImagePrefix}}
openshift_logging_image_version={{.LoggingImageVersion}}
openshift_logging_master_public_url={{.MasterPublicURL}}

openshift_logging_install_logging=true
openshift_logging_use_ops=false
openshift_logging_namespace={{.LoggingNamespace}}

openshift_logging_elasticseach_memory_limit=1024M
openshift_logging_elasticseach_storage_type=pvc
openshift_logging_elasticseach_pvc_size=100G

openshift_logging_kibana_hostname={{.KibanaHostName}}

[masters]
{{.MasterIP}} ansible_connection=local

[nodes]
{{.MasterIP}}

[etcd]
{{.MasterIP}}
`

type ansibleLoggingInventoryParams struct {
	Template            string
	LoggingImagePrefix  string
	LoggingImageVersion string
	LoggingNamespace    string
	KibanaHostName      string
}

type ansibleMetricsInventoryParams struct {
	MetricsImagePrefix  string
	MetricsImageVersion string
	MetricsResolution   string
	HawkularHostName    string
}

type ansibleInventoryParams struct {
	MasterIP          string
	MasterPublicURL   string
	OSERelease        string
	OSEDeploymentType string
	ansibleLoggingInventoryParams
	ansibleMetricsInventoryParams
}

type ansibleRunner struct {
	*Helper
	KubeClient     kclient.Interface
	SecurityClient securityclient.Interface
	ImageStreams   string
	Prefix         string
	Namespace      string
}

func newAnsibleRunner(h *Helper, kubeClient kclient.Interface, securityClient securityclient.Interface, namespace, imageStreams, prefix string) *ansibleRunner {
	return &ansibleRunner{
		Helper:         h,
		KubeClient:     kubeClient,
		SecurityClient: securityClient,
		ImageStreams:   imageStreams,
		Prefix:         prefix,
		Namespace:      namespace,
	}
}
func newAnsibleInventoryParams() ansibleInventoryParams {
	return ansibleInventoryParams{
		ansibleLoggingInventoryParams: ansibleLoggingInventoryParams{},
		ansibleMetricsInventoryParams: ansibleMetricsInventoryParams{},
	}
}

// writeInventoryToHost generates the inventory file given the parameters and writes
// the inventory file to a temp directory on the host
// return the basename of the inventory file
func (r *ansibleRunner) uploadInventoryToHost(inventoryTemplate string, params ansibleInventoryParams) (string, error) {
	inventory, err := generateAnsibleInventory(inventoryTemplate, params, r.ImageStreams)
	if err != nil {
		return "", err
	}
	file, err := ioutil.TempFile("", "openshift-inventory")
	if err != nil {
		return "", err
	}
	_, err = file.WriteString(inventory)
	if err != nil {
		return "", err
	}
	err = file.Close()
	if err != nil {
		return "", err
	}
	glog.V(1).Infof("Wrote inventory to local file: %s", file.Name())
	dest := fmt.Sprintf("%s/%s.inventory", "/var/lib/origin/openshift.local.config", r.Prefix)
	glog.V(1).Infof("Uploading file %s to host destination: %s", file.Name(), dest)
	err = r.Helper.hostHelper.UploadFileToContainer(file.Name(), dest)
	if err != nil {
		return "", err
	}
	return path.Base(dest), nil
}

// generateAnsibleInventory and return the content as a string
func generateAnsibleInventory(inventoryTemplate string, params ansibleInventoryParams, imageStreams string) (string, error) {

	// set the deploymentType
	if imageStreams == imageStreamCentos {
		params.OSEDeploymentType = deploymentTypeOrigin
	} else {
		params.OSEDeploymentType = deploymentTypeOCP
	}
	t, err := template.New("").Parse(inventoryTemplate)
	if err != nil {
		return "", errors.NewError("Unable to parse ansible inventory template").WithCause(err)
	}

	inventory := &bytes.Buffer{}
	err = t.Execute(inventory, params)
	if err != nil {
		return "", errors.NewError("Unable to substitute ansible params into the inventory template: %s", params).WithCause(err)
	}
	if glog.V(1) {
		glog.V(1).Infof("Generated ansible inventory:\n %s\n", inventory.String())
	}
	return inventory.String(), nil

}

func (r *ansibleRunner) createServiceAccount(namespace string) error {
	serviceAccount := &kapi.ServiceAccount{}
	serviceAccount.Name = openshiftAnsibleServiceAccount
	_, err := r.KubeClient.Core().ServiceAccounts(namespace).Create(serviceAccount)
	if err != nil && !kapierrors.IsAlreadyExists(err) {
		return errors.NewError(fmt.Sprintf("cannot create %s service account", serviceAccount.Name)).WithCause(err).WithDetails(r.Helper.OriginLog())
	}
	// Add privileged SCC to serviceAccount
	if err = AddSCCToServiceAccount(r.SecurityClient.Security(), "privileged", serviceAccount.Name, namespace, &bytes.Buffer{}); err != nil {
		return errors.NewError("cannot add privileged security context constraint to service account").WithCause(err).WithDetails(r.Helper.OriginLog())
	}
	return nil
}

func (r *ansibleRunner) RunPlaybook(params ansibleInventoryParams, playbook, hostConfigDir, imagePrefix, imageVersion string) error {
	if err := r.createServiceAccount(r.Namespace); err != nil {
		return err
	}

	inventoryBaseName, err := r.uploadInventoryToHost(params.Template, params)
	if err != nil {
		return err
	}

	image := fmt.Sprintf("%s-%s:%s", imagePrefix, defaultOpenshiftAnsibleImage, imageVersion)
	configBind := fmt.Sprintf("%s/master:/etc/origin/master", hostConfigDir)
	inventoryBind := fmt.Sprintf("%s/%s:/tmp/inventory", hostConfigDir, inventoryBaseName)
	if glog.V(1) {
		glog.V(1).Infof("Running image %s with playbook: %s", image, playbook)
		glog.V(1).Infof("With binding: %s", configBind)
		glog.V(1).Infof("With binding: %s", inventoryBind)
	}
	jobName := fmt.Sprintf("openshift-ansible-%s-job", r.Prefix)
	env := []kapi.EnvVar{
		{
			Name:  "INVENTORY_FILE",
			Value: "/tmp/inventory",
		},
		{
			Name:  "PLAYBOOK_FILE",
			Value: playbook,
		},
	}
	runAsUser := int64(0)
	podSpec := kapi.PodSpec{
		DNSPolicy:          kapi.DNSClusterFirst,
		RestartPolicy:      kapi.RestartPolicyNever,
		ServiceAccountName: openshiftAnsibleServiceAccount,
		SecurityContext: &kapi.PodSecurityContext{
			HostNetwork: true,
		},
		Containers: []kapi.Container{
			{
				Name:  jobName,
				Image: image,
				Env:   env,
				SecurityContext: &kapi.SecurityContext{
					RunAsUser: &runAsUser,
				},
				VolumeMounts: []kapi.VolumeMount{
					{
						Name:      "configdir",
						MountPath: "/etc/origin/master",
					},
					{
						Name:      "inventoryfile",
						MountPath: "/tmp/inventory",
					},
				},
			},
		},
		Volumes: []kapi.Volume{
			{
				Name: "configdir",
				VolumeSource: kapi.VolumeSource{
					HostPath: &kapi.HostPathVolumeSource{
						Path: fmt.Sprintf("%s/master", hostConfigDir),
					},
				},
			},
			{
				Name: "inventoryfile",
				VolumeSource: kapi.VolumeSource{
					HostPath: &kapi.HostPathVolumeSource{
						Path: fmt.Sprintf("%s/%s", hostConfigDir, inventoryBaseName),
					},
				},
			},
		},
	}

	completions := int32(1)
	deadline := int64(60 * 5)

	meta := metav1.ObjectMeta{
		Name: jobName,
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

	// Create the job client
	jobClient := r.KubeClient.Batch().Jobs(r.Namespace)

	// Submit the job
	_, err = jobClient.Create(job)
	if err != nil && kapierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
