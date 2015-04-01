package templates

import "strings"

func MainHelpTemplate() string {
	return decorate(mainHelpTemplate, false)
}

func MainUsageTemplate() string {
	return decorate(mainUsageTemplate, false)
}

func CliHelpTemplate() string {
	return decorate(cliHelpTemplate, false)
}

func CliUsageTemplate() string {
	return decorate(cliUsageTemplate, true)
}

func AdminHelpTemplate() string {
	return decorate(adminHelpTemplate, false)
}

func AdminUsageTemplate() string {
	return decorate(adminUsageTemplate, true)
}

func OptionsHelpTemplate() string {
	return decorate(optionsHelpTemplate, false)
}

func OptionsUsageTemplate() string {
	return decorate(optionsUsageTemplate, false)
}

func decorate(template string, trim bool) string {
	if trim && len(strings.Trim(template, " ")) > 0 {
		trimmed := strings.Trim(template, "\n")
		return funcs + trimmed
	}
	return funcs + template
}

const (
	// TODO: $isRootCmd should be done in code, not in the template
	funcIsRootCmd = `{{$isRootCmd := or (and (eq .Name "cli") (eq .Root.Name "openshift")) (eq .Name "osc") (and (eq .Name "admin") (eq .Root.Name "openshift")) (eq .Name "osadm")}}`
	funcRootCli   = `{{define "rootCli"}}{{if eq .Root.Name "osadm"}}osadm{{else}}{{if eq .Root.Name "osc"}}osc{{else}}openshift {{.Name}}{{end}}{{end}}{{end}}`
	funcs         = funcIsRootCmd + funcRootCli

	mainHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	mainUsageTemplate = `{{ $cmd := . }}
Usage: {{if .Runnable}}
  {{.UseLine}}{{if .HasFlags}} [options]{{end}}{{end}}{{if .HasSubCommands}}
  {{ .CommandPath}} <command>{{end}}{{if gt .Aliases 0}}

Aliases:
  {{.NameAndAliases}}{{end}}
{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}
  {{rpad .Use .UsagePadding }} {{.Short}}{{end}}{{end}}
{{end}}
{{ if .HasLocalFlags}}Options:
{{flagsUsages .LocalFlags}}{{end}}
{{ if .HasInheritedFlags}}Global Options:
{{flagsUsages .InheritedFlags}}{{end}}{{ if .HasSubCommands }}
Use "{{.Root.Name}} <command> --help" for more information about a given command.
{{end}}`

	cliHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	cliUsageTemplate = `{{ $cmd := . }}{{$explicitlyExposedFlags := exposed .}}{{$localNotPersistentFlags := flagsNotIntersected .LocalFlags .PersistentFlags}}{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}{{if ne .Name "options"}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}
{{end}}
{{ if or $localNotPersistentFlags.HasFlags $explicitlyExposedFlags.HasFlags}}Options:
{{ if $localNotPersistentFlags.HasFlags}}{{flagsUsages $localNotPersistentFlags}}{{end}}{{ if $explicitlyExposedFlags.HasFlags}}{{flagsUsages $explicitlyExposedFlags}}{{end}}
{{end}}{{ if not $isRootCmd}}Use "{{template "rootCli" .}} --help" for a list of all commands available in {{template "rootCli" .}}.
{{end}}{{ if .HasSubCommands }}Use "{{template "rootCli" .}} <command> --help" for more information about a given command.
{{end}}{{ if .HasInheritedFlags}}Use "{{template "rootCli" .}} options" for a list of global command-line options (applies to all commands).
{{end}}`

	adminHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	adminUsageTemplate = `{{ $cmd := . }}{{$explicitlyExposedFlags := exposed .}}{{$localNotPersistentFlags := flagsNotIntersected .LocalFlags .PersistentFlags}}{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}{{if ne .Name "options"}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}
{{end}}
{{ if or $localNotPersistentFlags.HasFlags $explicitlyExposedFlags.HasFlags}}Options:
{{ if $localNotPersistentFlags.HasFlags}}{{flagsUsages $localNotPersistentFlags}}{{end}}{{ if $explicitlyExposedFlags.HasFlags}}{{flagsUsages $explicitlyExposedFlags}}{{end}}
{{end}}{{ if not $isRootCmd}}Use "{{template "rootCli" .}} --help" for a list of all commands available in {{template "rootCli" .}}.
{{end}}{{ if .HasSubCommands }}Use "{{template "rootCli" .}} <command> --help" for more information about a given command.
{{end}}{{ if .HasInheritedFlags}}Use "{{template "rootCli" .}} options" for a list of global command-line options (applies to all commands).
{{end}}`

	optionsHelpTemplate = ``

	optionsUsageTemplate = `{{ if .HasInheritedFlags}}The following options can be passed to any command:

{{flagsUsages .InheritedFlags}}{{end}}`
)
