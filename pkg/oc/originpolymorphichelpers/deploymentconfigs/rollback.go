package deploymentconfigs

import (
	"bytes"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"
	kinternalprinters "k8s.io/kubernetes/pkg/printers/internalversion"

	appsv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	appstypedclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func NewDeploymentConfigRollbacker(appsClient appsclient.Interface) kubectl.Rollbacker {
	return &DeploymentConfigRollbacker{dn: appsClient.AppsV1()}
}

// DeploymentConfigRollbacker is an implementation of the kubectl Rollbacker interface
// for deployment configs.
type DeploymentConfigRollbacker struct {
	dn appstypedclient.DeploymentConfigsGetter
}

var _ kubectl.Rollbacker = &DeploymentConfigRollbacker{}

// Rollback the provided deployment config to a specific revision. If revision is zero, we will
// rollback to the previous deployment.
func (r *DeploymentConfigRollbacker) Rollback(obj runtime.Object, updatedAnnotations map[string]string, toRevision int64, dryRun bool) (string, error) {
	config, ok := obj.(*appsapi.DeploymentConfig)
	if !ok {
		return "", fmt.Errorf("passed object is not a deployment config: %#v", obj)
	}
	if config.Spec.Paused {
		return "", fmt.Errorf("cannot rollback a paused config; resume it first with 'rollout resume dc/%s' and try again", config.Name)
	}

	rollback := &appsv1.DeploymentConfigRollback{
		Name:               config.Name,
		UpdatedAnnotations: updatedAnnotations,
		Spec: appsv1.DeploymentConfigRollbackSpec{
			Revision:        toRevision,
			IncludeTemplate: true,
		},
	}

	rolledback, err := r.dn.DeploymentConfigs(config.Namespace).Rollback(config.Name, rollback)
	if err != nil {
		return "", err
	}

	if dryRun {
		out := bytes.NewBuffer([]byte("\n"))
		internalTemplate := &kapi.PodTemplateSpec{}
		if err := legacyscheme.Scheme.Convert(rolledback.Spec.Template, internalTemplate, nil); err != nil {
			return "", err
		}
		kinternalprinters.DescribePodTemplate(internalTemplate, kinternalprinters.NewPrefixWriter(out))
		return out.String(), nil
	}

	_, err = r.dn.DeploymentConfigs(config.Namespace).Update(rolledback)
	if err != nil {
		return "", err
	}

	return "rolled back", nil
}
