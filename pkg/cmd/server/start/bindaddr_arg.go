package start

import (
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
)

// BindAddrArg is a struct that the command stores flag values into.
type BindAddrArg struct {
	BindAddr flagtypes.Addr
}

func BindBindAddrArg(args *BindAddrArg, flags *pflag.FlagSet, prefix string) {
	flags.Var(&args.BindAddr, prefix+"listen", "The address to listen for connections on (host, host:port, or URL).")
}

func NewDefaultBindAddrArg() *BindAddrArg {
	config := &BindAddrArg{
		BindAddr: flagtypes.Addr{Value: "0.0.0.0:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default(),
	}

	return config
}
