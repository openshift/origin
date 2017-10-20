{{/* vim: set filetype=mustache: */}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Conditionally print out rbac api verison.
This will select v1 before v1beta1 if both are available.
*/}}
{{- define "rbacApiVersion" -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1" -}}
rbac.authorization.k8s.io/v1
{{- else if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1beta1" -}}
rbac.authorization.k8s.io/v1beta1
{{- end -}}
{{- end -}}
