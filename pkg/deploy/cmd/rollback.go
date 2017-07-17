package cmd

import (
	"bytes"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl"
	kinternalprinters "k8s.io/kubernetes/pkg/printers/internalversion"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
)

func NewDeploymentConfigRollbacker(oc client.Interface) kubectl.Rollbacker {
	return &DeploymentConfigRollbacker{dn: oc}
}

// DeploymentConfigRollbacker is an implementation of the kubectl Rollbacker interface
// for deployment configs.
type DeploymentConfigRollbacker struct {
	dn client.DeploymentConfigsNamespacer
}

var _ kubectl.Rollbacker = &DeploymentConfigRollbacker{}

// Rollback the provided deployment config to a specific revision. If revision is zero, we will
// rollback to the previous deployment.
func (r *DeploymentConfigRollbacker) Rollback(obj runtime.Object, updatedAnnotations map[string]string, toRevision int64, dryRun bool) (string, error) {
	config, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		return "", fmt.Errorf("passed object is not a deployment config: %#v", obj)
	}
	if config.Spec.Paused {
		return "", fmt.Errorf("cannot rollback a paused config; resume it first with 'rollout resume dc/%s' and try again", config.Name)
	}

	rollback := &deployapi.DeploymentConfigRollback{
		Name:               config.Name,
		UpdatedAnnotations: updatedAnnotations,
		Spec: deployapi.DeploymentConfigRollbackSpec{
			Revision:        toRevision,
			IncludeTemplate: true,
		},
	}

	rolledback, err := r.dn.DeploymentConfigs(config.Namespace).Rollback(rollback)
	if err != nil {
		return "", err
	}

	if dryRun {
		out := bytes.NewBuffer([]byte("\n"))
		kinternalprinters.DescribePodTemplate(rolledback.Spec.Template, kinternalprinters.NewPrefixWriter(out))
		return out.String(), nil
	}

	_, err = r.dn.DeploymentConfigs(config.Namespace).Update(rolledback)
	if err != nil {
		return "", err
	}

	return "rolled back", nil
}
