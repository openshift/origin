package openshift

const helpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}
`
const usageTemplate = `{{ $cmd := . }}
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
{{.AllPersistentFlags.FlagUsages}}{{end}}{{if .HasParent}}{{if and (gt .Commands 0) (gt .Parent.Commands 1) }}
Additional help topics: {{if gt .Commands 0 }}{{range .Commands}}{{if not .Runnable}} {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if gt .Parent.Commands 1 }}{{range .Parent.Commands}}{{if .Runnable}}{{if not (eq .Name $cmd.Name) }}{{end}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{end}}
{{end}}{{ if .HasSubCommands }}
Use "{{.Root.Name}} help [command]" for more information about that command.
{{end}}`
