package templates

const (
	MainHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}
`

	MainUsageTemplate = `{{ $cmd := . }}
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
Use "{{.Root.Name}} help [command]" for more information about that command.{{end}}`

	CliHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}
`
	CliUsageTemplate = `{{ $cmd := . }}{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}{{if ne .Name "options"}}
 {{rpad .Use .UsagePadding }} {{.Short}}{{end}}{{end}}{{end}}
{{end}}
{{ if .HasLocalFlags}}Options:
{{.LocalFlags.FlagUsages}}{{end}}{{ if .HasSubCommands }}
Use "{{.Root.Name}} help [command]" for more information about that command.{{end}}{{ if .HasAnyPersistentFlags}}
Use "{{.Root.Name}} options" for a list of global command-line options.{{end}}`

	OptionsUsageTemplate = `{{ if .HasAnyPersistentFlags}}The following options can be passed to any command:

{{.AllPersistentFlags.FlagUsages}}{{end}}`
)
