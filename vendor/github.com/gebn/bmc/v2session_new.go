package bmc

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/gebn/bmc/pkg/ipmi"

	"github.com/google/gopacket"
)

var (
	ErrIncorrectPassword = errors.New("RAKP2 HMAC fail (this indicates the " +
		"BMC is using a different password)")
	ErrNoSupportedCipherSuite = errors.New("none of the provided cipher " +
		"suite options were supported by the BMC")

	defaultCipherSuites = []ipmi.CipherSuite{
		ipmi.CipherSuite17,
		ipmi.CipherSuite3,
	}
)

// V2SessionOpts contains configurable parameters for RMCP+ session
// establishment. The default value is used when creating a version-agnostic
// Session instance.
type V2SessionOpts struct {
	SessionOpts

	// PrivilegeLevelLookup indicates whether to use both the MaxPrivilegeLevel
	// and Username to search for the relevant user entry. If this is true, as
	// both are used in the search, a user will a lower max privilege level than
	// MaxPrivilegeLevel would not be found. If this is true and the username is
	// empty, we effectively use role-based authentication.
	PrivilegeLevelLookup bool

	// KG is the key-generating key or "BMC key". It is almost always unset, as
	// it effectively adds a second password in addition to the user/role
	// password, which must be known a-priori to establish a session. It is a 20
	// byte value. If this field is unset, K_[UID], i.e. the user password, will
	// be used in its place (and it is recommended for all 20 bytes of that
	// password to be used to preserve the complexity).
	KG []byte

	// CipherSuites is the list of authentication, integrity and confidentiality
	// algorithms to use in descending order of preference. If omitted, the
	// library will use Cipher Suite 17 if possible, falling back on Cipher
	// Suite 3, for which support is mandatory. To avoid performing
	// discovery, provide a single cipher suite.
	CipherSuites []ipmi.CipherSuite
}

// NewSession establishes a new RMCP+ session. Two-key login is assumed to be
// disabled (i.e. KG is null), and all algorithms supported by the library will
// be offered. This should cover the majority of use cases, and is recommended
// unless you know a-priori that a BMC key is set.
func (s *V2SessionlessTransport) NewSession(
	ctx context.Context,
	opts *SessionOpts,
) (Session, error) {
	return s.NewV2Session(ctx, &V2SessionOpts{
		SessionOpts: *opts,
	})
}

// NewV2Session establishes a new RMCP+ session with fine-grained parameters.
// This function does not modify the input options. The caller is responsible
// for knowing that v2.0 is supported.
func (s *V2SessionlessTransport) NewV2Session(ctx context.Context, opts *V2SessionOpts) (*V2Session, error) {
	// all the effort is in establish(); this method exists to provide a single
	// point for incrementing the failure count
	sessionOpenAttempts.Inc()
	sess, err := s.newV2Session(ctx, opts)
	if err != nil {
		sessionOpenFailures.Inc()
		return nil, err
	}
	sessionsOpen.Inc()
	return sess, nil
}

// newV2Session negotiates a new session, returning it on success. It will
// return ErrIncorrectPassword if the BMC appears to be using a different
// password to the remote console.
func (s *V2SessionlessTransport) newV2Session(ctx context.Context, opts *V2SessionOpts) (*V2Session, error) {
	cipherSuite, err := s.determineCipherSuite(ctx, opts.CipherSuites)
	if err != nil {
		return nil, err
	}

	openSessionRsp, err := s.openSession(ctx, &ipmi.OpenSessionReq{
		MaxPrivilegeLevel: opts.MaxPrivilegeLevel,
		SessionID:         1,
		AuthenticationPayload: ipmi.AuthenticationPayload{
			Algorithm: cipherSuite.AuthenticationAlgorithm,
		},
		IntegrityPayload: ipmi.IntegrityPayload{
			Algorithm: cipherSuite.IntegrityAlgorithm,
		},
		ConfidentialityPayload: ipmi.ConfidentialityPayload{
			Algorithm: cipherSuite.ConfidentialityAlgorithm,
		},
	})
	if err != nil {
		return nil, err
	}

	// RAKP Message 1, 2
	remoteConsoleRandom := [16]byte{}
	if _, err := rand.Read(remoteConsoleRandom[:]); err != nil {
		return nil, err
	}
	rakpMessage1 := &ipmi.RAKPMessage1{
		ManagedSystemSessionID: openSessionRsp.ManagedSystemSessionID,
		RemoteConsoleRandom:    remoteConsoleRandom,
		PrivilegeLevelLookup:   opts.PrivilegeLevelLookup,
		MaxPrivilegeLevel:      opts.MaxPrivilegeLevel,
		Username:               opts.Username,
	}
	rakpMessage2, err := s.rakpMessage1(ctx, rakpMessage1)
	if err != nil {
		return nil, err
	}

	hashGenerator, err := algorithmAuthenticationHashGenerator(
		openSessionRsp.AuthenticationPayload.Algorithm)
	if err != nil {
		return nil, err
	}

	authCodeHash := hashGenerator.AuthCode(opts.Password)
	rakpMessage2AuthCode := calculateRAKPMessage2AuthCode(authCodeHash,
		rakpMessage1, rakpMessage2)
	if !hmac.Equal(rakpMessage2.AuthCode, rakpMessage2AuthCode) {
		return nil, ErrIncorrectPassword
	}

	rakpMessage4, err := s.rakpMessage3(ctx, &ipmi.RAKPMessage3{
		Status:                 ipmi.StatusCodeOK,
		ManagedSystemSessionID: openSessionRsp.ManagedSystemSessionID,
		AuthCode: calculateRAKPMessage3AuthCode(
			authCodeHash, rakpMessage1, rakpMessage2),
	})
	if err != nil {
		return nil, err
	}

	effectiveBMCKey := opts.KG
	if len(effectiveBMCKey) == 0 {
		effectiveBMCKey = opts.Password
	}
	sikHash := hashGenerator.SIK(effectiveBMCKey)
	sik := calculateSIK(sikHash, rakpMessage1, rakpMessage2)
	icvHash := hashGenerator.ICV(sik)
	rakpMessage4ICV := calculateRAKPMessage4ICV(icvHash, rakpMessage1,
		rakpMessage2)
	if !hmac.Equal(rakpMessage4.ICV, rakpMessage4ICV) {
		return nil, fmt.Errorf("RAKP4 ICV fail: got %v, want %v",
			hex.EncodeToString(rakpMessage4.ICV),
			hex.EncodeToString(rakpMessage4ICV))
	}

	keyMaterialGen := additionalKeyMaterialGenerator{
		hash: hashGenerator.K(sik),
	}
	hasher, err := algorithmHasher(openSessionRsp.IntegrityPayload.Algorithm,
		keyMaterialGen)
	if err != nil {
		return nil, err
	}
	cipherLayer, err := algorithmCipher(
		openSessionRsp.ConfidentialityPayload.Algorithm, keyMaterialGen)
	if err != nil {
		return nil, err
	}

	sess := &V2Session{
		v2ConnectionShared:             &s.v2ConnectionShared,
		LocalID:                        openSessionRsp.RemoteConsoleSessionID,
		RemoteID:                       openSessionRsp.ManagedSystemSessionID,
		SIK:                            sik,
		AuthenticationAlgorithm:        openSessionRsp.AuthenticationPayload.Algorithm,
		IntegrityAlgorithm:             openSessionRsp.IntegrityPayload.Algorithm,
		ConfidentialityAlgorithm:       openSessionRsp.ConfidentialityPayload.Algorithm,
		AdditionalKeyMaterialGenerator: keyMaterialGen,
		integrityAlgorithm:             hasher,
		confidentialityLayer:           cipherLayer,
		timeout:                        s.timeout,
	}
	// do not set properties of the session layer here, as it is overwritten
	// each send
	dlc := gopacket.DecodingLayerContainer(gopacket.DecodingLayerArray(nil))
	dlc = dlc.Put(&sess.rmcpLayer)
	dlc = dlc.Put(&sess.sessionSelectorLayer)
	dlc = dlc.Put(&sess.v2SessionLayer)
	dlc = dlc.Put(cipherLayer)
	dlc = dlc.Put(&sess.messageLayer)
	sess.decode = dlc.LayersDecoder(sess.rmcpLayer.LayerType(), gopacket.NilDecodeFeedback)
	return sess, nil
}

// determineCipherSuite picks the set of algorithms that will be used to
// establish the session, doing discovery if we have multiple options.
func (s *V2SessionlessTransport) determineCipherSuite(ctx context.Context, desiredSuites []ipmi.CipherSuite) (*ipmi.CipherSuite, error) {
	if len(desiredSuites) == 0 {
		desiredSuites = defaultCipherSuites
	}

	if len(desiredSuites) == 1 {
		// no discovery required
		return &desiredSuites[0], nil
	}

	supportedSuites, err := RetrieveSupportedCipherSuites(ctx, s)
	if err != nil {
		return nil, err
	}

	// assume all will be unique
	distinctSupportedSuites := make(map[ipmi.CipherSuite]struct{}, len(supportedSuites))
	for _, suite := range supportedSuites {
		// it's fine to discard IDs and OEMs - they are irrelevant for Open Session
		distinctSupportedSuites[suite.CipherSuite] = struct{}{}
	}

	for _, desiredSuite := range desiredSuites {
		if _, ok := distinctSupportedSuites[desiredSuite]; ok {
			return &desiredSuite, nil
		}
	}
	return nil, ErrNoSupportedCipherSuite
}
