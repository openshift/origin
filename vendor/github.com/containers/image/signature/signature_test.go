package signature

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/containers/image/version"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func TestInvalidSignatureError(t *testing.T) {
	// A stupid test just to keep code coverage
	s := "test"
	err := InvalidSignatureError{msg: s}
	assert.Equal(t, s, err.Error())
}

func TestNewUntrustedSignature(t *testing.T) {
	timeBefore := time.Now()
	sig := newUntrustedSignature(TestImageManifestDigest, TestImageSignatureReference)
	assert.Equal(t, TestImageManifestDigest, sig.UntrustedDockerManifestDigest)
	assert.Equal(t, TestImageSignatureReference, sig.UntrustedDockerReference)
	require.NotNil(t, sig.UntrustedCreatorID)
	assert.Equal(t, "atomic "+version.Version, *sig.UntrustedCreatorID)
	require.NotNil(t, sig.UntrustedTimestamp)
	timeAfter := time.Now()
	assert.True(t, timeBefore.Unix() <= *sig.UntrustedTimestamp)
	assert.True(t, *sig.UntrustedTimestamp <= timeAfter.Unix())
}

func TestMarshalJSON(t *testing.T) {
	// Empty string values
	s := newUntrustedSignature("", "_")
	_, err := s.MarshalJSON()
	assert.Error(t, err)
	s = newUntrustedSignature("_", "")
	_, err = s.MarshalJSON()
	assert.Error(t, err)

	// Success
	// Use intermediate variables for these values so that we can take their addresses.
	creatorID := "CREATOR"
	timestamp := int64(1484683104)
	for _, c := range []struct {
		input    untrustedSignature
		expected string
	}{
		{
			untrustedSignature{
				UntrustedDockerManifestDigest: "digest!@#",
				UntrustedDockerReference:      "reference#@!",
				UntrustedCreatorID:            &creatorID,
				UntrustedTimestamp:            &timestamp,
			},
			"{\"critical\":{\"identity\":{\"docker-reference\":\"reference#@!\"},\"image\":{\"docker-manifest-digest\":\"digest!@#\"},\"type\":\"atomic container signature\"},\"optional\":{\"creator\":\"CREATOR\",\"timestamp\":1484683104}}",
		},
		{
			untrustedSignature{
				UntrustedDockerManifestDigest: "digest!@#",
				UntrustedDockerReference:      "reference#@!",
			},
			"{\"critical\":{\"identity\":{\"docker-reference\":\"reference#@!\"},\"image\":{\"docker-manifest-digest\":\"digest!@#\"},\"type\":\"atomic container signature\"},\"optional\":{}}",
		},
	} {
		marshaled, err := c.input.MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, []byte(c.expected), marshaled)

		// Also call MarshalJSON through the JSON package.
		marshaled, err = json.Marshal(c.input)
		assert.NoError(t, err)
		assert.Equal(t, []byte(c.expected), marshaled)
	}
}

// Return the result of modifying validJSON with fn
func modifiedUntrustedSignatureJSON(t *testing.T, validJSON []byte, modifyFn func(mSI)) []byte {
	var tmp mSI
	err := json.Unmarshal(validJSON, &tmp)
	require.NoError(t, err)

	modifyFn(tmp)

	modifiedJSON, err := json.Marshal(tmp)
	require.NoError(t, err)
	return modifiedJSON
}

// Verify that input can be unmarshaled as an untrustedSignature, and that it passes JSON schema validation, and return the unmarshaled untrustedSignature.
func succesfullyUnmarshalUntrustedSignature(t *testing.T, schemaLoader gojsonschema.JSONLoader, input []byte) untrustedSignature {
	inputString := string(input)

	var s untrustedSignature
	err := json.Unmarshal(input, &s)
	require.NoError(t, err, inputString)

	res, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewStringLoader(inputString))
	assert.True(t, err == nil, inputString)
	assert.True(t, res.Valid(), inputString)

	return s
}

// Verify that input can't be unmashaled as an untrusted signature, and that it fails JSON schema validation.
func assertUnmarshalUntrustedSignatureFails(t *testing.T, schemaLoader gojsonschema.JSONLoader, input []byte) {
	inputString := string(input)

	var s untrustedSignature
	err := json.Unmarshal(input, &s)
	assert.Error(t, err, inputString)

	res, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewStringLoader(inputString))
	assert.True(t, err != nil || !res.Valid(), inputString)
}

func TestUnmarshalJSON(t *testing.T) {
	// NOTE: The schema at schemaPath is NOT authoritative; docs/atomic-signature.json and the code is, rather!
	// The schemaPath references are not testing that the code follows the behavior declared by the schema,
	// they are testing that the schema follows the behavior of the code!
	schemaPath, err := filepath.Abs("../docs/atomic-signature-embedded-json.json")
	require.NoError(t, err)
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)

	// Invalid input. Note that json.Unmarshal is guaranteed to validate input before calling our
	// UnmarshalJSON implementation; so test that first, then test our error handling for completeness.
	assertUnmarshalUntrustedSignatureFails(t, schemaLoader, []byte("&"))
	var s untrustedSignature
	err = s.UnmarshalJSON([]byte("&"))
	assert.Error(t, err)

	// Not an object
	assertUnmarshalUntrustedSignatureFails(t, schemaLoader, []byte("1"))

	// Start with a valid JSON.
	validSig := newUntrustedSignature("digest!@#", "reference#@!")
	validJSON, err := validSig.MarshalJSON()
	require.NoError(t, err)

	// Success
	s = succesfullyUnmarshalUntrustedSignature(t, schemaLoader, validJSON)
	assert.Equal(t, validSig, s)

	// Various ways to corrupt the JSON
	breakFns := []func(mSI){
		// A top-level field is missing
		func(v mSI) { delete(v, "critical") },
		func(v mSI) { delete(v, "optional") },
		// Extra top-level sub-object
		func(v mSI) { v["unexpected"] = 1 },
		// "critical" not an object
		func(v mSI) { v["critical"] = 1 },
		// "optional" not an object
		func(v mSI) { v["optional"] = 1 },
		// A field of "critical" is missing
		func(v mSI) { delete(x(v, "critical"), "type") },
		func(v mSI) { delete(x(v, "critical"), "image") },
		func(v mSI) { delete(x(v, "critical"), "identity") },
		// Extra field of "critical"
		func(v mSI) { x(v, "critical")["unexpected"] = 1 },
		// Invalid "type"
		func(v mSI) { x(v, "critical")["type"] = 1 },
		func(v mSI) { x(v, "critical")["type"] = "unexpected" },
		// Invalid "image" object
		func(v mSI) { x(v, "critical")["image"] = 1 },
		func(v mSI) { delete(x(v, "critical", "image"), "docker-manifest-digest") },
		func(v mSI) { x(v, "critical", "image")["unexpected"] = 1 },
		// Invalid "docker-manifest-digest"
		func(v mSI) { x(v, "critical", "image")["docker-manifest-digest"] = 1 },
		// Invalid "identity" object
		func(v mSI) { x(v, "critical")["identity"] = 1 },
		func(v mSI) { delete(x(v, "critical", "identity"), "docker-reference") },
		func(v mSI) { x(v, "critical", "identity")["unexpected"] = 1 },
		// Invalid "docker-reference"
		func(v mSI) { x(v, "critical", "identity")["docker-reference"] = 1 },
		// Invalid "creator"
		func(v mSI) { x(v, "optional")["creator"] = 1 },
		// Invalid "timestamp"
		func(v mSI) { x(v, "optional")["timestamp"] = "unexpected" },
		func(v mSI) { x(v, "optional")["timestamp"] = 0.5 }, // Fractional input
	}
	for _, fn := range breakFns {
		testJSON := modifiedUntrustedSignatureJSON(t, validJSON, fn)
		assertUnmarshalUntrustedSignatureFails(t, schemaLoader, testJSON)
	}

	// Modifications to unrecognized fields in "optional" are allowed and ignored
	allowedModificationFns := []func(mSI){
		// Add an optional field
		func(v mSI) { x(v, "optional")["unexpected"] = 1 },
	}
	for _, fn := range allowedModificationFns {
		testJSON := modifiedUntrustedSignatureJSON(t, validJSON, fn)
		s := succesfullyUnmarshalUntrustedSignature(t, schemaLoader, testJSON)
		assert.Equal(t, validSig, s)
	}

	// Optional fields can be missing
	validSig = untrustedSignature{
		UntrustedDockerManifestDigest: "digest!@#",
		UntrustedDockerReference:      "reference#@!",
		UntrustedCreatorID:            nil,
		UntrustedTimestamp:            nil,
	}
	validJSON, err = validSig.MarshalJSON()
	require.NoError(t, err)
	s = succesfullyUnmarshalUntrustedSignature(t, schemaLoader, validJSON)
	assert.Equal(t, validSig, s)
}

func TestSign(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	defer mech.Close()

	if err := mech.SupportsSigning(); err != nil {
		t.Skipf("Signing not supported: %v", err)
	}

	sig := newUntrustedSignature("digest!@#", "reference#@!")

	// Successful signing
	signature, err := sig.sign(mech, TestKeyFingerprint)
	require.NoError(t, err)

	verified, err := verifyAndExtractSignature(mech, signature, signatureAcceptanceRules{
		validateKeyIdentity: func(keyIdentity string) error {
			if keyIdentity != TestKeyFingerprint {
				return errors.Errorf("Unexpected keyIdentity")
			}
			return nil
		},
		validateSignedDockerReference: func(signedDockerReference string) error {
			if signedDockerReference != sig.UntrustedDockerReference {
				return errors.Errorf("Unexpected signedDockerReference")
			}
			return nil
		},
		validateSignedDockerManifestDigest: func(signedDockerManifestDigest digest.Digest) error {
			if signedDockerManifestDigest != sig.UntrustedDockerManifestDigest {
				return errors.Errorf("Unexpected signedDockerManifestDigest")
			}
			return nil
		},
	})
	require.NoError(t, err)

	assert.Equal(t, sig.UntrustedDockerManifestDigest, verified.DockerManifestDigest)
	assert.Equal(t, sig.UntrustedDockerReference, verified.DockerReference)

	// Error creating blob to sign
	_, err = untrustedSignature{}.sign(mech, TestKeyFingerprint)
	assert.Error(t, err)

	// Error signing
	_, err = sig.sign(mech, "this fingerprint doesn't exist")
	assert.Error(t, err)
}

func TestVerifyAndExtractSignature(t *testing.T) {
	mech, err := newGPGSigningMechanismInDirectory(testGPGHomeDirectory)
	require.NoError(t, err)
	defer mech.Close()

	type triple struct {
		keyIdentity                string
		signedDockerReference      string
		signedDockerManifestDigest digest.Digest
	}
	var wanted, recorded triple
	// recordingRules are a plausible signatureAcceptanceRules implementations, but equally
	// importantly record that we are passing the correct values to the rule callbacks.
	recordingRules := signatureAcceptanceRules{
		validateKeyIdentity: func(keyIdentity string) error {
			recorded.keyIdentity = keyIdentity
			if keyIdentity != wanted.keyIdentity {
				return errors.Errorf("keyIdentity mismatch")
			}
			return nil
		},
		validateSignedDockerReference: func(signedDockerReference string) error {
			recorded.signedDockerReference = signedDockerReference
			if signedDockerReference != wanted.signedDockerReference {
				return errors.Errorf("signedDockerReference mismatch")
			}
			return nil
		},
		validateSignedDockerManifestDigest: func(signedDockerManifestDigest digest.Digest) error {
			recorded.signedDockerManifestDigest = signedDockerManifestDigest
			if signedDockerManifestDigest != wanted.signedDockerManifestDigest {
				return errors.Errorf("signedDockerManifestDigest mismatch")
			}
			return nil
		},
	}

	signature, err := ioutil.ReadFile("./fixtures/image.signature")
	require.NoError(t, err)
	signatureData := triple{
		keyIdentity:                TestKeyFingerprint,
		signedDockerReference:      TestImageSignatureReference,
		signedDockerManifestDigest: TestImageManifestDigest,
	}

	// Successful verification
	wanted = signatureData
	recorded = triple{}
	sig, err := verifyAndExtractSignature(mech, signature, recordingRules)
	require.NoError(t, err)
	assert.Equal(t, TestImageSignatureReference, sig.DockerReference)
	assert.Equal(t, TestImageManifestDigest, sig.DockerManifestDigest)
	assert.Equal(t, signatureData, recorded)

	// For extra paranoia, test that we return a nil signature object on error.

	// Completely invalid signature.
	recorded = triple{}
	sig, err = verifyAndExtractSignature(mech, []byte{}, recordingRules)
	assert.Error(t, err)
	assert.Nil(t, sig)
	assert.Equal(t, triple{}, recorded)

	recorded = triple{}
	sig, err = verifyAndExtractSignature(mech, []byte("invalid signature"), recordingRules)
	assert.Error(t, err)
	assert.Nil(t, sig)
	assert.Equal(t, triple{}, recorded)

	// Valid signature of non-JSON: asked for keyIdentity, only
	invalidBlobSignature, err := ioutil.ReadFile("./fixtures/invalid-blob.signature")
	require.NoError(t, err)
	recorded = triple{}
	sig, err = verifyAndExtractSignature(mech, invalidBlobSignature, recordingRules)
	assert.Error(t, err)
	assert.Nil(t, sig)
	assert.Equal(t, triple{keyIdentity: signatureData.keyIdentity}, recorded)

	// Valid signature with a wrong key: asked for keyIdentity, only
	wanted = signatureData
	wanted.keyIdentity = "unexpected fingerprint"
	recorded = triple{}
	sig, err = verifyAndExtractSignature(mech, signature, recordingRules)
	assert.Error(t, err)
	assert.Nil(t, sig)
	assert.Equal(t, triple{keyIdentity: signatureData.keyIdentity}, recorded)

	// Valid signature with a wrong manifest digest: asked for keyIdentity and signedDockerManifestDigest
	wanted = signatureData
	wanted.signedDockerManifestDigest = "invalid digest"
	recorded = triple{}
	sig, err = verifyAndExtractSignature(mech, signature, recordingRules)
	assert.Error(t, err)
	assert.Nil(t, sig)
	assert.Equal(t, triple{
		keyIdentity:                signatureData.keyIdentity,
		signedDockerManifestDigest: signatureData.signedDockerManifestDigest,
	}, recorded)

	// Valid signature with a wrong image reference
	wanted = signatureData
	wanted.signedDockerReference = "unexpected docker reference"
	recorded = triple{}
	sig, err = verifyAndExtractSignature(mech, signature, recordingRules)
	assert.Error(t, err)
	assert.Nil(t, sig)
	assert.Equal(t, signatureData, recorded)
}

func TestGetUntrustedSignatureInformationWithoutVerifying(t *testing.T) {
	signature, err := ioutil.ReadFile("./fixtures/image.signature")
	require.NoError(t, err)
	// Successful parsing, all optional fields present
	info, err := GetUntrustedSignatureInformationWithoutVerifying(signature)
	require.NoError(t, err)
	assert.Equal(t, TestImageSignatureReference, info.UntrustedDockerReference)
	assert.Equal(t, TestImageManifestDigest, info.UntrustedDockerManifestDigest)
	assert.NotNil(t, info.UntrustedCreatorID)
	assert.Equal(t, "atomic ", *info.UntrustedCreatorID)
	assert.NotNil(t, info.UntrustedTimestamp)
	assert.Equal(t, time.Unix(1458239713, 0), *info.UntrustedTimestamp)
	assert.Equal(t, TestKeyShortID, info.UntrustedShortKeyIdentifier)
	// Successful parsing, no optional fields present
	signature, err = ioutil.ReadFile("./fixtures/no-optional-fields.signature")
	require.NoError(t, err)
	// Successful parsing
	info, err = GetUntrustedSignatureInformationWithoutVerifying(signature)
	require.NoError(t, err)
	assert.Equal(t, TestImageSignatureReference, info.UntrustedDockerReference)
	assert.Equal(t, TestImageManifestDigest, info.UntrustedDockerManifestDigest)
	assert.Nil(t, info.UntrustedCreatorID)
	assert.Nil(t, info.UntrustedTimestamp)
	assert.Equal(t, TestKeyShortID, info.UntrustedShortKeyIdentifier)

	// Completely invalid signature.
	_, err = GetUntrustedSignatureInformationWithoutVerifying([]byte{})
	assert.Error(t, err)

	_, err = GetUntrustedSignatureInformationWithoutVerifying([]byte("invalid signature"))
	assert.Error(t, err)

	// Valid signature of non-JSON
	invalidBlobSignature, err := ioutil.ReadFile("./fixtures/invalid-blob.signature")
	require.NoError(t, err)
	_, err = GetUntrustedSignatureInformationWithoutVerifying(invalidBlobSignature)
	assert.Error(t, err)
}
