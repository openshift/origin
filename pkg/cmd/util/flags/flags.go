package flags

import (
	"fmt"

	"github.com/spf13/pflag"
)

// Apply stores the provided arguments onto a flag set, reporting any errors
// encountered during the process.
func Apply(args map[string][]string, flags *pflag.FlagSet) []error {
	var errs []error
	for key, value := range args {
		flag := flags.Lookup(key)
		if flag == nil {
			errs = append(errs, fmt.Errorf("%q is not a valid flag", key))
			continue
		}
		for _, s := range value {
			if err := flag.Value.Set(s); err != nil {
				errs = append(errs, fmt.Errorf("%q could not be set: %v", err))
				break
			}
		}
	}
	return errs
}

func Resolve(args map[string][]string, fn func(*pflag.FlagSet)) []error {
	fs := pflag.NewFlagSet("extended", pflag.ContinueOnError)
	fn(fs)
	return Apply(args, fs)
}
