package templates

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode"

	"github.com/openshift/origin/pkg/cmd/util/term"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

type FlagExposer interface {
	ExposeFlags(cmd *cobra.Command, flags ...string) FlagExposer
}

func ActsAsRootCommand(cmd *cobra.Command, filters []string, groups ...CommandGroup) FlagExposer {
	if cmd == nil {
		panic("nil root command")
	}
	templater := &templater{
		RootCmd:       cmd,
		UsageTemplate: MainUsageTemplate(),
		HelpTemplate:  MainHelpTemplate(),
		CommandGroups: groups,
		Filtered:      filters,
	}
	cmd.SetUsageFunc(templater.UsageFunc())
	cmd.SetHelpFunc(templater.HelpFunc())
	return templater
}

func UseOptionsTemplates(cmd *cobra.Command) {
	templater := &templater{
		UsageTemplate: OptionsUsageTemplate(),
		HelpTemplate:  OptionsHelpTemplate(),
	}
	cmd.SetUsageFunc(templater.UsageFunc())
	cmd.SetHelpFunc(templater.HelpFunc())
}

type templater struct {
	UsageTemplate string
	HelpTemplate  string
	RootCmd       *cobra.Command
	CommandGroups
	Filtered []string
}

func (templater *templater) ExposeFlags(cmd *cobra.Command, flags ...string) FlagExposer {
	cmd.SetUsageFunc(templater.UsageFunc(flags...))
	return templater
}

func (templater *templater) HelpFunc() func(*cobra.Command, []string) {
	return func(c *cobra.Command, s []string) {
		t := template.New("help")
		t.Funcs(templater.templateFuncs())
		template.Must(t.Parse(templater.HelpTemplate))
		out := term.NewResponsiveWriter(c.OutOrStdout())
		err := t.Execute(out, c)
		if err != nil {
			c.Println(err)
		}
	}
}

func (templater *templater) UsageFunc(exposedFlags ...string) func(*cobra.Command) error {
	return func(c *cobra.Command) error {
		t := template.New("usage")
		t.Funcs(templater.templateFuncs(exposedFlags...))
		template.Must(t.Parse(templater.UsageTemplate))
		out := term.NewResponsiveWriter(c.OutOrStderr())
		return t.Execute(out, c)
	}
}

func (templater *templater) templateFuncs(exposedFlags ...string) template.FuncMap {
	return template.FuncMap{
		"trim":                strings.TrimSpace,
		"trimRight":           func(s string) string { return strings.TrimRightFunc(s, unicode.IsSpace) },
		"trimLeft":            func(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) },
		"gt":                  cobra.Gt,
		"eq":                  cobra.Eq,
		"rpad":                rpad,
		"appendIfNotPresent":  appendIfNotPresent,
		"flagsNotIntersected": flagsNotIntersected,
		"visibleFlags":        visibleFlags,
		"flagsUsages":         flagsUsages,
		"cmdGroups":           templater.cmdGroups,
		"cmdGroupsString":     templater.cmdGroupsString,
		"rootCmd":             templater.rootCmdName,
		"isRootCmd":           templater.isRootCmd,
		"optionsCmdFor":       templater.optionsCmdFor,
		"usageLine":           templater.usageLine,
		"exposed": func(c *cobra.Command) *flag.FlagSet {
			exposed := flag.NewFlagSet("exposed", flag.ContinueOnError)
			if len(exposedFlags) > 0 {
				for _, name := range exposedFlags {
					if flag := c.Flags().Lookup(name); flag != nil {
						exposed.AddFlag(flag)
					}
				}
			}
			return exposed
		},
	}
}

func (templater *templater) cmdGroups(c *cobra.Command, all []*cobra.Command) []CommandGroup {
	if len(templater.CommandGroups) > 0 && c == templater.RootCmd {
		all = filter(all, templater.Filtered...)
		return AddAdditionalCommands(templater.CommandGroups, "Other Commands:", all)
	}
	all = filter(all, "options")
	return []CommandGroup{
		{
			Message:  "Available Commands:",
			Commands: all,
		},
	}
}

func (t *templater) cmdGroupsString(c *cobra.Command) string {
	groups := []string{}
	for _, cmdGroup := range t.cmdGroups(c, c.Commands()) {
		cmds := []string{cmdGroup.Message}
		for _, cmd := range cmdGroup.Commands {
			if cmd.Runnable() {
				cmds = append(cmds, "  "+rpad(cmd.Name(), cmd.NamePadding())+" "+cmd.Short)
			}
		}
		groups = append(groups, strings.Join(cmds, "\n"))
	}
	return strings.Join(groups, "\n\n")
}

func (t *templater) rootCmdName(c *cobra.Command) string {
	return t.rootCmd(c).CommandPath()
}

func (t *templater) isRootCmd(c *cobra.Command) bool {
	return t.rootCmd(c) == c
}

func (t *templater) parents(c *cobra.Command) []*cobra.Command {
	parents := []*cobra.Command{c}
	for current := c; !t.isRootCmd(current) && current.HasParent(); {
		current = current.Parent()
		parents = append(parents, current)
	}
	return parents
}

func (t *templater) rootCmd(c *cobra.Command) *cobra.Command {
	if c != nil && !c.HasParent() {
		return c
	}
	if t.RootCmd == nil {
		panic("nil root cmd")
	}
	return t.RootCmd
}

func (t *templater) optionsCmdFor(c *cobra.Command) string {
	if !c.Runnable() {
		return ""
	}

	parentCmdHasOptsArg := false
	currentCmdHasOptsArg := false

	if t.RootCmd.HasParent() {
		if _, _, err := t.RootCmd.Parent().Find([]string{"options"}); err == nil {
			parentCmdHasOptsArg = true
		}
	}

	if _, _, err := t.RootCmd.Find([]string{"options"}); err == nil {
		currentCmdHasOptsArg = true
	}

	if (parentCmdHasOptsArg && currentCmdHasOptsArg) || !t.RootCmd.HasParent() {
		return t.RootCmd.CommandPath() + " options"
	}

	return t.RootCmd.Parent().CommandPath() + " options"
}

func (t *templater) usageLine(c *cobra.Command) string {
	usage := c.UseLine()
	suffix := "[options]"
	if c.HasFlags() && !strings.Contains(usage, suffix) {
		usage += " " + suffix
	}
	return usage
}

func flagsUsages(f *flag.FlagSet) string {
	x := new(bytes.Buffer)

	f.VisitAll(func(flag *flag.Flag) {
		if flag.Hidden {
			return
		}
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

func indentLines(s string, indentation int) string {
	r := []string{}
	for _, line := range strings.Split(s, "\n") {
		indented := strings.Repeat(" ", indentation) + line
		r = append(r, indented)
	}
	return strings.Join(r, "\n")
}

func appendIfNotPresent(s, stringToAppend string) string {
	if strings.Contains(s, stringToAppend) {
		return s
	}
	return s + " " + stringToAppend
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

func visibleFlags(l *flag.FlagSet) *flag.FlagSet {
	hidden := "help"
	f := flag.NewFlagSet("visible", flag.ContinueOnError)
	l.VisitAll(func(flag *flag.Flag) {
		if flag.Name != hidden {
			f.AddFlag(flag)
		}
	})
	return f
}

func filter(cmds []*cobra.Command, names ...string) []*cobra.Command {
	out := []*cobra.Command{}
	for _, c := range cmds {
		if c.Hidden {
			continue
		}
		skip := false
		for _, name := range names {
			if name == c.Name() {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, c)
	}
	return out
}
