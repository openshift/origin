package templates

import "strings"

func MainHelpTemplate() string {
	return decorate(mainHelpTemplate, false)
}

func MainUsageTemplate() string {
	return decorate(mainUsageTemplate, true) + "\n"
}

func OptionsHelpTemplate() string {
	return decorate(optionsHelpTemplate, false)
}

func OptionsUsageTemplate() string {
	return decorate(optionsUsageTemplate, false)
}

func decorate(template string, trim bool) string {
	if trim && len(strings.Trim(template, " ")) > 0 {
		template = strings.Trim(template, "\n")
	}
	return template
}

const (
	vars = `{{$isRootCmd := isRootCmd .}}` +
		`{{$rootCmd := rootCmd .}}` +
		`{{$visibleFlags := visibleFlags (flagsNotIntersected .LocalFlags .PersistentFlags)}}` +
		`{{$explicitlyExposedFlags := exposed .}}` +
		`{{$optionsCmdFor := optionsCmdFor .}}`

	mainHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	mainUsageTemplate = vars + `{{if and .Runnable (ne .UseLine "") (ne .UseLine $rootCmd)}}
Usage:
  {{.UseLine}}{{if .HasFlags}} [options]{{end}}{{if .HasExample}}

Examples:
{{ .Example | trimRight}}
{{end}}{{ if .HasAvailableSubCommands}}
{{end}}{{end}}{{ if .HasAvailableSubCommands}}{{range cmdGroups . .Commands}}
{{.Message}}{{range .Commands}}{{if .Runnable}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}
{{end}}{{end}}{{ if or $visibleFlags.HasFlags $explicitlyExposedFlags.HasFlags}}
Options:
{{ if $visibleFlags.HasFlags}}{{flagsUsages $visibleFlags}}{{end}}{{ if $explicitlyExposedFlags.HasFlags}}{{flagsUsages $explicitlyExposedFlags}}{{end}}{{end}}{{ if .HasSubCommands }}
Use "{{$rootCmd}} help <command>" for more information about a given command.{{end}}{{ if $optionsCmdFor}}
Use "{{$optionsCmdFor}}" for a list of global command-line options (applies to all commands).{{end}}`

	optionsHelpTemplate = ``

	optionsUsageTemplate = `{{ if .HasInheritedFlags}}The following options can be passed to any command:

{{flagsUsages .InheritedFlags}}{{end}}`
)
