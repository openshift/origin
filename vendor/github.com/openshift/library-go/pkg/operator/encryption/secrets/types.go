package secrets

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// This label is used to find secrets that build up the final encryption config.  The names of the
	// secrets are in format <shared prefix>-<unique monotonically increasing uint> (the uint is the keyID).
	// For example, openshift-kube-apiserver-encryption-3.  Note that other than the -3 postfix, the name of
	// the secret is irrelevant since the label is used to find the secrets.  Of course the key minting
	// controller cares about the entire name since it needs to know when it has already created a secret for a given
	// keyID meaning it cannot just use a random prefix.  As such the name must include the data that is contained
	// within the label.  Thus the format used is <component>-encryption-<keyID>.  This keeps everything distinct
	// and fully deterministic.  The keys are ordered by keyID where a smaller ID means an earlier key.
	// This means that the latest secret (the one with the largest keyID) is the current desired write key.
	EncryptionKeySecretsLabel = "encryption.apiserver.operator.openshift.io/component"

	// These annotations are used to mark the current observed state of a secret.

	// The time (in RFC3339 format) at which the migrated state observation occurred.  The key minting
	// controller parses this field to determine if enough time has passed and a new key should be created.
	EncryptionSecretMigratedTimestamp = "encryption.apiserver.operator.openshift.io/migrated-timestamp"
	// The list of resources that were migrated when encryptionSecretMigratedTimestamp was set.
	// See the MigratedGroupResources struct below to understand the JSON encoding used.
	EncryptionSecretMigratedResources = "encryption.apiserver.operator.openshift.io/migrated-resources"

	// encryptionSecretMode is the annotation that determines how the provider associated with a given key is
	// configured.  For example, a key could be used with AES-CBC or Secretbox.  This allows for algorithm
	// agility.  When the default mode used by the key minting controller changes, it will force the creation
	// of a new key under the new mode even if encryptionSecretMigrationInterval has not been reached.
	encryptionSecretMode = "encryption.apiserver.operator.openshift.io/mode"

	// encryptionSecretInternalReason is the annotation that denotes why a particular key
	// was created based on "internal" reasons (i.e. key minting controller decided a new
	// key was needed for some reason X).  It is tracked solely for the purposes of debugging.
	encryptionSecretInternalReason = "encryption.apiserver.operator.openshift.io/internal-reason"

	// encryptionSecretExternalReason is the annotation that denotes why a particular key was created based on
	// "external" reasons (i.e. force key rotation for some reason Y).  It allows the key minting controller to
	// determine if a new key should be created even if encryptionSecretMigrationInterval has not been reached.
	encryptionSecretExternalReason = "encryption.apiserver.operator.openshift.io/external-reason"

	// In the data field of the secret API object, this (map) key is used to hold the actual encryption key
	// (i.e. for AES-CBC mode the value associated with this map key is 32 bytes of random noise).
	EncryptionSecretKeyDataKey = "encryption.apiserver.operator.openshift.io-key"

	// encryptionSecretFinalizer is a finalizer attached to all secrets generated
	// by the encryption controllers.  Its sole purpose is to prevent the accidental
	// deletion of secrets by enforcing a two phase delete.
	EncryptionSecretFinalizer = "encryption.apiserver.operator.openshift.io/deletion-protection"
)

// MigratedGroupResources is the data structured stored in the
// encryption.apiserver.operator.openshift.io/migrated-resources
// of a key secret.
type MigratedGroupResources struct {
	Resources []schema.GroupResource `json:"resources"`
}
