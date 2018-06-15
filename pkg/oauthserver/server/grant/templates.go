package grant

import "html/template"

var defaultGrantTemplate = template.Must(template.New("defaultGrantForm").Parse(defaultGrantTemplateString))

const defaultGrantTemplateString = `<!DOCTYPE html>

<html>

<head>
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <title>
      Authorize 
      {{ if and .ServiceAccountName .ServiceAccountNamespace }}
        service account {{ .ServiceAccountName }} in project {{ .ServiceAccountNamespace }}
      {{ else }}
        {{ .Values.ClientID }}
      {{ end }}
    </title>
    <style>
        body    { font-family: sans-serif; line-height: 1.2em; margin: 2em 5%; color: #363636; }

        table         { border-collapse: collapse; margin: 2em 0; width: 100%; max-width: 600px; }
        table caption { text-align: left; }
        td            { padding: .5em .25em; vertical-align: top; }
        tr + tr       { border-top: 1px solid #d1d1d1; }

        h1,h2,h3 { font-weight: normal; line-height: 1.3em; }

        input[type=submit]                      { font-size: 1em;   }
        input[type=submit] + input[type=submit] { margin-left: 1em; }

        .identifier-highlight { color: #8b8d8f; }

        .scope-title         { font-size: .9em; font-weight: bold; padding-top: .1em; padding-bottom: .25em; }
        .scope-details       { font-size: .8em; }
        .redirect-info       { font-size: .8em; margin: 2em 0 2em .25em; }
        .existing-permission { color: #3f9c35; text-align: center; }

        .muted   { color: #9c9c9c; }
        .warning { color: #ec7a08; }
        .error   { color: #cc0000; }

        @media (max-width:481px) {
          body { margin: .5em; }
          h1,h2,h3 { margin: 0; padding-bottom: .5em; }
          table { margin: .5em 0 }
          input[type=submit] { display: block; width: 100%; margin: 1em 0 !important; }
          h1 { font-size: 1.5em; }
          h2 { font-size: 1.3em; }
          h3 { font-size: 1.1em; }
        }
    </style>
</head>

<!-- Define a subtemplate to use for rendering existing or requested scopes -->
{{ define "scope" }}
          <div class="scope-title">{{ .Name }}</div>
          {{ if .Description }}<div class="scope-details muted"  >{{ .Description }}</div>
{{ end -}}
          {{ if .Warning     }}<div class="scope-details warning">{{ .Warning }}</div>
{{ end -}}
          {{ if .Error       }}<div class="scope-details error"  >{{ .Error }}</div>
{{ end -}}
{{ end }}

<body>
{{ if .Error }}
<div class="error">{{ .Error }}</div>
{{ else }}
<form action="{{ .Action }}" method="POST">
  <input type="hidden" name="{{ .Names.Then        }}" value="{{ .Values.Then        }}">
  <input type="hidden" name="{{ .Names.CSRF        }}" value="{{ .Values.CSRF        }}">
  <input type="hidden" name="{{ .Names.ClientID    }}" value="{{ .Values.ClientID    }}">
  <input type="hidden" name="{{ .Names.UserName    }}" value="{{ .Values.UserName    }}">
  <input type="hidden" name="{{ .Names.RedirectURI }}" value="{{ .Values.RedirectURI }}">

  <h1>Authorize Access</h1>

  <h3>
    {{ if and .ServiceAccountName .ServiceAccountNamespace }}
      Service account <span class="identifier-highlight">{{ .ServiceAccountName }}</span> in project <span class="identifier-highlight">{{ .ServiceAccountNamespace }}</span>
    {{ else }}
      <span class="identifier-highlight">{{ .Values.ClientID }}</span>
    {{ end }}

    is requesting
    
    {{ if .GrantedScopes }}
      additional permissions
    {{ else }}
      permission
    {{ end }}
    
    to access your account (<span class="identifier-highlight">{{ .Values.UserName }}</span>)
  </h3>

  <!-- Display scopes that have already been granted -->
  {{ if .GrantedScopes -}}
  <table>
    <caption>Existing access</caption>
    <colgroup><col><col width="100%"></colgroup>
    {{ range $i,$scope := .GrantedScopes -}}
    <tr>
      <td>
        <div class="scope-title existing-permission">&#10003;</div>
        <!-- Add an invisible checkbox to make spacing match the "requested permissions" table -->
        <input type="checkbox" checked disabled style="visibility:hidden">
      </td>
      <td>
        <div>
{{ template "scope" . }}
        </div>
      </td>
    </tr>
    {{ end }}
  </table>
  {{ end }}

  <!-- Write hidden inputs for requested scopes that have already been granted -->
  {{ range $i,$scope := .Values.Scopes -}}
    {{ if .Granted -}}
      <input type="hidden" name="{{ $.Names.Scopes }}" value="{{ .Name }}">
    {{- end }}
  {{ end }}

  <!-- Display requested scopes that have not been granted -->
  <table>
    <caption>
      {{- if .GrantedScopes -}}
        Additional requested permissions
      {{- else -}}
        Requested permissions
      {{- end -}}
    </caption>
    <colgroup><col><col width="100%"></colgroup>
  {{ range $i,$scope := .Values.Scopes }}
    {{ if not .Granted }}
    <tr>
      <td>
        <input type="checkbox" checked name="{{ $.Names.Scopes }}" value="{{ .Name }}" id="scope-{{$i}}">
      </td>
      <td>
        <label for="scope-{{ $i }}">
{{ template "scope" . }}
        </label>
      </td>
    </tr>
    {{ end }}
  {{ end }}
  </table>

  <!-- Tell the user where they're going -->
  {{ if .Values.RedirectURI -}}
  <div class="redirect-info">
    <div class="muted">You will be redirected to {{ .Values.RedirectURI }}</div>
  </div>
  {{- end }}

  <div>
    <input type="submit" name="{{ .Names.Approve }}" value="Allow selected permissions">
    <input type="submit" name="{{ .Names.Deny    }}" value="Deny">
  </div>
</form>
{{ end }}
</body>
</html>
`
