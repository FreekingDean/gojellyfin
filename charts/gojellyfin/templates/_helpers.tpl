{{- define "gojellyfin.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gojellyfin.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "gojellyfin.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "gojellyfin.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gojellyfin.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- define "gojellyfin.labels" -}}
helm.sh/chart: {{ include "gojellyfin.chart" .root }}
{{ include "gojellyfin.selectorLabels" . }}
{{- with .root.Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .root.Release.Service }}
{{- end }}

{{- define "gojellyfin.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}

{{- define "gojellyfin.databaseSecretName" -}}
{{- default (include "gojellyfin.fullname" .) .Values.database.existingSecret }}
{{- end }}

{{- define "gojellyfin.databaseEnv" -}}
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: {{ include "gojellyfin.databaseSecretName" . }}
      key: {{ .Values.database.secretKey }}
{{- end }}

{{- define "gojellyfin.temporalEnv" -}}
{{- if .Values.temporal.hostPort }}
{{- if not .Values.temporal.namespace }}
{{- fail "temporal.hostPort is set but temporal.namespace is not, and the binary refuses to invent one" }}
{{- end }}
- name: TEMPORAL_HOSTPORT
  value: {{ .Values.temporal.hostPort | quote }}
- name: TEMPORAL_NAMESPACE
  value: {{ .Values.temporal.namespace | quote }}
{{- end }}
{{- end }}

{{- define "gojellyfin.sharedEnv" -}}
{{- with .Values.tmdb.existingSecret }}
- name: TMDB_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ . }}
      key: {{ $.Values.tmdb.secretKey }}
{{- end }}
{{- with .Values.tracing.otlpEndpoint }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ . | quote }}
{{- end }}
{{- include "gojellyfin.temporalEnv" . }}
{{- end }}

{{- define "gojellyfin.workerEnv" -}}
{{- include "gojellyfin.databaseEnv" . }}
{{- include "gojellyfin.sharedEnv" . }}
{{- end }}

{{- define "gojellyfin.serverEnv" -}}
{{- include "gojellyfin.databaseEnv" .root }}
- name: HTTP_PORT
  value: {{ .root.Values.httpPort | quote }}
{{- with .root.Values.hostname }}
- name: PUBLISHED_SERVER_URL
  value: {{ printf "https://%s" . | quote }}
{{- end }}
{{- if .root.Values.cors.enabled }}
- name: CORS_ORIGINS
  value: {{ .root.Values.cors.origin | quote }}
{{- end }}
- name: TRANSCODER_JOBS
  value: {{ include "gojellyfin.transcoderJobs" .workload | quote }}
- name: TRANSCODER_STALL_TIMEOUT
  value: {{ .workload.transcoderStallTimeout | quote }}
{{- include "gojellyfin.sharedEnv" .root }}
{{- end }}

{{- define "gojellyfin.cpuCores" -}}
{{- $cpu := toString . -}}
{{- if hasSuffix "m" $cpu -}}
{{- divf (float64 (trimSuffix "m" $cpu)) 1000 -}}
{{- else -}}
{{- float64 $cpu -}}
{{- end -}}
{{- end }}

{{- define "gojellyfin.transcoderJobs" -}}
{{- if .transcoderJobs -}}
{{- .transcoderJobs -}}
{{- else -}}
{{- $cores := int (floor (float64 (include "gojellyfin.cpuCores" (dig "requests" "cpu" "1" .resources)))) -}}
{{- max $cores 1 -}}
{{- end -}}
{{- end }}

{{- define "gojellyfin.mediaVolume" -}}
- name: media
{{- if .Values.media.volume }}
  {{- toYaml .Values.media.volume | nindent 2 }}
{{- else }}
  emptyDir: {}
{{- end }}
{{- end }}

{{- define "gojellyfin.mediaVolumeMount" -}}
- name: media
  mountPath: {{ .root.Values.media.mountPath }}
  readOnly: {{ .workload.mediaReadOnly }}
{{- end }}
