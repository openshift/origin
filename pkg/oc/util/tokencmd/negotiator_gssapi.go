// +build gssapi

package tokencmd

import (
	"errors"
	"net"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/apcera/gssapi"
	"github.com/golang/glog"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func GSSAPIEnabled() bool {
	return true
}

type gssapiNegotiator struct {
	// handle to a loaded gssapi lib
	lib *gssapi.Lib
	// error seen when loading the lib
	loadError error
	// lock to make sure we only load it once
	loadOnce sync.Once

	// service name of the server we are negotiating against
	name *gssapi.Name
	// security context holding the state of the negotiation
	ctx *gssapi.CtxId
	// flags indicating what options we want for the negotiation
	// TODO: surface option for mutual authentication, e.g. gssapi.GSS_C_MUTUAL_FLAG
	flags uint32

	// principalName contains the name of the principal desired by the user, if specified.
	principalName string
	// cred holds the credentials obtained for the specified principalName.
	// if no principalName is specified, GSS_C_NO_CREDENTIAL is used.
	cred *gssapi.CredId

	// track whether the last response from InitSecContext was GSS_S_COMPLETE
	complete bool
}

func NewGSSAPINegotiator(principalName string) Negotiater {
	return &gssapiNegotiator{principalName: principalName}
}

func (g *gssapiNegotiator) Load() error {
	_, err := g.loadLib()
	return err
}

func (g *gssapiNegotiator) InitSecContext(requestURL string, challengeToken []byte) (tokenToSend []byte, err error) {
	lib, err := g.loadLib()
	if err != nil {
		return nil, err
	}

	// Initialize our context if we haven't already
	if g.ctx == nil {

		if len(g.principalName) > 0 {
			// Get credentials for a specific principal
			glog.V(5).Infof("acquiring credentials for principal name %s", g.principalName)
			credBuffer, err := lib.MakeBufferString(g.principalName)
			if err != nil {
				return nil, convertGSSAPIError(err)
			}
			defer credBuffer.Release()

			credName, err := credBuffer.Name(lib.GSS_KRB5_NT_PRINCIPAL_NAME)
			if err != nil {
				return nil, convertGSSAPIError(err)
			}
			defer credName.Release()

			cred, _, _, err := lib.AcquireCred(credName, time.Duration(0), lib.GSS_C_NO_OID_SET, gssapi.GSS_C_INITIATE)
			if err != nil {
				glog.V(5).Infof("AcquireCred returned error: %v", err)
				return nil, convertGSSAPIError(err)
			}
			g.cred = cred
		} else {
			// otherwise, express no opinion about the credentials and let gssapi decide
			g.cred = lib.GSS_C_NO_CREDENTIAL
		}

		u, err := url.Parse(requestURL)
		if err != nil {
			return nil, err
		}

		hostname := u.Host
		if h, _, err := net.SplitHostPort(u.Host); err == nil {
			hostname = h
		}

		serviceName := "HTTP@" + hostname
		glog.V(5).Infof("importing service name %s", serviceName)
		nameBuf, err := lib.MakeBufferString(serviceName)
		if err != nil {
			return nil, convertGSSAPIError(err)
		}
		defer nameBuf.Release()

		name, err := nameBuf.Name(lib.GSS_C_NT_HOSTBASED_SERVICE)
		if err != nil {
			return nil, convertGSSAPIError(err)
		}
		g.name = name
		g.ctx = lib.GSS_C_NO_CONTEXT
	}

	incomingTokenBuffer, err := lib.MakeBufferBytes(challengeToken)
	if err != nil {
		return nil, convertGSSAPIError(err)
	}
	defer incomingTokenBuffer.Release()

	var outgoingToken *gssapi.Buffer
	g.ctx, _, outgoingToken, _, _, err = lib.InitSecContext(g.cred, g.ctx, g.name, lib.GSS_C_NO_OID, g.flags, time.Duration(0), lib.GSS_C_NO_CHANNEL_BINDINGS, incomingTokenBuffer)
	defer outgoingToken.Release()

	switch err {
	case nil:
		glog.V(5).Infof("InitSecContext returned GSS_S_COMPLETE")
		g.complete = true
		return outgoingToken.Bytes(), nil
	case gssapi.ErrContinueNeeded:
		glog.V(5).Infof("InitSecContext returned GSS_S_CONTINUE_NEEDED")
		g.complete = false
		return outgoingToken.Bytes(), nil
	default:
		glog.V(5).Infof("InitSecContext returned error: %v", err)
		return nil, convertGSSAPIError(err)
	}
}

func (g *gssapiNegotiator) IsComplete() bool {
	return g.complete
}

func (g *gssapiNegotiator) Release() error {
	var errs []error
	if err := g.name.Release(); err != nil {
		errs = append(errs, convertGSSAPIError(err))
	}
	if err := g.ctx.Release(); err != nil {
		errs = append(errs, convertGSSAPIError(err))
	}
	if err := g.cred.Release(); err != nil {
		errs = append(errs, convertGSSAPIError(err))
	}
	if err := g.lib.Unload(); err != nil {
		errs = append(errs, convertGSSAPIError(err))
	}
	return utilerrors.NewAggregate(errs)
}

func (g *gssapiNegotiator) loadLib() (*gssapi.Lib, error) {
	g.loadOnce.Do(func() {
		glog.V(5).Infof("loading gssapi")

		var libPaths []string
		switch runtime.GOOS {
		case "darwin":
			libPaths = []string{"libgssapi_krb5.dylib"}
		case "linux":
			// MIT, Heimdal
			libPaths = []string{"libgssapi_krb5.so.2", "libgssapi.so.3"}
		default:
			// Default search path
			libPaths = []string{""}
		}

		var loadErrors []error
		for _, libPath := range libPaths {
			lib, loadError := gssapi.Load(&gssapi.Options{LibPath: libPath})

			// If we successfully loaded from this path, return early
			if loadError == nil {
				glog.V(5).Infof("loaded gssapi %s", libPath)
				g.lib = lib
				return
			}

			// Otherwise, log and aggregate
			glog.V(5).Infof("%v", loadError)
			loadErrors = append(loadErrors, convertGSSAPIError(loadError))
		}
		g.loadError = utilerrors.NewAggregate(loadErrors)
	})
	return g.lib, g.loadError
}

// convertGSSAPIError takes a GSSAPI error and turns it into a regular error.
// This prevents us from accessing memory that has already been released by GSSAPI.
func convertGSSAPIError(err error) error {
	return errors.New(err.Error())
}
