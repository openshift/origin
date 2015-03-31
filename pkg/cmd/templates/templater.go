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
			"trim": strings.TrimSpace,
			"gt":   cobra.Gt,
			"eq":   cobra.Eq,
			"rpad": func(s string, padding int) string {
				template := fmt.Sprintf("%%-%ds", padding)
				return fmt.Sprintf(template, s)
			},
			"exposed": func(*cobra.Command) *flag.FlagSet {
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
			"flagsUsages": func(f *flag.FlagSet) string {
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
			},
		})

		template.Must(t.Parse(templater.UsageTemplate))
		return t.Execute(c.Out(), c)
	}
}

func UseCliTemplates(cmd *cobra.Command) {
	cmd.SetHelpTemplate(CliHelpTemplate())
	templater := &Templater{UsageTemplate: CliUsageTemplate()}
	cmd.SetUsageFunc(templater.UsageFunc())
}

func UseAdminTemplates(cmd *cobra.Command) {
	cmd.SetHelpTemplate(AdminHelpTemplate())
	templater := &Templater{UsageTemplate: AdminUsageTemplate()}
	cmd.SetUsageFunc(templater.UsageFunc())
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
