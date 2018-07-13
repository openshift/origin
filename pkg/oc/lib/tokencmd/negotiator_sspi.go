// +build windows

package tokencmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/cmd/util/term"

	"github.com/alexbrainman/sspi"
	"github.com/alexbrainman/sspi/negotiate"
	"github.com/golang/glog"
)

const (
	// sane set of default flags, see sspiNegotiator.desiredFlags
	// TODO make configurable?
	desiredFlags = sspi.ISC_REQ_CONFIDENTIALITY |
		sspi.ISC_REQ_INTEGRITY |
		sspi.ISC_REQ_MUTUAL_AUTH |
		sspi.ISC_REQ_REPLAY_DETECT |
		sspi.ISC_REQ_SEQUENCE_DETECT
	// subset of desiredFlags that must be set, see sspiNegotiator.requiredFlags
	// TODO make configurable?
	requiredFlags = sspi.ISC_REQ_CONFIDENTIALITY |
		sspi.ISC_REQ_INTEGRITY |
		sspi.ISC_REQ_MUTUAL_AUTH

	// various windows user name formats
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa380525(v=vs.85).aspx
	// https://msdn.microsoft.com/en-us/library/ms724268(VS.85).aspx
	// separator used in fully qualified user name or down-level logon name format (DOMAIN\Username)
	domainSeparator = `\`
	// https://msdn.microsoft.com/en-us/library/ms677605(v=vs.85).aspx#userPrincipalName
	// separator used in user principal name (UPN) format (username@domain.com)
	upnSeparator = "@"
	// https://msdn.microsoft.com/en-us/library/system.environment.userdomainname(v=vs.110).aspx
	// environment variable that holds the network domain name associated with the current user
	// this is the NetBIOS domain name which should fit within the length requirement (see maxDomain)
	shortDomainEnvVar = "USERDOMAIN"

	// max lengths for various fields, see sspiNegotiator.getDomainAndUsername and sspiNegotiator.getPassword
	// When using the Negotiate package, the maximum character lengths for user name, password, and domain are
	// 256, 256, and 15, respectively.
	maxUsername = 256
	maxPassword = 256
	maxDomain   = 15
)

func SSPIEnabled() bool {
	return true
}

// sspiNegotiator handles negotiate flows on windows via SSPI
// It expects sspiNegotiator.InitSecContext to be called until sspiNegotiator.IsComplete returns true
type sspiNegotiator struct {
	// principalName is an optional username (in fully qualified, user principal name or short format).
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa374714(v=vs.85).aspx
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa380131(v=vs.85).aspx
	// pAuthData [in]: If credentials are supplied, they are passed via a pointer to a sspi.SEC_WINNT_AUTH_IDENTITY
	// structure that includes those credentials.
	principalName string
	// password is an optional password used to log into a specific account if principalName is non-empty.
	// This allows logging in via username and password even when basic auth is not enabled.
	password string

	// reader is used to prompt for a password if principalName is non-empty and password is empty.
	reader io.Reader
	// writer is used to output prompts when prompting for password.
	writer io.Writer
	// host is the server being authenticated to.  Used only for displaying messages when prompting for password.
	host string

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms721572(v=vs.85).aspx#_security_credentials_gly
	// phCredential [in, optional]: A handle to the credentials returned by AcquireCredentialsHandle (Negotiate).
	// This handle is used to build the security context.  sspi.SECPKG_CRED_OUTBOUND is used to request OUTBOUND credentials.
	cred *sspi.Credentials
	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms721625(v=vs.85).aspx#_security_security_context_gly
	// Manages all steps of the Negotiate negotiation.
	ctx *negotiate.ClientContext
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa375509(v=vs.85).aspx
	// fContextReq [in]: Bit flags that indicate requests for the context.
	desiredFlags uint32
	// requiredFlags is the subset of desiredFlags that must be set for flag verification to succeed
	requiredFlags uint32
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa375509(v=vs.85).aspx
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa374764(v=vs.85).aspx
	// Set to true once InitializeSecurityContext or CompleteAuthToken return sspi.SEC_E_OK
	complete bool
}

func NewSSPINegotiator(principalName, password, host string, reader io.Reader) Negotiator {
	return &sspiNegotiator{
		principalName: principalName,
		password:      password,
		reader:        reader,
		writer:        os.Stdout,
		host:          host,
		desiredFlags:  desiredFlags,
		requiredFlags: requiredFlags,
	}
}

func (s *sspiNegotiator) Load() error {
	glog.V(5).Info("Attempt to load SSPI")
	// do nothing since SSPI uses lazy DLL loading
	return nil
}

func (s *sspiNegotiator) InitSecContext(requestURL string, challengeToken []byte) (tokenToSend []byte, err error) {
	defer runtime.HandleCrash()

	if needsInit := s.cred == nil || s.ctx == nil; needsInit {
		logSSPI("Start SSPI flow: %s", requestURL)
		return s.initContext(requestURL)
	}

	glog.V(5).Info("Continue SSPI flow")
	return s.updateContext(challengeToken)
}

func (s *sspiNegotiator) IsComplete() bool {
	return s.complete
}

func (s *sspiNegotiator) Release() error {
	defer runtime.HandleCrash()
	glog.V(5).Info("Attempt to release SSPI")
	var errs []error
	if err := s.ctx.Release(); err != nil {
		logSSPI("SSPI context release failed: %v", err)
		errs = append(errs, err)
	}
	if err := s.cred.Release(); err != nil {
		logSSPI("SSPI credential release failed: %v", err)
		errs = append(errs, err)
	}
	return errors.Reduce(errors.NewAggregate(errs))
}

func (s *sspiNegotiator) initContext(requestURL string) (outputToken []byte, err error) {
	cred, err := s.getUserCredentials()
	if err != nil {
		logSSPI("getUserCredentials failed: %v", err)
		return nil, err
	}
	s.cred = cred
	glog.V(5).Info("getUserCredentials successful")

	serviceName, err := getServiceName('/', requestURL)
	if err != nil {
		return nil, err
	}

	logSSPI("importing service name %s", serviceName)
	ctx, outputToken, err := negotiate.NewClientContextWithFlags(s.cred, serviceName, s.desiredFlags)
	if err != nil {
		logSSPI("NewClientContextWithFlags failed: %v", err)
		return nil, err
	}
	s.ctx = ctx
	glog.V(5).Info("NewClientContextWithFlags successful")
	return outputToken, nil
}

func (s *sspiNegotiator) getUserCredentials() (*sspi.Credentials, error) {
	if len(s.principalName) == 0 && len(s.password) > 0 {
		return nil, fmt.Errorf("username cannot be empty with non-empty password")
	}

	// Try to use principalName if specified
	if len(s.principalName) > 0 {
		domain, username, err := s.getDomainAndUsername()
		if err != nil {
			return nil, err
		}
		password, err := s.getPassword(domain, username)
		if err != nil {
			return nil, err
		}

		logSSPI("Using AcquireUserCredentials because principalName is not empty, principalName=%s, username=%s, domain=%s",
			s.principalName, username, domain)
		// this call seems to never fail, even when domain / username / password are nonsense
		cred, err := negotiate.AcquireUserCredentials(domain, username, password)
		if err != nil {
			logSSPI("AcquireUserCredentials failed: %v", err)
			return nil, err
		}
		glog.V(5).Info("AcquireUserCredentials successful")
		return cred, nil
	}

	glog.V(5).Info("Using AcquireCurrentUserCredentials because principalName is empty")
	return negotiate.AcquireCurrentUserCredentials()
}

func (s *sspiNegotiator) getDomainAndUsername() (domain, username string, err error) {
	switch {
	case strings.Contains(s.principalName, domainSeparator):
		data := strings.Split(s.principalName, domainSeparator)
		// try to provide useful error messages
		if len(data) != 2 || len(data[1]) == 0 {
			return "", "", fmt.Errorf(
				`invalid username %s, fully qualified user name format must have single backslash and non-empty user (ex: DOMAIN\Username)`,
				s.principalName)
		}
		domain = data[0]
		username = data[1]

	case strings.Contains(s.principalName, upnSeparator):
		// leave domain empty and assume it is qualified in the username (UPN format)
		username = s.principalName

	default:
		// this is a short name meaning we will need to lookup the current user's domain
		// TODO should we use syscall.NetGetJoinInformation first and then fallback to the env var?
		domain, _ = os.LookupEnv(shortDomainEnvVar)
		username = s.principalName
	}

	// try to provide useful error messages
	if domainLen, usernameLen := len(domain), len(username); domainLen > maxDomain || usernameLen > maxUsername {
		return "", "",
			fmt.Errorf("the maximum character lengths for user name and domain are %d and %d, respectively:\n"+
				"input username=%s username=%s,len=%d domain=%s,len=%d",
				maxUsername, maxDomain, s.principalName, username, usernameLen, domain, domainLen)
	}

	return domain, username, nil
}

func (s *sspiNegotiator) getPassword(domain, username string) (string, error) {
	password := s.password

	if missingPassword := len(password) == 0; missingPassword {
		// mimic output from basic auth prompt
		if hasDomain := len(domain) > 0; hasDomain {
			fmt.Fprintf(s.writer, "Authentication required for %s (%s)\n", s.host, domain)
		} else {
			fmt.Fprintf(s.writer, "Authentication required for %s\n", s.host)
		}
		fmt.Fprintf(s.writer, "Username: %s\n", username)
		// empty password from prompt is ok
		// we do not need to worry about being stuck in a prompt loop because ClientContext.Update
		// will fail if the password is incorrect and that will end the challenge flow
		password = term.PromptForPasswordString(s.reader, s.writer, "Password: ")
	}

	// try to provide useful error messages
	if passwordLen := len(password); passwordLen > maxPassword {
		return "", fmt.Errorf("the maximum character length for password is %d: password=<redacted>,len=%d",
			maxPassword, passwordLen)
	}

	return password, nil
}

func (s *sspiNegotiator) updateContext(challengeToken []byte) (outputToken []byte, err error) {
	// ClientContext.Update does not return errors for success codes:
	// 1. sspi.SEC_E_OK (complete=true and err=nil)
	// 2. sspi.SEC_I_CONTINUE_NEEDED (complete=false and err=nil)
	// 3. sspi.SEC_I_COMPLETE_AND_CONTINUE and sspi.SEC_I_COMPLETE_NEEDED
	// complete=false and err=nil as long as sspi.CompleteAuthToken returns sspi.SEC_E_OK
	// Thus we can safely assume that any error returned here is an error code
	authCompleted, outputToken, err := s.ctx.Update(challengeToken)
	if err != nil {
		logSSPI("ClientContext.Update failed: %v", err)
		return nil, err
	}
	s.complete = authCompleted
	logSSPI("ClientContext.Update successful, complete=%v", s.complete)

	// TODO should we skip the flag check if complete = true?
	if nonFatalErr := s.ctx.VerifyFlags(); nonFatalErr == nil {
		glog.V(5).Info("ClientContext.VerifyFlags successful")
	} else {
		logSSPI("ClientContext.VerifyFlags failed: %v", nonFatalErr)
		if fatalErr := s.ctx.VerifySelectiveFlags(s.requiredFlags); fatalErr != nil {
			logSSPI("ClientContext.VerifySelectiveFlags failed: %v", fatalErr)
			return nil, fatalErr
		}
		glog.V(5).Info("ClientContext.VerifySelectiveFlags successful")
	}

	return outputToken, nil
}

// logSSPI is the equivalent of glog.V(5).Infof(format, args) except it
// includes error code information for any syscall.Errno contained in args
func logSSPI(format string, args ...interface{}) {
	if glog.V(5) {
		for i, arg := range args {
			if errno, ok := arg.(syscall.Errno); ok {
				args[i] = fmt.Sprintf("%v, code=%#v", errno, errno)
			}
		}
		s := fmt.Sprintf(format, args...)
		glog.InfoDepth(1, s)
	}
}
