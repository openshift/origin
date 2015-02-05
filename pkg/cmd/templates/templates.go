package templates

import "strings"

func decorate(template string) string {
	if len(strings.Trim(template, " ")) > 0 {
		trimmed := strings.Trim(template, "\n")
		return trimmed
	} else {
		return template
	}
}

func MainHelpTemplate() string {
	return mainHelpTemplate
}

func MainUsageTemplate() string {
	return mainUsageTemplate
}

func CliHelpTemplate() string {
	return cliHelpTemplate
}

func CliUsageTemplate() string {
	return decorate(cliUsageTemplate)
}

func OptionsHelpTemplate() string {
	return optionsHelpTemplate
}

func OptionsUsageTemplate() string {
	return optionsUsageTemplate
}

const (
	mainHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	mainUsageTemplate = `{{ $cmd := . }}
Usage: {{if .Runnable}}
  {{.UseLine}}{{if .HasFlags}} [options]{{end}}{{end}}{{if .HasSubCommands}}
  {{ .CommandPath}} [command]{{end}}{{if gt .Aliases 0}}

Aliases:
  {{.NameAndAliases}}{{end}}
{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}
  {{rpad .Use .UsagePadding }} {{.Short}}{{end}}{{end}}
{{end}}
{{ if .HasLocalFlags}}Options:
{{.LocalFlags.FlagUsages}}{{end}}
{{ if .HasAnyPersistentFlags}}Global Options:
{{.AllPersistentFlags.FlagUsages}}{{end}}{{ if .HasSubCommands }}
Use "{{.Root.Name}} help [command]" for more information about that command.
{{end}}`

	cliHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	cliUsageTemplate = `{{ $cmd := . }}{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}{{if ne .Name "options"}}
  {{rpad .Name 20 }}{{.Short}}{{end}}{{end}}{{end}}
{{end}}
{{ if .HasLocalFlags}}Options:
{{.LocalFlags.FlagUsages}}{{end}}{{ if ne .Name .Root.Name}}Use "{{.Root.Name}} --help" for a list of all commands available in {{.Root.Name}}.
{{end}}{{ if .HasSubCommands }}Use "{{.Root.Name}} <command> --help" for more information about a given command.
{{end}}{{ if .HasAnyPersistentFlags}}Use "{{.Root.Name}} options --help" for a list of global command-line options (applies to all commands).
{{end}}`

	optionsHelpTemplate = `{{ if .HasAnyPersistentFlags}}The following options can be passed to any command:

{{.AllPersistentFlags.FlagUsages}}{{end}}`

	optionsUsageTemplate = ``
)
