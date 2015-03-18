package start

import (
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/server/admin"
)

type PolicyArgs struct {
	PolicyFile          string
	CreatePolicyFile    bool
	OverwritePolicyFile bool
}

func BindPolicyArgs(args *PolicyArgs, flags *pflag.FlagSet, prefix string) {
	flags.BoolVar(&args.CreatePolicyFile, prefix+"create-policy-file", args.CreatePolicyFile, "Create bootstrap policy if none is present.")
}

func NewDefaultPolicyArgs() *PolicyArgs {
	return &PolicyArgs{
		CreatePolicyFile: true,
		PolicyFile:       admin.DefaultPolicyFile,
	}
}
