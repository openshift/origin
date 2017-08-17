// +build containers_image_openpgp

package signature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenpgpSigningMechanismSupportsSigning(t *testing.T) {
	mech, _, err := NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()
	err = mech.SupportsSigning()
	assert.Error(t, err)
	assert.IsType(t, SigningNotSupportedError(""), err)
}

func TestOpenpgpSigningMechanismSign(t *testing.T) {
	mech, _, err := NewEphemeralGPGSigningMechanism([]byte{})
	require.NoError(t, err)
	defer mech.Close()
	_, err = mech.Sign([]byte{}, TestKeyFingerprint)
	assert.Error(t, err)
	assert.IsType(t, SigningNotSupportedError(""), err)
}
