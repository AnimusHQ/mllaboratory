{{- define "animus-dataplane.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "animus-dataplane.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "animus-dataplane.labels" -}}
app.kubernetes.io/name: {{ include "animus-dataplane.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
{{- end -}}

{{- define "animus-dataplane.image" -}}
{{- $repo := .Values.image.repository -}}
{{- $tag := .Values.image.tag -}}
{{- $digest := .Values.image.digest -}}
{{- $profile := lower (default "dev" .Values.profile) -}}
{{- if and (eq $profile "production") (eq (trim $digest) "") -}}
{{- fail "image.digest is required when profile=production" -}}
{{- end -}}
{{- if $digest -}}
{{- printf "%s/dataplane@%s" $repo $digest -}}
{{- else -}}
{{- printf "%s/dataplane:%s" $repo $tag -}}
{{- end -}}
{{- end -}}

{{- define "animus-dataplane.secretsName" -}}
{{ include "animus-dataplane.fullname" . }}-secrets
{{- end -}}
