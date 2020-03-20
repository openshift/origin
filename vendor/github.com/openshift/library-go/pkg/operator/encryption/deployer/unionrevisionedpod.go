package deployer

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
)

// UnionRevisionLabelPodDeployer provides unified state from multiple distinct deployers.
type UnionRevisionLabelPodDeployer struct {
	delegates []statemachine.Deployer
	hasSynced []cache.InformerSynced
}

var _ statemachine.Deployer = &UnionRevisionLabelPodDeployer{}

// NewUnionRevisionLabelPodDeployer creates a deployer that returns a unified state from multiple distinct deployers.
// That means:
//  - none has reported an error
//  - all have converged
//  - all have observed exactly the same encryption configuration
func NewUnionRevisionLabelPodDeployer(delegates ...statemachine.Deployer) *UnionRevisionLabelPodDeployer {
	return &UnionRevisionLabelPodDeployer{delegates: delegates}
}

// DeployedEncryptionConfigSecret returns the actual encryption configuration across multiple deployers
func (d *UnionRevisionLabelPodDeployer) DeployedEncryptionConfigSecret() (secret *corev1.Secret, converged bool, err error) {
	seenSecrets := []*corev1.Secret{}

	for _, delegate := range d.delegates {
		secret, converged, err := delegate.DeployedEncryptionConfigSecret()
		if !converged || err != nil {
			return nil, converged, err
		}

		seenSecrets = append(seenSecrets, secret)
	}

	if len(seenSecrets) == 0 {
		return nil, false, nil
	}

	// we need to check that the encryption configuration is exactly the same among deployers
	// so we promote the fist secret and compare it with the rest
	goldenSecret := seenSecrets[0]
	seenSecrets = seenSecrets[1:]

	goldenEncryptionCfg, err := encryptionconfig.FromSecret(goldenSecret)
	if err != nil {
		return nil, false, err
	}

	for _, secret := range seenSecrets {
		currentEncryptionCfg, err := encryptionconfig.FromSecret(secret)
		if err != nil {
			return nil, false, err
		}

		if !reflect.DeepEqual(goldenEncryptionCfg.Resources, currentEncryptionCfg.Resources) {
			return nil, false, nil
		}
	}

	return goldenSecret, true, nil
}

func (d *UnionRevisionLabelPodDeployer) HasSynced() bool {
	for _, hasSynced := range d.hasSynced {
		if !hasSynced() {
			return false
		}
	}
	return true
}

// AddEventHandler registers a event handler that might influence the result of DeployedEncryptionConfigSecret for all configured deployers.
func (d *UnionRevisionLabelPodDeployer) AddEventHandler(handler cache.ResourceEventHandler) {
	d.hasSynced = []cache.InformerSynced{}
	for _, delegate := range d.delegates {
		delegate.AddEventHandler(handler)
		d.hasSynced = append(d.hasSynced, delegate.HasSynced)
	}
}
