package start

import (
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
)

// ListenArg is a struct that the command stores flag values into.
type ListenArg struct {
	// ListenAddr is the address to listen for connections on (scheme://host:port).
	ListenAddr flagtypes.Addr
}

// BindListenArg binds values to the given arguments by using flags
func BindListenArg(args *ListenArg, flags *pflag.FlagSet, prefix string) {
	flags.Var(&args.ListenAddr, prefix+"listen", "The address to listen for connections on (scheme://host:port).")
}

// NewDefaultListenArg returns a new address to listen for connections
func NewDefaultListenArg() *ListenArg {
	config := &ListenArg{
		ListenAddr: flagtypes.Addr{Value: "0.0.0.0:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
	}

	return config
}

// UseTLS checks whether the address we listen for connections uses TLS or not
func (l *ListenArg) UseTLS() bool {
	return l.ListenAddr.URL.Scheme == "https"
}
