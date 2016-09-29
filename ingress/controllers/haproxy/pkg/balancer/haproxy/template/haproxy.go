package template

// HAProxy configuration template
const HAProxy = `
# Generated HAProxy
{{ with .Global}}
global
  daemon
  pidfile /var/run/haproxy.pid
  stats socket /var/run/haproxy.stat mode 777
  # need to check logging
  maxconn {{ .Maxconn }}
  maxpipes {{ .Maxpipes }}
  spread-checks {{ .SpreadChecks }}{{ if .Debug }}
  debug{{ end }}{{ end }}

{{ with .Defaults }}
defaults
  log global
  mode {{ .Mode }}
  balance {{ .Balance }}
  maxconn {{ .Maxconn }}
  {{ if .TCPLog }}option tcplog{{ end }}
  {{ if .HTTPLog }}option httplog{{ end }}
  {{ if .AbortOnClose }}option abortonclose{{ end }}
  {{ if .HTTPServerClose }}option httpclose{{ end }}
  {{ if .ForwardFor }}option forwardfor{{ end }}
  retries {{ .Retries }}
  {{ if .Redispatch }}option redispatch{{ end }}
  timeout client {{ .TimeoutClient }}
  timeout connect {{ .TimeoutConnect }}
  timeout server {{ .TimeoutServer }}
  {{ if .DontLogNull }}option dontlognull{{ end }}
  timeout check {{ .TimeoutCheck }}
{{ end }}{{$certs_dir:= .CertsDir }}{{ range .Frontends }}

frontend {{ .Name }}{{ with .Bind }}
  bind {{ .IP }}:{{ .Port }}{{ if .IsTLS }} ssl {{ range .Certs }}crt {{$certs_dir}}/{{.Name}}.pem {{ end }}{{ end }}{{ end }}{{ if .DefaultBackend.Backend }}
  default_backend {{ .DefaultBackend.Backend }}{{end}}{{ range .ACLs }}
  acl {{ .Name }} {{.Content}}{{end}}{{ range .UseBackendsByPrio }}
  use_backend {{ .Backend }} if {{ range .ACLs }}{{ .Name }} {{end}}{{end}}
{{ end }}
{{range $name, $be := .Backends}}
backend {{$name}}{{ range $sname, $server := .Servers}}
  server {{ $sname }} {{ $server.Address }}:{{ $server.Port }} check inter {{ $server.CheckInter}}{{end}}
{{end}}
`
