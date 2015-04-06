package templates

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

type Templater struct {
	UsageTemplate string
	Exposed       []string
}

func (templater *Templater) UsageFunc() func(*cobra.Command) error {
	return func(c *cobra.Command) error {
		t := template.New("custom")

		t.Funcs(template.FuncMap{
			"trim":                strings.TrimSpace,
			"gt":                  cobra.Gt,
			"eq":                  cobra.Eq,
			"rpad":                rpad,
			"flagsNotIntersected": flagsNotIntersected,
			"flagsUsages":         flagsUsages,
			"rootCmd":             rootCmdName,
			"isRootCmd":           isRootCmd,
			"exposed": func(c *cobra.Command) *flag.FlagSet {
				exposed := flag.NewFlagSet("exposed", flag.ContinueOnError)
				if len(templater.Exposed) > 0 {
					for _, name := range templater.Exposed {
						if flag := c.Flags().Lookup(name); flag != nil {
							exposed.AddFlag(flag)
						}
					}
				}
				return exposed
			},
		})

		template.Must(t.Parse(templater.UsageTemplate))
		return t.Execute(c.Out(), c)
	}
}

func UseMainTemplates(cmd *cobra.Command) {
	cmd.SetHelpTemplate(MainHelpTemplate())
	templater := &Templater{UsageTemplate: MainUsageTemplate()}
	cmd.SetUsageFunc(templater.UsageFunc())
}

func UseOptionsTemplates(cmd *cobra.Command) {
	cmd.SetHelpTemplate(OptionsHelpTemplate())
	templater := &Templater{UsageTemplate: OptionsUsageTemplate()}
	cmd.SetUsageFunc(templater.UsageFunc())
}

func rootCmd(c *cobra.Command) []string {
	root := []string{}

	var appendCmdName func(*cobra.Command)
	appendCmdName = func(x *cobra.Command) {
		if x.HasParent() {
			appendCmdName(x.Parent())
			root = append(root, x.Parent().Name())
		}
	}
	appendCmdName(c)

	if cName := c.Name(); c.HasSubCommands() && len(root) == 1 && root[0] == "openshift" && cName != "openshift" {
		root = append(root, cName)
	}

	if len(root) == 0 {
		root = append(root, c.Name())
	}

	return root
}

func rootCmdName(c *cobra.Command) string {
	return strings.Join(rootCmd(c), " ")
}

func isRootCmd(c *cobra.Command) bool {
	r := rootCmd(c)
	return c.HasSubCommands() && r[len(r)-1] == c.Name()
}

func flagsUsages(f *flag.FlagSet) string {
	x := new(bytes.Buffer)

	f.VisitAll(func(flag *flag.Flag) {
		format := "--%s=%s: %s\n"

		if flag.Value.Type() == "string" {
			format = "--%s='%s': %s\n"
		}

		if len(flag.Shorthand) > 0 {
			format = "  -%s, " + format
		} else {
			format = "   %s   " + format
		}

		fmt.Fprintf(x, format, flag.Shorthand, flag.Name, flag.DefValue, flag.Usage)
	})

	return x.String()
}

func rpad(s string, padding int) string {
	template := fmt.Sprintf("%%-%ds", padding)
	return fmt.Sprintf(template, s)
}

func flagsNotIntersected(l *flag.FlagSet, r *flag.FlagSet) *flag.FlagSet {
	f := flag.NewFlagSet("notIntersected", flag.ContinueOnError)
	l.VisitAll(func(flag *flag.Flag) {
		if r.Lookup(flag.Name) == nil {
			f.AddFlag(flag)
		}
	})
	return f
}
