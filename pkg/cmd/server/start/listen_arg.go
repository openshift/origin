package start

import (
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
)

// ListenArg is a struct that the command stores flag values into.
type ListenArg struct {
	ListenAddr flagtypes.Addr
}

func BindListenArg(args *ListenArg, flags *pflag.FlagSet, prefix string) {
	flags.Var(&args.ListenAddr, prefix+"listen", "The address to listen for connections on (scheme://host:port).")
}

func NewDefaultListenArg() *ListenArg {
	config := &ListenArg{
		ListenAddr: flagtypes.Addr{Value: "0.0.0.0:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
	}

	return config
}

func (l *ListenArg) UseTLS() bool {
	return l.ListenAddr.URL.Scheme == "https"
}
