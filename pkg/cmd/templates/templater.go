package templates

import (
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
